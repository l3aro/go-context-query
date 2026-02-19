//go:build windows
// +build windows

package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func Start(opts *StartOptions) (*StartResult, error) {
	status, err := CheckStatus()
	if err == nil && status.Running && status.Ready {
		return &StartResult{
			Success: false,
			PID:     status.PID,
			Error:   "daemon already running",
		}, nil
	}

	daemonPath := opts.DaemonPath
	if daemonPath == "" {
		daemonPath = findDaemonBinary()
		if daemonPath == "" {
			return nil, fmt.Errorf("daemon binary not found")
		}
	}

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

	if err := WritePID(pid); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("writing PID file: %w", err)
	}

	result := &StartResult{
		Success:   true,
		PID:       pid,
		StartedAt: startedAt,
	}

	if opts.WaitForReady {
		if err := waitForReady(opts.SocketPath, opts.ReadyTimeout); err != nil {
			return result, fmt.Errorf("waiting for ready: %w", err)
		}
		result.Ready = true
	}

	return result, nil
}

func Stop() (*StopResult, error) {
	pid, err := ReadPID()
	if err != nil {
		return nil, fmt.Errorf("reading PID: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("finding process: %w", err)
	}

	if err := process.Kill(); err != nil {
		return nil, fmt.Errorf("killing process: %w", err)
	}

	RemovePID()

	return &StopResult{
		Success: true,
	}, nil
}

func findDaemonBinary() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}

	exeDir := filepath.Dir(exePath)
	daemonPath := filepath.Join(exeDir, "gcqd")

	if _, err := os.Stat(daemonPath); err == nil {
		return daemonPath
	}

	return ""
}

func getSocketPath() (string, error) {
	socketPath := os.Getenv("GCQ_SOCKET_PATH")
	if socketPath != "" {
		return socketPath, nil
	}

	configPath := os.Getenv("GCQ_CONFIG_PATH")
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home dir: %w", err)
		}
		configPath = filepath.Join(homeDir, ".gcq", "config.yaml")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("reading config: %w", err)
	}

	var config struct {
		SocketPath string `yaml:"socket_path"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("parsing config: %w", err)
	}

	if config.SocketPath == "" {
		return "", fmt.Errorf("socket_path not configured")
	}

	return config.SocketPath, nil
}

func processAlive(process *os.Process) bool {
	err := process.Signal(os.Signal(nil))
	return err == nil
}

func waitForReady(socketPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for daemon ready")
}
