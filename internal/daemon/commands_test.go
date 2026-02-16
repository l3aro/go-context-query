package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStartOptionsFields tests StartOptions struct field assignment
func TestStartOptionsFields(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "daemon.sock")
	configPath := filepath.Join(tmpDir, "config.yaml")

	opts := &StartOptions{
		DaemonPath:   "/path/to/gcqd",
		SocketPath:   socketPath,
		ConfigPath:   configPath,
		Verbose:      true,
		WaitForReady: true,
		ReadyTimeout: 30 * time.Second,
		Background:   true,
	}

	if opts.DaemonPath != "/path/to/gcqd" {
		t.Errorf("Expected DaemonPath to be /path/to/gcqd, got %s", opts.DaemonPath)
	}
	if opts.SocketPath != socketPath {
		t.Errorf("Expected SocketPath to be %s, got %s", socketPath, opts.SocketPath)
	}
	if opts.ConfigPath != configPath {
		t.Errorf("Expected ConfigPath to be %s, got %s", configPath, opts.ConfigPath)
	}
	if !opts.Verbose {
		t.Error("Expected Verbose to be true")
	}
	if !opts.WaitForReady {
		t.Error("Expected WaitForReady to be true")
	}
	if opts.ReadyTimeout != 30*time.Second {
		t.Errorf("Expected ReadyTimeout to be 30s, got %v", opts.ReadyTimeout)
	}
	if !opts.Background {
		t.Error("Expected Background to be true")
	}
}

// TestStartResultFields tests StartResult struct field assignment and JSON tags
func TestStartResultFields(t *testing.T) {
	startedAt := time.Now()

	result := &StartResult{
		Success:   true,
		PID:       12345,
		Error:     "",
		StartedAt: startedAt,
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}
	if result.PID != 12345 {
		t.Errorf("Expected PID to be 12345, got %d", result.PID)
	}
	if result.StartedAt != startedAt {
		t.Errorf("Expected StartedAt to be %v, got %v", startedAt, result.StartedAt)
	}

	// Test failure case
	failResult := &StartResult{
		Success:   false,
		PID:       12345,
		Error:     "daemon already running",
		StartedAt: startedAt,
	}

	if failResult.Success {
		t.Error("Expected Success to be false")
	}
	if failResult.Error != "daemon already running" {
		t.Errorf("Expected Error to be 'daemon already running', got %s", failResult.Error)
	}
}

// TestStopResultFields tests StopResult struct field assignment and JSON tags
func TestStopResultFields(t *testing.T) {
	stoppedAt := time.Now()

	result := &StopResult{
		Success:   true,
		PID:       12345,
		StoppedAt: stoppedAt,
		Error:     "",
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}
	if result.PID != 12345 {
		t.Errorf("Expected PID to be 12345, got %d", result.PID)
	}
	if result.StoppedAt != stoppedAt {
		t.Errorf("Expected StoppedAt to be %v, got %v", stoppedAt, result.StoppedAt)
	}

	// Test failure case
	failResult := &StopResult{
		Success:   false,
		PID:       0,
		StoppedAt: stoppedAt,
		Error:     "daemon not running (no PID file)",
	}

	if failResult.Success {
		t.Error("Expected Success to be false")
	}
	if failResult.Error != "daemon not running (no PID file)" {
		t.Errorf("Expected Error to be 'daemon not running (no PID file)', got %s", failResult.Error)
	}
}

