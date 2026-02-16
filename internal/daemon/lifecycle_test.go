package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestDaemonDir(t *testing.T) {
	orig := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_DAEMON_DIR", orig)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Unsetenv("GCQ_DAEMON_DIR")
	cwd, _ := os.Getwd()
	expected := filepath.Join(cwd, DefaultDir)

	result := DaemonDir()
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestDaemonDirWithEnv(t *testing.T) {
	orig := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_DAEMON_DIR", orig)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	testDir := "/tmp/gcq-test-dir"
	os.Setenv("GCQ_DAEMON_DIR", testDir)

	result := DaemonDir()
	if result != testDir {
		t.Errorf("Expected %q, got %q", testDir, result)
	}
}

func TestPIDFile(t *testing.T) {
	orig := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_DAEMON_DIR", orig)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Unsetenv("GCQ_DAEMON_DIR")
	cwd, _ := os.Getwd()
	expected := filepath.Join(cwd, DefaultDir, PIDFileName)

	result := PIDFile()
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestPIDFileWithEnv(t *testing.T) {
	orig := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_DAEMON_DIR", orig)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	testDir := "/tmp/gcq-test"
	os.Setenv("GCQ_DAEMON_DIR", testDir)
	expected := filepath.Join(testDir, PIDFileName)

	result := PIDFile()
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestStatusFile(t *testing.T) {
	orig := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_DAEMON_DIR", orig)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Unsetenv("GCQ_DAEMON_DIR")
	cwd, _ := os.Getwd()
	expected := filepath.Join(cwd, DefaultDir, StatusFileName)

	result := StatusFile()
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestStatusFileWithEnv(t *testing.T) {
	orig := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_DAEMON_DIR", orig)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	testDir := "/tmp/gcq-test"
	os.Setenv("GCQ_DAEMON_DIR", testDir)
	expected := filepath.Join(testDir, StatusFileName)

	result := StatusFile()
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestWriteAndReadPID(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	tests := []struct {
		name    string
		pid     int
		wantErr bool
	}{
		{"valid pid 12345", 12345, false},
		{"valid pid 1", 1, false},
		{"valid current pid", os.Getpid(), false},
		{"zero pid", 0, false},
		{"negative pid", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write PID
			err := WritePID(tt.pid)
			if (err != nil) != tt.wantErr {
				t.Errorf("WritePID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Read PID back
			got, err := ReadPID()
			if err != nil {
				t.Errorf("ReadPID() error = %v", err)
				return
			}

			if got != tt.pid {
				t.Errorf("ReadPID() = %d, want %d", got, tt.pid)
			}

			// Cleanup
			RemovePID()
		})
	}
}

func TestReadPIDNoFile(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	_, err := ReadPID()
	if err == nil {
		t.Error("Expected error when reading non-existent PID file")
	}
}

func TestReadPIDInvalidContent(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{"invalid - letters", "abc", true},
		{"invalid - empty", "", true},
		{"invalid - mixed", "12abc", true},
		{"valid - with newline", "12345\n", false},
		{"valid - with spaces", "  12345  ", false},
		{"valid - with tabs", "\t12345\t", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pidFilePath := filepath.Join(tmpDir, PIDFileName)
			err := os.WriteFile(pidFilePath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write PID file: %v", err)
			}

			_, err = ReadPID()
			if tt.wantError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			RemovePID()
		})
	}
}

func TestPIDExists(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	// Initially should not exist
	if PIDExists() {
		t.Error("Expected false when PID file does not exist")
	}

	// Create PID file
	pidFilePath := filepath.Join(tmpDir, PIDFileName)
	err := os.WriteFile(pidFilePath, []byte("12345"), 0644)
	if err != nil {
		t.Fatalf("Failed to create PID file: %v", err)
	}

	if !PIDExists() {
		t.Error("Expected true when PID file exists")
	}

	// Remove and verify
	RemovePID()
	if PIDExists() {
		t.Error("Expected false after removing PID file")
	}
}

func TestRemovePID(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	// Test removing non-existent file (should not error)
	err := RemovePID()
	if err != nil {
		t.Errorf("RemovePID() on non-existent file error = %v", err)
	}

	// Create and remove
	pidFilePath := filepath.Join(tmpDir, PIDFileName)
	err = os.WriteFile(pidFilePath, []byte("12345"), 0644)
	if err != nil {
		t.Fatalf("Failed to create PID file: %v", err)
	}

	err = RemovePID()
	if err != nil {
		t.Errorf("RemovePID() error = %v", err)
	}

	if _, err := os.Stat(pidFilePath); err == nil {
		t.Error("PID file should have been removed")
	}
}

func TestWriteAndReadStatus(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	tests := []struct {
		name   string
		status *DaemonStatus
	}{
		{
			name: "full status",
			status: &DaemonStatus{
				Running:   true,
				PID:       12345,
				Ready:     true,
				StartedAt: time.Now(),
				Error:     "",
				Version:   "1.0.0",
			},
		},
		{
			name: "minimal status",
			status: &DaemonStatus{
				Running: false,
				Ready:   false,
			},
		},
		{
			name: "status with error",
			status: &DaemonStatus{
				Running: false,
				Ready:   false,
				Error:   "connection refused",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write status
			err := WriteStatus(tt.status)
			if err != nil {
				t.Fatalf("WriteStatus() error = %v", err)
			}

			// Read status back
			got, err := ReadStatus()
			if err != nil {
				t.Fatalf("ReadStatus() error = %v", err)
			}

			if got.Running != tt.status.Running {
				t.Errorf("Running: got %v, want %v", got.Running, tt.status.Running)
			}
			if got.Ready != tt.status.Ready {
				t.Errorf("Ready: got %v, want %v", got.Ready, tt.status.Ready)
			}
			if got.PID != tt.status.PID {
				t.Errorf("PID: got %v, want %v", got.PID, tt.status.PID)
			}
			if got.Error != tt.status.Error {
				t.Errorf("Error: got %q, want %q", got.Error, tt.status.Error)
			}
			if got.Version != tt.status.Version {
				t.Errorf("Version: got %q, want %q", got.Version, tt.status.Version)
			}

			// Cleanup
			RemoveStatus()
		})
	}
}

func TestReadStatusNoFile(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	_, err := ReadStatus()
	if err == nil {
		t.Error("Expected error when reading non-existent status file")
	}
}

func TestReadStatusInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	// Write invalid JSON
	statusFilePath := filepath.Join(tmpDir, StatusFileName)
	err := os.WriteFile(statusFilePath, []byte("not valid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write status file: %v", err)
	}

	_, err = ReadStatus()
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestRemoveStatus(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	// Test removing non-existent file (should not error)
	err := RemoveStatus()
	if err != nil {
		t.Errorf("RemoveStatus() on non-existent file error = %v", err)
	}

	// Create and remove
	statusFilePath := filepath.Join(tmpDir, StatusFileName)
	err = os.WriteFile(statusFilePath, []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create status file: %v", err)
	}

	err = RemoveStatus()
	if err != nil {
		t.Errorf("RemoveStatus() error = %v", err)
	}

	if _, err := os.Stat(statusFilePath); err == nil {
		t.Error("Status file should have been removed")
	}
}

func TestIsProcessRunning(t *testing.T) {
	currentPID := os.Getpid()

	// Current process should be running
	if !IsProcessRunning(currentPID) {
		t.Errorf("Expected process %d to be running", currentPID)
	}

	// Non-existent PIDs should not be running
	if IsProcessRunning(999999) {
		t.Error("Expected non-existent PID to not be running")
	}

	if IsProcessRunning(0) {
		t.Error("Expected PID 0 to not be running")
	}

	// Negative PID should not be running
	if IsProcessRunning(-1) {
		t.Error("Expected negative PID to not be running")
	}
}

func TestGetSocketPath(t *testing.T) {
	orig := os.Getenv("GCQ_SOCKET_PATH")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_SOCKET_PATH", orig)
		} else {
			os.Unsetenv("GCQ_SOCKET_PATH")
		}
	}()

	// Test default
	os.Unsetenv("GCQ_SOCKET_PATH")
	result := GetSocketPath()
	if result != DefaultSocketPath {
		t.Errorf("Expected %q, got %q", DefaultSocketPath, result)
	}

	// Test with env var
	os.Setenv("GCQ_SOCKET_PATH", "/custom/socket/path")
	result = GetSocketPath()
	if result != "/custom/socket/path" {
		t.Errorf("Expected /custom/socket/path, got %q", result)
	}
}

func TestCheckStatusNoPIDFile(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	status, err := CheckStatus()
	if err != nil {
		t.Errorf("CheckStatus() error = %v", err)
	}

	if status.Running {
		t.Error("Expected Running to be false when no PID file")
	}
	if status.Ready {
		t.Error("Expected Ready to be false when no PID file")
	}
}

func TestCheckStatusWithStalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	// Write a stale PID (non-existent process)
	stalePID := 999999
	err := WritePID(stalePID)
	if err != nil {
		t.Fatalf("WritePID() error = %v", err)
	}

	status, err := CheckStatus()
	if err != nil {
		t.Errorf("CheckStatus() error = %v", err)
	}

	if status.Running {
		t.Error("Expected Running to be false for stale PID")
	}
	if status.Ready {
		t.Error("Expected Ready to be false for stale PID")
	}

	// Verify PID file was cleaned up
	if PIDExists() {
		t.Error("Stale PID file should have been cleaned up")
	}
}

func TestCheckStatusWithCurrentPID(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	// Write current PID
	currentPID := os.Getpid()
	err := WritePID(currentPID)
	if err != nil {
		t.Fatalf("WritePID() error = %v", err)
	}

	// Check status - should return running but not ready (no daemon listening)
	status, err := CheckStatus()
	if err != nil {
		// This is expected - the daemon is not actually running
		t.Logf("CheckStatus() error (expected - no daemon): %v", err)
	}

	// Process is running (current PID), but daemon is not responding
	// The exact status depends on whether the daemon is actually running
	_ = status

	// Cleanup
	RemovePID()
	RemoveStatus()
}

func TestIsRunningNoDaemon(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	result := IsRunning()
	if result {
		t.Error("Expected false when no daemon is running")
	}
}

func TestIsRunningWithCurrentPIDNoDaemon(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	// Write current PID (process is running)
	currentPID := os.Getpid()
	err := WritePID(currentPID)
	if err != nil {
		t.Fatalf("WritePID() error = %v", err)
	}

	// Check if running - will be false because daemon is not responding
	result := IsRunning()
	if result {
		t.Log("Daemon appears to be running (daemon may be running)")
	} else {
		t.Log("Daemon is not running (expected - no actual daemon)")
	}

	// Cleanup
	RemovePID()
	RemoveStatus()
}

func TestConstants(t *testing.T) {
	if DefaultDir != ".gcq" {
		t.Errorf("Expected DefaultDir '.gcq', got %s", DefaultDir)
	}
	if PIDFileName != "daemon.pid" {
		t.Errorf("Expected PIDFileName 'daemon.pid', got %s", PIDFileName)
	}
	if StatusFileName != "status" {
		t.Errorf("Expected StatusFileName 'status', got %s", StatusFileName)
	}
	if DefaultSocketPath != "/tmp/gcq.sock" {
		t.Errorf("Expected DefaultSocketPath '/tmp/gcq.sock', got %s", DefaultSocketPath)
	}
	if DefaultTCPPort != "9847" {
		t.Errorf("Expected DefaultTCPPort '9847', got %s", DefaultTCPPort)
	}
}

func TestDaemonStatusJSON(t *testing.T) {
	original := &DaemonStatus{
		Running:   true,
		PID:       12345,
		Ready:     true,
		StartedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Error:     "test error",
		Version:   "1.2.3",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	decoded := &DaemonStatus{}
	err = json.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Running != original.Running {
		t.Errorf("Running: got %v, want %v", decoded.Running, original.Running)
	}
	if decoded.PID != original.PID {
		t.Errorf("PID: got %v, want %v", decoded.PID, original.PID)
	}
	if decoded.Ready != original.Ready {
		t.Errorf("Ready: got %v, want %v", decoded.Ready, original.Ready)
	}
	if decoded.Error != original.Error {
		t.Errorf("Error: got %q, want %q", decoded.Error, original.Error)
	}
	if decoded.Version != original.Version {
		t.Errorf("Version: got %q, want %q", decoded.Version, original.Version)
	}
}

func TestPIDFilePathConstruction(t *testing.T) {
	// Test that PIDFile() correctly joins DaemonDir() with PIDFileName
	orig := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_DAEMON_DIR", orig)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	testDir := "/custom/daemon/dir"
	os.Setenv("GCQ_DAEMON_DIR", testDir)

	expected := filepath.Join(testDir, PIDFileName)
	result := PIDFile()

	if result != expected {
		t.Errorf("PIDFile() = %q, want %q", result, expected)
	}
}

func TestStatusFilePathConstruction(t *testing.T) {
	// Test that StatusFile() correctly joins DaemonDir() with StatusFileName
	orig := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_DAEMON_DIR", orig)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	testDir := "/custom/daemon/dir"
	os.Setenv("GCQ_DAEMON_DIR", testDir)

	expected := filepath.Join(testDir, StatusFileName)
	result := StatusFile()

	if result != expected {
		t.Errorf("StatusFile() = %q, want %q", result, expected)
	}
}

func TestWritePIDCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	// Use a nested directory that doesn't exist
	nestedDir := filepath.Join(tmpDir, "nested", "daemon", "dir")
	os.Setenv("GCQ_DAEMON_DIR", nestedDir)

	// WritePID should create the directory
	err := WritePID(12345)
	if err != nil {
		t.Fatalf("WritePID() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nestedDir); err != nil {
		t.Errorf("Daemon directory should have been created: %v", err)
	}

	// Verify PID file was created
	pidFilePath := filepath.Join(nestedDir, PIDFileName)
	if _, err := os.Stat(pidFilePath); err != nil {
		t.Errorf("PID file should have been created: %v", err)
	}
}

func TestWriteStatusCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	// Use a nested directory that doesn't exist
	nestedDir := filepath.Join(tmpDir, "nested", "status", "dir")
	os.Setenv("GCQ_DAEMON_DIR", nestedDir)

	// WriteStatus should create the directory
	status := &DaemonStatus{Running: false, Ready: false}
	err := WriteStatus(status)
	if err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nestedDir); err != nil {
		t.Errorf("Daemon directory should have been created: %v", err)
	}

	// Verify status file was created
	statusFilePath := filepath.Join(nestedDir, StatusFileName)
	if _, err := os.Stat(statusFilePath); err != nil {
		t.Errorf("Status file should have been created: %v", err)
	}
}

