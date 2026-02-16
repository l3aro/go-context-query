package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRouterWithDefaults(t *testing.T) {
	router := NewRouter()

	if router.client == nil {
		t.Error("Expected client to be initialized")
	}
	if router.useDaemon {
		t.Error("Expected useDaemon to be false by default")
	}
	if !router.autoDetect {
		t.Error("Expected autoDetect to be true by default")
	}
}

func TestWithDaemonOption(t *testing.T) {
	router := NewRouter(WithDaemon())

	if !router.useDaemon {
		t.Error("Expected useDaemon to be true")
	}
	if router.autoDetect {
		t.Error("Expected autoDetect to be false when WithDaemon is set")
	}
}

func TestWithoutDaemonOption(t *testing.T) {
	router := NewRouter(WithoutDaemon())

	if router.useDaemon {
		t.Error("Expected useDaemon to be false")
	}
	if router.autoDetect {
		t.Error("Expected autoDetect to be false when WithoutDaemon is set")
	}
}

func TestWithAutoDetectOption(t *testing.T) {
	router := NewRouter(WithAutoDetect())

	if router.useDaemon {
		t.Error("Expected useDaemon to be false")
	}
	if !router.autoDetect {
		t.Error("Expected autoDetect to be true")
	}
}

func TestWithDaemonOverridesAutoDetect(t *testing.T) {
	router := NewRouter(WithAutoDetect(), WithDaemon())

	if !router.useDaemon {
		t.Error("Expected useDaemon to be true")
	}
	if router.autoDetect {
		t.Error("Expected autoDetect to be false when WithDaemon overrides")
	}
}

func TestMultipleRouterOptions(t *testing.T) {
	router := NewRouter(
		WithAutoDetect(),
		WithDaemon(),
	)

	if !router.useDaemon {
		t.Error("Expected useDaemon to be true")
	}
	if router.autoDetect {
		t.Error("Expected autoDetect to be false after WithDaemon")
	}
}

func TestShouldUseDaemonWithAutoDetect(t *testing.T) {
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

	router := NewRouter(WithAutoDetect())

	result := router.ShouldUseDaemon()
	if result {
		t.Log("Auto-detect found daemon running (expected if socket exists)")
	}
}

func TestShouldUseDaemonWithForcedDaemon(t *testing.T) {
	router := NewRouter(WithDaemon())

	result := router.ShouldUseDaemon()
	if !result {
		t.Error("Expected ShouldUseDaemon to return true when forced with WithDaemon")
	}
}

func TestShouldUseDaemonWithForcedNoDaemon(t *testing.T) {
	router := NewRouter(WithoutDaemon())

	result := router.ShouldUseDaemon()
	if result {
		t.Error("Expected ShouldUseDaemon to return false when forced with WithoutDaemon")
	}
}

func TestShouldUseDaemonNoPIDFile(t *testing.T) {
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

	router := NewRouter(WithAutoDetect())

	result := router.ShouldUseDaemon()
	if result {
		t.Error("Expected ShouldUseDaemon to return false when no daemon is running")
	}
}

func TestShouldUseDaemonWithStalePID(t *testing.T) {
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

	router := NewRouter(WithAutoDetect())

	result := router.ShouldUseDaemon()
	if result {
		t.Error("Expected ShouldUseDaemon to return false with stale PID")
	}
}

func TestRouterSearchRequiresDaemon(t *testing.T) {
	router := NewRouter(WithoutDaemon())

	_, err := router.Search(nil, SearchParams{Query: "test"})
	if err != ErrDaemonNotAvailable {
		t.Errorf("Expected ErrDaemonNotAvailable, got %v", err)
	}
}

func TestRouterExtractRequiresDaemon(t *testing.T) {
	router := NewRouter(WithoutDaemon())

	_, err := router.Extract(nil, ExtractParams{Path: "/test"})
	if err != ErrDaemonNotAvailable {
		t.Errorf("Expected ErrDaemonNotAvailable, got %v", err)
	}
}

func TestRouterContextRequiresDaemon(t *testing.T) {
	router := NewRouter(WithoutDaemon())

	_, err := router.Context(nil, ContextParams{Query: "test"})
	if err != ErrDaemonNotAvailable {
		t.Errorf("Expected ErrDaemonNotAvailable, got %v", err)
	}
}

func TestRouterCallsRequiresDaemon(t *testing.T) {
	router := NewRouter(WithoutDaemon())

	_, err := router.Calls(nil, CallsParams{File: "main.go", Func: "main"})
	if err != ErrDaemonNotAvailable {
		t.Errorf("Expected ErrDaemonNotAvailable, got %v", err)
	}
}

func TestRouterWarmRequiresDaemon(t *testing.T) {
	router := NewRouter(WithoutDaemon())

	_, err := router.Warm(nil, WarmParams{Paths: []string{"/test"}})
	if err != ErrDaemonNotAvailable {
		t.Errorf("Expected ErrDaemonNotAvailable, got %v", err)
	}
}

func TestRouterGetStatusRequiresDaemon(t *testing.T) {
	router := NewRouter(WithoutDaemon())

	_, err := router.GetStatus(nil)
	if err != ErrDaemonNotAvailable {
		t.Errorf("Expected ErrDaemonNotAvailable, got %v", err)
	}
}

func TestRouterIsDaemonAvailable(t *testing.T) {
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

	router := NewRouter()

	result := router.IsDaemonAvailable()
	if result {
		t.Log("Daemon is available")
	}
}

func TestRouterGetDaemonInfoNoDaemon(t *testing.T) {
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

	router := NewRouter()

	info, err := router.GetDaemonInfo()
	if err == nil {
		t.Logf("GetDaemonInfo returned: %+v", info)
	}
}

func TestRouterStructFields(t *testing.T) {
	router := &Router{
		client:     New(),
		useDaemon:  true,
		autoDetect: false,
	}

	if router.client == nil {
		t.Error("Expected client to be set")
	}
	if !router.useDaemon {
		t.Error("Expected useDaemon to be true")
	}
	if router.autoDetect {
		t.Error("Expected autoDetect to be false")
	}
}

func TestErrDaemonNotAvailable(t *testing.T) {
	if ErrDaemonNotAvailable == nil {
		t.Error("ErrDaemonNotAvailable should not be nil")
	}
	if ErrDaemonNotAvailable.Error() != "daemon not available" {
		t.Errorf("Expected 'daemon not available', got %s", ErrDaemonNotAvailable.Error())
	}
}

func TestErrNotSupported(t *testing.T) {
	if ErrNotSupported == nil {
		t.Error("ErrNotSupported should not be nil")
	}
	if ErrNotSupported.Error() != "operation not supported in direct mode" {
		t.Errorf("Expected 'operation not supported in direct mode', got %s", ErrNotSupported.Error())
	}
}
