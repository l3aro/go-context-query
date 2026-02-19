//go:build !windows
// +build !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Start starts the daemon
func Start(opts *StartOptions) (*StartResult, error) {
	// Check if already running
	status, err := CheckStatus()
	if err == nil && status.Running && status.Ready {
		return &StartResult{
			Success: false,
			PID:     status.PID,
			Error:   "daemon already running",
		}, nil
	}

	// Find daemon binary if not specified
	daemonPath := opts.DaemonPath
	if daemonPath == "" {
		daemonPath = findDaemonBinary()
		if daemonPath == "" {
			return nil, fmt.Errorf("daemon binary not found")
		}
	}

	// Prepare environment
	env := os.Environ()
	if opts.SocketPath != "" {
		env = append(env, "GCQ_SOCKET_PATH="+opts.SocketPath)
	}
	if opts.ConfigPath != "" {
		env = append(env, "GCQ_CONFIG_PATH="+opts.ConfigPath)
	}
	if opts.Verbose {
		env = append(env, "GCQ_VERBOSE=true")
	}

	// Start the daemon
	cmd := &exec.Cmd{
		Path: daemonPath,
		Env:  env,
	}

	if opts.Background {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting daemon: %w", err)
	}

	pid := cmd.Process.Pid
	startedAt := time.Now()

	// Write PID file
	if err := WritePID(pid); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("writing PID file: %w", err)
	}

	// Write initial status
	if err := WriteStatus(&DaemonStatus{
		Running:   true,
		PID:       pid,
		Ready:     false,
		StartedAt: startedAt,
	}); err != nil {
		cmd.Process.Kill()
		RemovePID()
		return nil, fmt.Errorf("writing status: %w", err)
	}

	// Wait for ready if requested
	if opts.WaitForReady {
		timeout := opts.ReadyTimeout
		if timeout <= 0 {
			timeout = ReadyTimeout
		}

		// Create a context with timeout for waiting
		waitCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		ready, err := waitForReady(waitCtx, timeout)
		if err != nil {
			// Try to kill the process
			cmd.Process.Kill()
			RemovePID()
			RemoveStatus()
			return &StartResult{
				Success:   false,
				PID:       pid,
				StartedAt: startedAt,
				Error:     fmt.Sprintf("daemon not ready: %v", err),
			}, nil
		}

		// Update status to ready
		WriteStatus(&DaemonStatus{
			Running:   true,
			PID:       pid,
			Ready:     ready,
			StartedAt: startedAt,
		})
	}

	return &StartResult{
		Success:   true,
		PID:       pid,
		StartedAt: startedAt,
	}, nil
}

// findDaemonBinary finds the daemon binary path
func findDaemonBinary() string {
	// Check current directory
	exePath := filepath.Join(".", "bin", "gcqd")
	if _, err := os.Stat(exePath); err == nil {
		return exePath
	}

	// Check parent directory
	exePath = filepath.Join("..", "bin", "gcqd")
	if _, err := os.Stat(exePath); err == nil {
		return exePath
	}

	// Check GCQ_DAEMON_PATH env var
	if path := os.Getenv("GCQ_DAEMON_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Use default
	return "gcqd"
}

// waitForReady waits for the daemon to be ready.
// It checks the context for cancellation and respects context deadlines.
func waitForReady(ctx context.Context, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)

	// Use earlier of context deadline or timeout
	if d, ok := ctx.Deadline(); ok {
		if d.Before(deadline) {
			deadline = d
		}
	}

	for time.Now().Before(deadline) {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		status, err := CheckStatus()
		if err == nil && status.Running && status.Ready {
			return true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return false, fmt.Errorf("timeout waiting for daemon to be ready")
}

// Stop stops the daemon
func Stop() (*StopResult, error) {
	// Check if PID file exists
	if !PIDExists() {
		return &StopResult{
			Success: false,
			Error:   "daemon not running (no PID file)",
		}, nil
	}

	// Read PID
	pid, err := ReadPID()
	if err != nil {
		return &StopResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read PID: %v", err),
		}, nil
	}

	// Check if process is running
	if !IsProcessRunning(pid) {
		// Cleanup stale PID file
		RemovePID()
		RemoveStatus()
		return &StopResult{
			Success: false,
			Error:   "daemon not running (process not found)",
		}, nil
	}

	// Try graceful shutdown first via socket
	if err := sendStopCommand(); err == nil {
		// Wait for process to exit
		if waitForShutdown(pid, ShutdownTimeout) {
			RemovePID()
			RemoveStatus()
			return &StopResult{
				Success:   true,
				PID:       pid,
				StoppedAt: time.Now(),
			}, nil
		}
	}

	// Force kill if graceful shutdown failed
	process, err := os.FindProcess(pid)
	if err != nil {
		RemovePID()
		RemoveStatus()
		return &StopResult{
			Success:   true,
			PID:       pid,
			StoppedAt: time.Now(),
			Error:     "process already terminated",
		}, nil
	}

	if err := process.Kill(); err != nil {
		return &StopResult{
			Success: false,
			PID:     pid,
			Error:   fmt.Sprintf("failed to kill process: %v", err),
		}, nil
	}

	// Wait for process to exit
	waitForShutdown(pid, 2*time.Second)

	// Cleanup
	RemovePID()
	RemoveStatus()

	return &StopResult{
		Success:   true,
		PID:       pid,
		StoppedAt: time.Now(),
	}, nil
}

// sendStopCommand sends a stop command via Unix socket
func sendStopCommand() error {
	socketPath := GetSocketPath()
	var conn net.Conn
	var err error

	// Check if we should use TCP (Windows)
	isWindows := false
	if runtime := strings.ToLower(os.Getenv("GOOS")); runtime == "windows" || !strings.HasPrefix(socketPath, "/") {
		isWindows = true
	}

	if isWindows {
		port := os.Getenv("GCQ_TCP_PORT")
		if port == "" {
			port = DefaultTCPPort
		}
		conn, err = net.Dial("tcp", "localhost:"+port)
	} else {
		conn, err = net.Dial("unix", socketPath)
	}

	if err != nil {
		return fmt.Errorf("connecting to daemon: %w", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Send stop command
	cmd := map[string]interface{}{
		"type": "stop",
		"id":   "stop-cmd",
	}

	encoder := json.NewEncoder(conn)
	return encoder.Encode(cmd)
}

// waitForShutdown waits for the process to shutdown
func waitForShutdown(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if !IsProcessRunning(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}

	return false
}

// Status returns the current daemon status
func Status() (*DaemonStatus, error) {
	return CheckStatus()
}

// GetStatus returns a formatted status result
func GetStatus() (*StatusResult, error) {
	status, err := CheckStatus()
	if err != nil {
		return &StatusResult{
			Status:  "unknown",
			Running: false,
			Ready:   false,
			Error:   err.Error(),
		}, nil
	}

	result := &StatusResult{
		Running:   status.Running,
		Ready:     status.Ready,
		PID:       status.PID,
		Version:   status.Version,
		StartedAt: status.StartedAt,
	}

	if !status.Running {
		result.Status = "stopped"
	} else if !status.Ready {
		result.Status = "starting"
	} else {
		result.Status = "running"
	}

	if status.Error != "" {
		result.Error = status.Error
	}

	return result, nil
}
