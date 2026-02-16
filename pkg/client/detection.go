// Package client provides daemon detection and connection management.
package client

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
	// DaemonDirName is the directory for daemon state files
	DaemonDirName = ".gcq"
	// PIDFileName is the name of the PID file
	PIDFileName = "daemon.pid"
	// StatusFileName is the name of the status file
	StatusFileName = "status"
)

// DaemonInfo contains information about a running daemon
type DaemonInfo struct {
	PID        int
	SocketPath string
	Ready      bool
	StartedAt  time.Time
}

// DetectionOptions for daemon detection
type DetectionOptions struct {
	// SocketPath is the Unix socket path to check
	SocketPath string
	// PIDFilePath is the path to the PID file
	PIDFilePath string
	// Timeout is the connection timeout
	Timeout time.Duration
}

// DefaultDetectionOptions returns default detection options
func DefaultDetectionOptions() *DetectionOptions {
	return &DetectionOptions{
		SocketPath: getSocketPath(),
		Timeout:    2 * time.Second,
	}
}

// PIDFile returns the PID file path
func PIDFile() string {
	if path := os.Getenv("GCQ_DAEMON_DIR"); path != "" {
		return filepath.Join(path, PIDFileName)
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, DaemonDirName, PIDFileName)
}

// StatusFile returns the status file path
func StatusFile() string {
	if path := os.Getenv("GCQ_DAEMON_DIR"); path != "" {
		return filepath.Join(path, StatusFileName)
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, DaemonDirName, StatusFileName)
}

// IsRunning checks if the daemon is running by checking both PID file
// and attempting to ping via socket
func IsRunning() bool {
	// First check if PID file exists
	if !pidFileExists() {
		return false
	}

	// Check if process is running
	pid, err := readPID()
	if err != nil {
		return false
	}

	if !isProcessRunning(pid) {
		// Cleanup stale PID file
		RemovePIDFile()
		RemoveStatusFile()
		return false
	}

	// Try to ping the daemon
	if !pingDaemon() {
		return false
	}

	return true
}

// DetectDaemon checks if the daemon is running and returns info
func DetectDaemon(opts *DetectionOptions) (*DaemonInfo, error) {
	if opts == nil {
		opts = DefaultDetectionOptions()
	}

	// Check if PID file exists
	if !pidFileExists() {
		return nil, fmt.Errorf("daemon not running (no PID file)")
	}

	// Read PID
	pid, err := readPID()
	if err != nil {
		return nil, fmt.Errorf("failed to read PID: %w", err)
	}

	// Check if process is running
	if !isProcessRunning(pid) {
		// Cleanup stale files
		RemovePIDFile()
		RemoveStatusFile()
		return nil, fmt.Errorf("daemon not running (process not found)")
	}

	// Try to ping via socket
	socketPath := opts.SocketPath
	if socketPath == "" {
		socketPath = getSocketPath()
	}

	var conn net.Conn
	var dialErr error

	if useTCP() {
		conn, dialErr = net.DialTimeout("tcp", "localhost:"+getTCPPort(), opts.Timeout)
	} else {
		conn, dialErr = net.DialTimeout("unix", socketPath, opts.Timeout)
	}

	if dialErr != nil {
		return &DaemonInfo{
			PID:        pid,
			SocketPath: socketPath,
			Ready:      false,
		}, nil
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(opts.Timeout))

	// Send status command
	cmd := map[string]interface{}{
		"type": "status",
		"id":   "detect",
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(cmd); err != nil {
		return &DaemonInfo{
			PID:        pid,
			SocketPath: socketPath,
			Ready:      false,
		}, nil
	}

	decoder := json.NewDecoder(conn)
	var resp map[string]interface{}
	if err := decoder.Decode(&resp); err != nil {
		return &DaemonInfo{
			PID:        pid,
			SocketPath: socketPath,
			Ready:      false,
		}, nil
	}

	// Check if response indicates success
	if errMsg, ok := resp["error"].(string); ok && errMsg != "" {
		return &DaemonInfo{
			PID:        pid,
			SocketPath: socketPath,
			Ready:      false,
		}, nil
	}

	return &DaemonInfo{
		PID:        pid,
		SocketPath: socketPath,
		Ready:      true,
	}, nil
}

// pingDaemon attempts to ping the daemon
func pingDaemon() bool {
	socketPath := getSocketPath()

	var conn net.Conn
	var err error

	if useTCP() {
		conn, err = net.DialTimeout("tcp", "localhost:"+getTCPPort(), 2*time.Second)
	} else {
		conn, err = net.DialTimeout("unix", socketPath, 2*time.Second)
	}

	if err != nil {
		return false
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))

	cmd := map[string]interface{}{
		"type": "status",
		"id":   "ping",
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(cmd); err != nil {
		return false
	}

	decoder := json.NewDecoder(conn)
	var resp map[string]interface{}
	if err := decoder.Decode(&resp); err != nil {
		return false
	}

	// Any response (even error) means the daemon is listening
	return true
}

// pidFileExists checks if the PID file exists.
// Uses os.Stat error check pattern intentionally - we only care about existence.
func pidFileExists() bool {
	_, err := os.Stat(PIDFile())
	return err == nil
}

// readPID reads the PID from the PID file
func readPID() (int, error) {
	data, err := os.ReadFile(PIDFile())
	if err != nil {
		return 0, err
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, err
	}

	return pid, nil
}

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds
	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// RemovePIDFile removes the PID file
func RemovePIDFile() {
	os.Remove(PIDFile())
}

// RemoveStatusFile removes the status file
func RemoveStatusFile() {
	os.Remove(StatusFile())
}

// SocketExists checks if the socket file exists
func SocketExists() bool {
	socketPath := getSocketPath()
	if useTCP() {
		// For TCP, try to connect
		conn, err := net.DialTimeout("tcp", "localhost:"+getTCPPort(), time.Second)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}

	_, err := os.Stat(socketPath)
	return err == nil
}

// GetDaemonPID returns the daemon PID if running, or 0 if not
func GetDaemonPID() int {
	if !pidFileExists() {
		return 0
	}

	pid, err := readPID()
	if err != nil {
		return 0
	}

	if !isProcessRunning(pid) {
		return 0
	}

	return pid
}