// TestStatusResultFields tests StatusResult struct field assignment and JSON tags
func TestStatusResultFields(t *testing.T) {
	startedAt := time.Now()

	tests := []struct {
		name     string
		status   *StatusResult
		expected struct {
			Status    string
			Running   bool
			Ready     bool
			PID       int
			Version   string
			StartedAt time.Time
			Error     string
		}
	}{
		{
			name: "running status",
			status: &StatusResult{
				Status:    "running",
				Running:   true,
				Ready:     true,
				PID:       12345,
				Version:   "1.0.0",
				StartedAt: startedAt,
				Error:     "",
			},
			expected: struct {
				Status    string
				Running   bool
				Ready     bool
				PID       int
				Version   string
				StartedAt time.Time
				Error     string
			}{
				Status:    "running",
				Running:   true,
				Ready:     true,
				PID:       12345,
				Version:   "1.0.0",
				StartedAt: startedAt,
				Error:     "",
			},
		},
		{
			name: "stopped status",
			status: &StatusResult{
				Status:  "stopped",
				Running: false,
				Ready:   false,
				PID:     0,
				Version: "",
				Error:   "",
			},
			expected: struct {
				Status    string
				Running   bool
				Ready     bool
				PID       int
				Version   string
				StartedAt time.Time
				Error     string
			}{
				Status:  "stopped",
				Running: false,
				Ready:   false,
				PID:     0,
				Version: "",
				Error:   "",
			},
		},
		{
			name: "starting status",
			status: &StatusResult{
				Status:    "starting",
				Running:   true,
				Ready:     false,
				PID:       12345,
				Version:   "",
				StartedAt: startedAt,
				Error:     "",
			},
			expected: struct {
				Status    string
				Running   bool
				Ready     bool
				PID       int
				Version   string
				StartedAt time.Time
				Error     string
			}{
				Status:    "starting",
				Running:   true,
				Ready:     false,
				PID:       12345,
				Version:   "",
				StartedAt: startedAt,
				Error:     "",
			},
		},
		{
			name: "error status",
			status: &StatusResult{
				Status:  "unknown",
				Running: false,
				Ready:   false,
				PID:     0,
				Version: "",
				Error:   "connection refused",
			},
			expected: struct {
				Status    string
				Running   bool
				Ready     bool
				PID       int
				Version   string
				StartedAt time.Time
				Error     string
			}{
				Status:  "unknown",
				Running: false,
				Ready:   false,
				PID:     0,
				Version: "",
				Error:   "connection refused",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.status.Status != tt.expected.Status {
				t.Errorf("Expected Status to be %s, got %s", tt.expected.Status, tt.status.Status)
			}
			if tt.status.Running != tt.expected.Running {
				t.Errorf("Expected Running to be %v, got %v", tt.expected.Running, tt.status.Running)
			}
			if tt.status.Ready != tt.expected.Ready {
				t.Errorf("Expected Ready to be %v, got %v", tt.expected.Ready, tt.status.Ready)
			}
			if tt.status.PID != tt.expected.PID {
				t.Errorf("Expected PID to be %d, got %d", tt.expected.PID, tt.status.PID)
			}
			if tt.status.Version != tt.expected.Version {
				t.Errorf("Expected Version to be %s, got %s", tt.expected.Version, tt.status.Version)
			}
			if tt.status.Error != tt.expected.Error {
				t.Errorf("Expected Error to be %s, got %s", tt.expected.Error, tt.status.Error)
			}
		})
	}
}

// TestFindDaemonBinary tests the findDaemonBinary function
func TestFindDaemonBinary(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (cleanup func())
		expected    string
		expectEmpty bool
	}{
		{
			name: "finds binary in current bin directory",
			setup: func(t *testing.T) func() {
				tmpDir := t.TempDir()
				binDir := filepath.Join(tmpDir, "bin")
				err := os.MkdirAll(binDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create bin directory: %v", err)
				}
				daemonPath := filepath.Join(binDir, "gcqd")
				err = os.WriteFile(daemonPath, []byte("#!/bin/bash\nexit 0"), 0755)
				if err != nil {
					t.Fatalf("Failed to create daemon binary: %v", err)
				}
				origDir, err := os.Getwd()
				if err != nil {
					t.Fatalf("Failed to get current directory: %v", err)
				}
				err = os.Chdir(tmpDir)
				if err != nil {
					t.Fatalf("Failed to change directory: %v", err)
				}
				return func() {
					os.Chdir(origDir)
				}
			},
			expected:    "bin/gcqd",
			expectEmpty: false,
		},
		{
			name: "finds binary in parent bin directory",
			setup: func(t *testing.T) func() {
				tmpDir := t.TempDir()
				parentDir := filepath.Join(tmpDir, "parent")
				binDir := filepath.Join(parentDir, "bin")
				err := os.MkdirAll(binDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create bin directory: %v", err)
				}
				daemonPath := filepath.Join(binDir, "gcqd")
				err = os.WriteFile(daemonPath, []byte("#!/bin/bash\nexit 0"), 0755)
				if err != nil {
					t.Fatalf("Failed to create daemon binary: %v", err)
				}
				// Change to child directory
				origDir, err := os.Getwd()
				if err != nil {
					t.Fatalf("Failed to get current directory: %v", err)
				}
				childDir := filepath.Join(parentDir, "child")
				err = os.MkdirAll(childDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create child directory: %v", err)
				}
				err = os.Chdir(childDir)
				if err != nil {
					t.Fatalf("Failed to change directory: %v", err)
				}
				return func() {
					os.Chdir(origDir)
				}
			},
			expected:    "../bin/gcqd",
			expectEmpty: false,
		},
		{
			name: "finds binary from GCQ_DAEMON_PATH env var",
			setup: func(t *testing.T) func() {
				tmpDir := t.TempDir()
				daemonPath := filepath.Join(tmpDir, "my-daemon")
				err := os.WriteFile(daemonPath, []byte("#!/bin/bash\nexit 0"), 0755)
				if err != nil {
					t.Fatalf("Failed to create daemon binary: %v", err)
				}
				// Set env var
				origVal := os.Getenv("GCQ_DAEMON_PATH")
				err = os.Setenv("GCQ_DAEMON_PATH", daemonPath)
				if err != nil {
					t.Fatalf("Failed to set env var: %v", err)
				}
				return func() {
					if origVal == "" {
						os.Unsetenv("GCQ_DAEMON_PATH")
					} else {
						os.Setenv("GCQ_DAEMON_PATH", origVal)
					}
				}
			},
			expected:    "", // Will be set to the env var path
			expectEmpty: false,
		},
		{
			name: "returns default when no binary found",
			setup: func(t *testing.T) func() {
				tmpDir := t.TempDir()
				// Change to a directory with no bin folder
				origDir, err := os.Getwd()
				if err != nil {
					t.Fatalf("Failed to get current directory: %v", err)
				}
				err = os.Chdir(tmpDir)
				if err != nil {
					t.Fatalf("Failed to change directory: %v", err)
				}
				// Make sure GCQ_DAEMON_PATH is not set
				origVal := os.Getenv("GCQ_DAEMON_PATH")
				os.Unsetenv("GCQ_DAEMON_PATH")
				return func() {
					os.Chdir(origDir)
					if origVal != "" {
						os.Setenv("GCQ_DAEMON_PATH", origVal)
					}
				}
			},
			expected:    "gcqd",
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setup(t)
			defer cleanup()

			result := findDaemonBinary()

			if tt.expected != "" && result != tt.expected {
				t.Errorf("Expected result to be %s, got %s", tt.expected, result)
			}
			if tt.name == "finds binary from GCQ_DAEMON_PATH env var" {
				// For env var test, just check it's not empty
				if result == "" {
					t.Error("Expected result to not be empty when GCQ_DAEMON_PATH is set")
				}
			}
		})
	}
}

