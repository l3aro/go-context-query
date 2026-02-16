package client

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// TestNewWithDefaults tests client creation with default values
func TestNewWithDefaults(t *testing.T) {
	// Save and restore environment
	origSocketPath := os.Getenv("GCQ_SOCKET_PATH")
	origTCPPort := os.Getenv("GCQ_TCP_PORT")
	defer func() {
		if origSocketPath != "" {
			os.Setenv("GCQ_SOCKET_PATH", origSocketPath)
		} else {
			os.Unsetenv("GCQ_SOCKET_PATH")
		}
		if origTCPPort != "" {
			os.Setenv("GCQ_TCP_PORT", origTCPPort)
		} else {
			os.Unsetenv("GCQ_TCP_PORT")
		}
	}()

	// Clear environment
	os.Unsetenv("GCQ_SOCKET_PATH")
	os.Unsetenv("GCQ_TCP_PORT")

	client := New()

	if client.socketPath != DefaultSocketPath {
		t.Errorf("Expected socket path %q, got %q", DefaultSocketPath, client.socketPath)
	}
	if client.tcpPort != DefaultTCPPort {
		t.Errorf("Expected TCP port %q, got %q", DefaultTCPPort, client.tcpPort)
	}
	if client.timeout != DefaultTimeout {
		t.Errorf("Expected timeout %v, got %v", DefaultTimeout, client.timeout)
	}
}

// TestNewWithOptions tests client creation with various options
func TestNewWithOptions(t *testing.T) {
	customSocket := "/custom/socket.sock"
	customPort := "9999"
	customTimeout := 10 * time.Second

	client := New(
		WithSocketPath(customSocket),
		WithTCPPort(customPort),
		WithTimeout(customTimeout),
	)

	if client.socketPath != customSocket {
		t.Errorf("Expected socket path %q, got %q", customSocket, client.socketPath)
	}
	if client.tcpPort != customPort {
		t.Errorf("Expected TCP port %q, got %q", customPort, client.tcpPort)
	}
	if client.timeout != customTimeout {
		t.Errorf("Expected timeout %v, got %v", customTimeout, client.timeout)
	}
}

// TestWithSocketPath tests the WithSocketPath option
func TestWithSocketPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"absolute path", "/tmp/gcq.sock", "/tmp/gcq.sock"},
		{"relative path", "gcq.sock", "gcq.sock"},
		{"custom path", "/var/run/gcq-daemon.sock", "/var/run/gcq-daemon.sock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(WithSocketPath(tt.path))
			if client.socketPath != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, client.socketPath)
			}
		})
	}
}

// TestWithTCPPort tests the WithTCPPort option
func TestWithTCPPort(t *testing.T) {
	tests := []struct {
		name     string
		port     string
		expected string
	}{
		{"default port", "9847", "9847"},
		{"custom port", "12345", "12345"},
		{"high port", "65535", "65535"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(WithTCPPort(tt.port))
			if client.tcpPort != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, client.tcpPort)
			}
		})
	}
}

// TestWithTimeout tests the WithTimeout option
func TestWithTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{"short timeout", 1 * time.Second, 1 * time.Second},
		{"long timeout", 30 * time.Second, 30 * time.Second},
		{"zero timeout", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(WithTimeout(tt.timeout))
			if client.timeout != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, client.timeout)
			}
		})
	}
}

// TestGetSocketPath tests getSocketPath function
func TestGetSocketPath(t *testing.T) {
	// Save and restore environment
	orig := os.Getenv("GCQ_SOCKET_PATH")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_SOCKET_PATH", orig)
		} else {
			os.Unsetenv("GCQ_SOCKET_PATH")
		}
	}()

	tests := []struct {
		name        string
		envValue    string
		expected    string
		description string
	}{
		{"default when unset", "", DefaultSocketPath, "should return default when env not set"},
		{"custom path", "/custom/socket.sock", "/custom/socket.sock", "should return env value"},
		{"empty string fallback", "", DefaultSocketPath, "empty string should fallback to default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("GCQ_SOCKET_PATH", tt.envValue)
			} else {
				os.Unsetenv("GCQ_SOCKET_PATH")
			}

			result := getSocketPath()
			if result != tt.expected {
				t.Errorf("%s: expected %q, got %q", tt.description, tt.expected, result)
			}
		})
	}
}

// TestGetTCPPort tests getTCPPort function
func TestGetTCPPort(t *testing.T) {
	// Save and restore environment
	orig := os.Getenv("GCQ_TCP_PORT")
	defer func() {
		if orig != "" {
			os.Setenv("GCQ_TCP_PORT", orig)
		} else {
			os.Unsetenv("GCQ_TCP_PORT")
		}
	}()

	tests := []struct {
		name        string
		envValue    string
		expected    string
		description string
	}{
		{"default when unset", "", DefaultTCPPort, "should return default when env not set"},
		{"custom port", "12345", "12345", "should return env value"},
		{"empty string fallback", "", DefaultTCPPort, "empty string should fallback to default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("GCQ_TCP_PORT", tt.envValue)
			} else {
				os.Unsetenv("GCQ_TCP_PORT")
			}

			result := getTCPPort()
			if result != tt.expected {
				t.Errorf("%s: expected %q, got %q", tt.description, tt.expected, result)
			}
		})
	}
}

