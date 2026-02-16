// Package daemon provides lifecycle management for the go-context-query daemon.
// It handles PID file management, status tracking, and start/stop/status commands.
package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// DefaultDir is the default directory for daemon files
	DefaultDir = ".gcq"
	// PIDFileName is the name of the PID file
	PIDFileName = "daemon.pid"
	// StatusFileName is the name of the status file
	StatusFileName = "status"
	// DefaultSocketPath is the default Unix socket path
	DefaultSocketPath = "/tmp/gcq.sock"
	// DefaultTCPPort is the default TCP port for Windows
	DefaultTCPPort = "9847"
	// ReadyTimeout is the timeout for waiting daemon to be ready
	ReadyTimeout = 10 * time.Second
	// ShutdownTimeout is the timeout for waiting daemon to shutdown
	ShutdownTimeout = 5 * time.Second
)

// DaemonDir returns the path to the daemon directory
func DaemonDir() string {
	dir := os.Getenv("GCQ_DAEMON_DIR")
	if dir != "" {
		return dir
	}
	// Use current directory as base
	cwd, err := os.Getwd()
	if err != nil {
		return DefaultDir
	}
	return filepath.Join(cwd, DefaultDir)
}

// PIDFile returns the path to the PID file
func PIDFile() string {
	return filepath.Join(DaemonDir(), PIDFileName)
}

// StatusFile returns the path to the status file
func StatusFile() string {
	return filepath.Join(DaemonDir(), StatusFileName)
}

// ensureDaemonDir ensures the daemon directory exists
func ensureDaemonDir() error {
	dir := DaemonDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating daemon directory: %w", err)
	}
	return nil
}

// WritePID writes the PID to the PID file
func WritePID(pid int) error {
	if err := ensureDaemonDir(); err != nil {
		return err
	}
	pidStr := strconv.Itoa(pid)
	if err := os.WriteFile(PIDFile(), []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}
	return nil
}

// ReadPID reads the PID from the PID file
func ReadPID() (int, error) {
	data, err := os.ReadFile(PIDFile())
	if err != nil {
		return 0, fmt.Errorf("reading PID file: %w", err)
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("parsing PID: %w", err)
	}
	return pid, nil
}

// RemovePID removes the PID file
func RemovePID() error {
	if err := os.Remove(PIDFile()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing PID file: %w", err)
	}
	return nil
}

// PIDExists checks if the PID file exists.
// Uses os.Stat error check pattern intentionally - we only care about existence.
func PIDExists() bool {
	_, err := os.Stat(PIDFile())
	return err == nil
}

// DaemonStatus represents the status of the daemon
type DaemonStatus struct {
	Running   bool      `json:"running"`
	PID       int       `json:"pid,omitempty"`
	Ready     bool      `json:"ready"`
	StartedAt time.Time `json:"started_at,omitempty"`
	Error     string    `json:"error,omitempty"`
	Version   string    `json:"version,omitempty"`
}

// WriteStatus writes the status to the status file
func WriteStatus(status *DaemonStatus) error {
	if err := ensureDaemonDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling status: %w", err)
	}
	if err := os.WriteFile(StatusFile(), data, 0644); err != nil {
		return fmt.Errorf("writing status file: %w", err)
	}
	return nil
}

// ReadStatus reads the status from the status file
func ReadStatus() (*DaemonStatus, error) {
	data, err := os.ReadFile(StatusFile())
	if err != nil {
		return nil, fmt.Errorf("reading status file: %w", err)
	}
	var status DaemonStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("parsing status: %w", err)
	}
	return &status, nil
}

// RemoveStatus removes the status file
func RemoveStatus() error {
	if err := os.Remove(StatusFile()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing status file: %w", err)
	}
	return nil
}

// IsProcessRunning checks if a process with the given PID is running
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds, we need to send signal 0 to check
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// GetSocketPath returns the socket path from config or default
func GetSocketPath() string {
	socketPath := os.Getenv("GCQ_SOCKET_PATH")
	if socketPath != "" {
		return socketPath
	}
	return DefaultSocketPath
}

// pingDaemon sends a status ping to the daemon and returns the response
func pingDaemon() (*DaemonStatus, error) {
	socketPath := GetSocketPath()
	var conn net.Conn
	var err error

	// Check if we should use TCP (Windows)
	if runtime := strings.ToLower(os.Getenv("GOOS")); runtime == "windows" || !strings.HasPrefix(socketPath, "/") {
		port := os.Getenv("GCQ_TCP_PORT")
		if port == "" {
			port = DefaultTCPPort
		}
		conn, err = net.Dial("tcp", "localhost:"+port)
	} else {
		conn, err = net.Dial("unix", socketPath)
	}

	if err != nil {
		return nil, fmt.Errorf("connecting to daemon: %w", err)
	}
	defer conn.Close()

	// Set deadline
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Send status command
	cmd := map[string]interface{}{
		"type": "status",
		"id":   "ping",
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(cmd); err != nil {
		return nil, fmt.Errorf("sending command: %w", err)
	}

	var resp map[string]interface{}
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if errMsg, ok := resp["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("daemon error: %s", errMsg)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	status := &DaemonStatus{
		Running: true,
		Ready:   true,
	}

	if v, ok := result["version"].(string); ok {
		status.Version = v
	}
	if v, ok := result["status"].(string); ok {
		status.Ready = (v == "running")
	}

	return status, nil
}

// CheckStatus checks the daemon status
func CheckStatus() (*DaemonStatus, error) {
	// Check if PID file exists
	if !PIDExists() {
		return &DaemonStatus{
			Running: false,
			Ready:   false,
		}, nil
	}

	// Read PID
	pid, err := ReadPID()
	if err != nil {
		return &DaemonStatus{
			Running: false,
			Ready:   false,
			Error:   fmt.Sprintf("failed to read PID: %v", err),
		}, nil
	}

	// Check if process is running
	if !IsProcessRunning(pid) {
		// Process not running but PID file exists - cleanup
		RemovePID()
		RemoveStatus()
		return &DaemonStatus{
			Running: false,
			Ready:   false,
		}, nil
	}

	// Try to ping the daemon
	status, err := pingDaemon()
	if err != nil {
		// Process running but not responding
		return &DaemonStatus{
			Running: true,
			PID:     pid,
			Ready:   false,
			Error:   fmt.Sprintf("daemon not responding: %v", err),
		}, nil
	}

	status.PID = pid
	return status, nil
}

// IsRunning checks if the daemon is currently running
func IsRunning() bool {
	status, err := CheckStatus()
	return err == nil && status.Running && status.Ready
}