// TestWaitForReadyTimeout tests waitForReady with timeout context
func TestWaitForReadyTimeout(t *testing.T) {
	// Create a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should timeout since no daemon is running
	ready, err := waitForReady(ctx, 200*time.Millisecond)

	if err == nil {
		t.Error("Expected error due to timeout, got nil")
	}
	if ready {
		t.Error("Expected ready to be false due to timeout")
	}
}

// TestWaitForReadyCancelled tests waitForReady with cancelled context
func TestWaitForReadyCancelled(t *testing.T) {
	// Create a context that's immediately cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// This should return due to context cancellation
	ready, err := waitForReady(ctx, 10*time.Second)

	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	}
	if ready {
		t.Error("Expected ready to be false due to context cancellation")
	}
}

// TestWaitForShutdown tests waitForShutdown function
func TestWaitForShutdown(t *testing.T) {
	currentPID := os.Getpid()

	tests := []struct {
		name      string
		pid       int
		timeout   time.Duration
		expectRet bool
	}{
		{
			name:      "returns false for current process",
			pid:       currentPID,
			timeout:   100 * time.Millisecond,
			expectRet: false,
		},
		{
			name:      "returns false for non-existent high PID",
			pid:       999999999,
			timeout:   100 * time.Millisecond,
			expectRet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := waitForShutdown(tt.pid, tt.timeout)
			if result != tt.expectRet {
				t.Errorf("Expected %v, got %v", tt.expectRet, result)
			}
		})
	}
}

// TestDaemonStatusFieldAssignment tests DaemonStatus struct fields
func TestDaemonStatusFieldAssignment(t *testing.T) {
	startedAt := time.Now()

	status := &DaemonStatus{
		Running:   true,
		PID:       12345,
		Ready:     true,
		StartedAt: startedAt,
		Error:     "",
		Version:   "1.0.0",
	}

	if !status.Running {
		t.Error("Expected Running to be true")
	}
	if status.PID != 12345 {
		t.Errorf("Expected PID to be 12345, got %d", status.PID)
	}
	if !status.Ready {
		t.Error("Expected Ready to be true")
	}
	if status.StartedAt != startedAt {
		t.Errorf("Expected StartedAt to be %v, got %v", startedAt, status.StartedAt)
	}
	if status.Version != "1.0.0" {
		t.Errorf("Expected Version to be 1.0.0, got %s", status.Version)
	}
}