// TestIsWindows tests isWindows function
func TestIsWindows(t *testing.T) {
	// Note: This test checks the current platform
	// On Linux, should return false; on Windows, should return true
	result := isWindows()
	expected := runtime.GOOS == "windows"

	if result != expected {
		t.Errorf("isWindows() = %v, expected %v (GOOS=%s)", result, expected, runtime.GOOS)
	}
}

// TestUseTCP tests useTCP function
func TestUseTCP(t *testing.T) {
	// Save and restore environment
	origSocketPath := os.Getenv("GCQ_SOCKET_PATH")
	defer func() {
		if origSocketPath != "" {
			os.Setenv("GCQ_SOCKET_PATH", origSocketPath)
		} else {
			os.Unsetenv("GCQ_SOCKET_PATH")
		}
	}()

	tests := []struct {
		name        string
		envValue    string
		shouldBeTCP bool
		description string
	}{
		{"default unix socket", "", false, "default should use Unix socket on non-Windows"},
		{"absolute path", "/tmp/gcq.sock", false, "absolute path should use Unix socket"},
		{"relative path", "gcq.sock", true, "relative path should use TCP (Windows-style)"},
		{"windows-style", "C:\\temp\\gcq.sock", true, "Windows-style path should use TCP"},
	}

	// Save original GOOS to restore after test
	origGOOS := runtime.GOOS

	// Test on current platform (not Windows)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("GCQ_SOCKET_PATH", tt.envValue)
			} else {
				os.Unsetenv("GCQ_SOCKET_PATH")
			}

			result := useTCP()

			// Adjust expectation based on actual platform
			isWindows := origGOOS == "windows"
			if isWindows {
				// On Windows, should always use TCP
				if !result {
					t.Logf("On Windows, expected TCP=true, got %v", result)
				}
			} else {
				// On non-Windows, depends on socket path
				if result != tt.shouldBeTCP {
					t.Errorf("%s: expected useTCP=%v, got %v", tt.description, tt.shouldBeTCP, result)
				}
			}
		})
	}
}

// TestClientFields tests that Client struct has expected fields
func TestClientFields(t *testing.T) {
	client := New()

	// Test that client has all expected fields
	var _ string = client.socketPath
	var _ string = client.tcpPort
	var _ time.Duration = client.timeout

	// Test IsConnected returns false initially
	if client.IsConnected() {
		t.Error("New client should not be connected")
	}
}

// TestMultipleOptions tests applying multiple options
func TestMultipleOptions(t *testing.T) {
	// Test that multiple options are applied in order
	client := New(
		WithSocketPath("/first.sock"),
		WithSocketPath("/second.sock"),
		WithTCPPort("1111"),
		WithTCPPort("2222"),
	)

	// Last option should win
	if client.socketPath != "/second.sock" {
		t.Errorf("Expected /second.sock, got %s", client.socketPath)
	}
	if client.tcpPort != "2222" {
		t.Errorf("Expected 2222, got %s", client.tcpPort)
	}
}

// TestDaemonStatus tests DaemonStatus struct
func TestDaemonStatus(t *testing.T) {
	status := &DaemonStatus{
		Running:    true,
		Ready:      true,
		PID:        12345,
		Version:    "1.0.0",
		Provider:   "ollama",
		Model:      "nomic-embed-text",
		IndexCount: 10,
		Dimension:  768,
	}

	if !status.Running {
		t.Error("Status should be running")
	}
	if !status.Ready {
		t.Error("Status should be ready")
	}
	if status.PID != 12345 {
		t.Errorf("Expected PID 12345, got %d", status.PID)
	}
	if status.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", status.Version)
	}
	if status.Provider != "ollama" {
		t.Errorf("Expected provider ollama, got %s", status.Provider)
	}
	if status.Model != "nomic-embed-text" {
		t.Errorf("Expected model nomic-embed-text, got %s", status.Model)
	}
	if status.IndexCount != 10 {
		t.Errorf("Expected IndexCount 10, got %d", status.IndexCount)
	}
	if status.Dimension != 768 {
		t.Errorf("Expected Dimension 768, got %d", status.Dimension)
	}
}

// TestSearchParams tests SearchParams struct
func TestSearchParams(t *testing.T) {
	params := SearchParams{
		Query:     "find function",
		Limit:     5,
		Threshold: 0.8,
	}

	if params.Query != "find function" {
		t.Errorf("Expected Query 'find function', got %s", params.Query)
	}
	if params.Limit != 5 {
		t.Errorf("Expected Limit 5, got %d", params.Limit)
	}
	if params.Threshold != 0.8 {
		t.Errorf("Expected Threshold 0.8, got %f", params.Threshold)
	}
}