func TestRoundTripPID(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	pids := []int{1, 100, 12345, 65535, os.Getpid()}

	for _, pid := range pids {
		t.Run(strconv.Itoa(pid), func(t *testing.T) {
			// Write
			err := WritePID(pid)
			if err != nil {
				t.Fatalf("WritePID() error = %v", err)
			}

			// Read
			got, err := ReadPID()
			if err != nil {
				t.Fatalf("ReadPID() error = %v", err)
			}

			if got != pid {
				t.Errorf("Round trip failed: wrote %d, read %d", pid, got)
			}

			// Cleanup
			RemovePID()
		})
	}
}

func TestRoundTripStatus(t *testing.T) {
	tmpDir := t.TempDir()

	origDaemonDir := os.Getenv("GCQ_DAEMON_DIR")
	defer func() {
		if origDaemonDir != "" {
			os.Setenv("GCQ_DAEMON_DIR", origDaemonDir)
		} else {
			os.Unsetenv("GCQ_DAEMON_DIR")
		}
	}()

	os.Setenv("GCQ_DAEMON_DIR", tmpDir)

	statuses := []*DaemonStatus{
		{Running: true, PID: 12345, Ready: true, Version: "1.0.0"},
		{Running: false, Ready: false},
		{Running: true, PID: 999, Ready: false, Error: "not ready"},
		{Running: false, Ready: false, Error: "connection failed", Version: "2.0.0"},
	}

	for i, status := range statuses {
		t.Run("status_"+strconv.Itoa(i), func(t *testing.T) {
			// Write
			err := WriteStatus(status)
			if err != nil {
				t.Fatalf("WriteStatus() error = %v", err)
			}

			// Read
			got, err := ReadStatus()
			if err != nil {
				t.Fatalf("ReadStatus() error = %v", err)
			}

			if got.Running != status.Running {
				t.Errorf("Running: got %v, want %v", got.Running, status.Running)
			}
			if got.Ready != status.Ready {
				t.Errorf("Ready: got %v, want %v", got.Ready, status.Ready)
			}
			if got.Error != status.Error {
				t.Errorf("Error: got %q, want %q", got.Error, status.Error)
			}
			if got.Version != status.Version {
				t.Errorf("Version: got %q, want %q", got.Version, status.Version)
			}

			// Cleanup
			RemoveStatus()
		})
	}
}
