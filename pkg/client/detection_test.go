package client

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestDefaultDetectionOptions(t *testing.T) {
	opts := DefaultDetectionOptions()

	if opts.SocketPath != DefaultSocketPath {
		t.Errorf("Expected SocketPath %q, got %q", DefaultSocketPath, opts.SocketPath)
	}
	if opts.Timeout != 2*time.Second {
		t.Errorf("Expected Timeout %v, got %v", 2*time.Second, opts.Timeout)
	}
}

func TestDetectionOptionsFields(t *testing.T) {
	opts := &DetectionOptions{
		SocketPath:  "/custom/socket",
		PIDFilePath: "/custom/pidfile",
		Timeout:     5 * time.Second,
	}

	if opts.SocketPath != "/custom/socket" {
		t.Errorf("Expected SocketPath '/custom/socket', got %s", opts.SocketPath)
	}
	if opts.PIDFilePath != "/custom/pidfile" {
		t.Errorf("Expected PIDFilePath '/custom/pidfile', got %s", opts.PIDFilePath)
	}
	if opts.Timeout != 5*time.Second {
		t.Errorf("Expected Timeout 5s, got %v", opts.Timeout)
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
	expected := filepath.Join(cwd, DaemonDirName, PIDFileName)

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
	expected := filepath.Join(cwd, DaemonDirName, StatusFileName)

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

func TestPidFileExists(t *testing.T) {
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

	if pidFileExists() {
		t.Error("Expected false when PID file does not exist")
	}

	pidFilePath := filepath.Join(tmpDir, PIDFileName)
	err := os.WriteFile(pidFilePath, []byte("12345"), 0644)
	if err != nil {
		t.Fatalf("Failed to create PID file: %v", err)
	}

	if !pidFileExists() {
		t.Error("Expected true when PID file exists")
	}
}

func TestReadPID(t *testing.T) {
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
		pid       string
		wantPID   int
		wantError bool
	}{
		{"valid pid", "12345", 12345, false},
		{"valid pid with newline", "12345\n", 12345, false},
		{"valid pid with spaces", "  12345  ", 12345, false},
		{"invalid pid - letters", "abc", 0, true},
		{"invalid pid - empty", "", 0, true},
		{"negative is valid in Go Atoi", "-1", -1, false},
		{"zero is valid in Go Atoi", "0", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pidFilePath := filepath.Join(tmpDir, PIDFileName)
			err := os.WriteFile(pidFilePath, []byte(tt.pid), 0644)
			if err != nil {
				t.Fatalf("Failed to create PID file: %v", err)
			}

			pid, err := readPID()
			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if pid != tt.wantPID {
					t.Errorf("Expected PID %d, got %d", tt.wantPID, pid)
				}
			}
		})
	}
}

func TestIsProcessRunning(t *testing.T) {
	currentPID := os.Getpid()

	if !isProcessRunning(currentPID) {
		t.Errorf("Expected process %d to be running", currentPID)
	}

	if isProcessRunning(999999) {
		t.Error("Expected non-existent PID to not be running")
	}

	if isProcessRunning(0) {
		t.Error("Expected PID 0 to not be running")
	}
}

func TestGetDaemonPIDNoFile(t *testing.T) {
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

	pid := GetDaemonPID()
	if pid != 0 {
		t.Errorf("Expected 0 when no PID file, got %d", pid)
	}
}

func TestGetDaemonPIDWithStaleFile(t *testing.T) {
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

	pidFilePath := filepath.Join(tmpDir, PIDFileName)
	os.WriteFile(pidFilePath, []byte("999999"), 0644)

	pid := GetDaemonPID()
	if pid != 0 {
		t.Errorf("Expected 0 for stale PID file, got %d", pid)
	}
}

func TestGetDaemonPIDWithValidPID(t *testing.T) {
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

	currentPID := os.Getpid()
	pidFilePath := filepath.Join(tmpDir, PIDFileName)
	os.WriteFile(pidFilePath, []byte(strconv.Itoa(currentPID)), 0644)

	pid := GetDaemonPID()
	if pid != currentPID {
		t.Errorf("Expected %d, got %d", currentPID, pid)
	}
}

func TestDaemonInfo(t *testing.T) {
	info := &DaemonInfo{
		PID:        12345,
		SocketPath: "/tmp/gcq.sock",
		Ready:      true,
		StartedAt:  time.Now(),
	}

	if info.PID != 12345 {
		t.Errorf("Expected PID 12345, got %d", info.PID)
	}
	if info.SocketPath != "/tmp/gcq.sock" {
		t.Errorf("Expected SocketPath /tmp/gcq.sock, got %s", info.SocketPath)
	}
	if !info.Ready {
		t.Error("Expected Ready to be true")
	}
}

func TestRemovePIDFile(t *testing.T) {
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

	pidFilePath := filepath.Join(tmpDir, PIDFileName)
	os.WriteFile(pidFilePath, []byte("12345"), 0644)

	if _, err := os.Stat(pidFilePath); err != nil {
		t.Fatalf("PID file should exist: %v", err)
	}

	RemovePIDFile()

	if _, err := os.Stat(pidFilePath); err == nil {
		t.Error("PID file should have been removed")
	}
}

func TestRemoveStatusFile(t *testing.T) {
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

	statusFilePath := filepath.Join(tmpDir, StatusFileName)
	os.WriteFile(statusFilePath, []byte("{}"), 0644)

	if _, err := os.Stat(statusFilePath); err != nil {
		t.Fatalf("Status file should exist: %v", err)
	}

	RemoveStatusFile()

	if _, err := os.Stat(statusFilePath); err == nil {
		t.Error("Status file should have been removed")
	}
}

func TestIsRunningNoPIDFile(t *testing.T) {
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

	if IsRunning() {
		t.Error("Expected false when no PID file exists")
	}
}

func TestIsRunningWithStalePIDFile(t *testing.T) {
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

	pidFilePath := filepath.Join(tmpDir, PIDFileName)
	os.WriteFile(pidFilePath, []byte("999999"), 0644)

	result := IsRunning()
	if result {
		t.Error("Expected false for stale PID file")
	}

	if _, err := os.Stat(pidFilePath); err == nil {
		t.Error("Stale PID file should have been cleaned up")
	}
}

func TestIsRunningWithCurrentPID(t *testing.T) {
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

	currentPID := os.Getpid()
	pidFilePath := filepath.Join(tmpDir, PIDFileName)
	os.WriteFile(pidFilePath, []byte(string(rune(currentPID))), 0644)

	result := IsRunning()
	if result {
		t.Log("Daemon appears to be running (likely due to ping check)")
	}
}

func TestConstants(t *testing.T) {
	if DaemonDirName != ".gcq" {
		t.Errorf("Expected DaemonDirName '.gcq', got %s", DaemonDirName)
	}
	if PIDFileName != "daemon.pid" {
		t.Errorf("Expected PIDFileName 'daemon.pid', got %s", PIDFileName)
	}
	if StatusFileName != "status" {
		t.Errorf("Expected StatusFileName 'status', got %s", StatusFileName)
	}
}