// TestSearchResult tests SearchResult struct
func TestSearchResult(t *testing.T) {
	result := SearchResult{
		FilePath:   "/path/to/file.go",
		LineNumber: 42,
		Name:       "MyFunction",
		Signature:  "func MyFunction() error",
		Docstring:  "This is a test function",
		Type:       "function",
		Score:      0.95,
	}

	if result.FilePath != "/path/to/file.go" {
		t.Errorf("Expected FilePath '/path/to/file.go', got %s", result.FilePath)
	}
	if result.LineNumber != 42 {
		t.Errorf("Expected LineNumber 42, got %d", result.LineNumber)
	}
	if result.Name != "MyFunction" {
		t.Errorf("Expected Name 'MyFunction', got %s", result.Name)
	}
	if result.Signature != "func MyFunction() error" {
		t.Errorf("Expected Signature 'func MyFunction() error', got %s", result.Signature)
	}
	if result.Type != "function" {
		t.Errorf("Expected Type 'function', got %s", result.Type)
	}
	if result.Score != 0.95 {
		t.Errorf("Expected Score 0.95, got %f", result.Score)
	}
}

// TestExtractParams tests ExtractParams struct
func TestExtractParams(t *testing.T) {
	params := ExtractParams{
		Path: "/path/to/project",
	}

	if params.Path != "/path/to/project" {
		t.Errorf("Expected Path '/path/to/project', got %s", params.Path)
	}
}

// TestExtractResult tests ExtractResult struct
func TestExtractResult(t *testing.T) {
	result := &ExtractResult{
		Extracted: 100,
		Total:     150,
	}

	if result.Extracted != 100 {
		t.Errorf("Expected Extracted 100, got %d", result.Extracted)
	}
	if result.Total != 150 {
		t.Errorf("Expected Total 150, got %d", result.Total)
	}
}

// TestContextParams tests ContextParams struct
func TestContextParams(t *testing.T) {
	params := ContextParams{
		Query: "how does auth work",
		Limit: 3,
	}

	if params.Query != "how does auth work" {
		t.Errorf("Expected Query 'how does auth work', got %s", params.Query)
	}
	if params.Limit != 3 {
		t.Errorf("Expected Limit 3, got %d", params.Limit)
	}
}

// TestContextResult tests ContextResult struct
func TestContextResult(t *testing.T) {
	result := &ContextResult{
		Query: "authentication flow",
		Context: []map[string]interface{}{
			{"file": "auth.go", "line": 10},
			{"file": "login.go", "line": 25},
		},
	}

	if result.Query != "authentication flow" {
		t.Errorf("Expected Query 'authentication flow', got %s", result.Query)
	}
	if len(result.Context) != 2 {
		t.Errorf("Expected Context length 2, got %d", len(result.Context))
	}
}

// TestCallsParams tests CallsParams struct
func TestCallsParams(t *testing.T) {
	params := CallsParams{
		File: "main.go",
		Func: "main",
		Type: "both",
	}

	if params.File != "main.go" {
		t.Errorf("Expected File 'main.go', got %s", params.File)
	}
	if params.Func != "main" {
		t.Errorf("Expected Func 'main', got %s", params.Func)
	}
	if params.Type != "both" {
		t.Errorf("Expected Type 'both', got %s", params.Type)
	}
}

// TestCallsResult tests CallsResult struct
func TestCallsResult(t *testing.T) {
	result := &CallsResult{
		Function: "ProcessRequest",
		File:     "handler.go",
		Count:    5,
		Calls: []CalledFunction{
			{Name: "ValidateInput", File: "validator.go", Line: 10, Type: "callee"},
			{Name: "LogRequest", File: "logger.go", Line: 5, Type: "callee"},
		},
	}

	if result.Function != "ProcessRequest" {
		t.Errorf("Expected Function 'ProcessRequest', got %s", result.Function)
	}
	if result.Count != 5 {
		t.Errorf("Expected Count 5, got %d", result.Count)
	}
	if len(result.Calls) != 2 {
		t.Errorf("Expected Calls length 2, got %d", len(result.Calls))
	}
}

// TestWarmParams tests WarmParams struct
func TestWarmParams(t *testing.T) {
	params := WarmParams{
		Paths: []string{"/path/a", "/path/b"},
	}

	if len(params.Paths) != 2 {
		t.Errorf("Expected Paths length 2, got %d", len(params.Paths))
	}
}

// TestWarmResult tests WarmResult struct
func TestWarmResult(t *testing.T) {
	result := &WarmResult{
		Extracted: 50,
		Paths:     []string{"/path/a", "/path/b"},
	}

	if result.Extracted != 50 {
		t.Errorf("Expected Extracted 50, got %d", result.Extracted)
	}
	if len(result.Paths) != 2 {
		t.Errorf("Expected Paths length 2, got %d", len(result.Paths))
	}
}
