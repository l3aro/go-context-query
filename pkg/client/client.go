// Package client provides a client for connecting to the gcq daemon.
// It supports automatic detection of running daemons and graceful fallback
// to direct execution when the daemon is unavailable.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultSocketPath is the default Unix socket path
	DefaultSocketPath = "/tmp/gcq.sock"
	// DefaultTCPPort is the default TCP port for Windows
	DefaultTCPPort = "9847"
	// DefaultTimeout is the default connection timeout
	DefaultTimeout = 5 * time.Second
)

// Client is a daemon client
type Client struct {
	socketPath string
	tcpPort    string
	timeout    time.Duration
	mu         sync.RWMutex
	connected  bool
}

// Option is a client option
type Option func(*Client)

// WithSocketPath sets the socket path
func WithSocketPath(path string) Option {
	return func(c *Client) {
		c.socketPath = path
	}
}

// WithTCPPort sets the TCP port (for Windows)
func WithTCPPort(port string) Option {
	return func(c *Client) {
		c.tcpPort = port
	}
}

// WithTimeout sets the connection timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// New creates a new daemon client
func New(opts ...Option) *Client {
	c := &Client{
		socketPath: getSocketPath(),
		tcpPort:    getTCPPort(),
		timeout:    DefaultTimeout,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// getSocketPath gets the socket path from environment or default
func getSocketPath() string {
	if path := os.Getenv("GCQ_SOCKET_PATH"); path != "" {
		return path
	}
	return DefaultSocketPath
}

// getTCPPort gets the TCP port from environment or default
func getTCPPort() string {
	if port := os.Getenv("GCQ_TCP_PORT"); port != "" {
		return port
	}
	return DefaultTCPPort
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return runtime.GOOS == "windows"
}

// useTCP returns true if we should use TCP instead of Unix sockets
func useTCP() bool {
	if isWindows() {
		return true
	}
	// Check if socket path is not an absolute path (Windows-style)
	socketPath := getSocketPath()
	return !strings.HasPrefix(socketPath, "/")
}

// connect establishes a connection to the daemon
func (c *Client) connect() (net.Conn, error) {
	var conn net.Conn
	var err error

	if useTCP() {
		conn, err = net.Dial("tcp", "localhost:"+c.tcpPort)
	} else {
		conn, err = net.Dial("unix", c.socketPath)
	}

	if err != nil {
		return nil, fmt.Errorf("connecting to daemon: %w", err)
	}

	conn.SetDeadline(time.Now().Add(c.timeout))
	return conn, nil
}

// sendCommand sends a command to the daemon and returns the response
func (c *Client) sendCommand(ctx context.Context, cmdType string, params interface{}) (map[string]interface{}, error) {
	conn, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Set deadline from context if available
	if ctx != nil {
		deadline, ok := ctx.Deadline()
		if ok {
			conn.SetDeadline(deadline)
		}
	} else {
		conn.SetDeadline(time.Now().Add(c.timeout))
	}

	// Create command
	cmd := map[string]interface{}{
		"type": cmdType,
		"id":   generateID(),
	}

	if params != nil {
		paramsJSON, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshaling params: %w", err)
		}
		cmd["params"] = paramsJSON
	}

	// Send command
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(cmd); err != nil {
		return nil, fmt.Errorf("sending command: %w", err)
	}

	// Read response
	decoder := json.NewDecoder(conn)
	var resp map[string]interface{}
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check for errors
	if errMsg, ok := resp["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("daemon error: %s", errMsg)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	return result, nil
}

// generateID generates a unique command ID
func generateID() string {
	return fmt.Sprintf("cmd-%d", time.Now().UnixNano())
}

// DaemonStatus represents daemon status information
type DaemonStatus struct {
	Running    bool   `json:"running"`
	Ready      bool   `json:"ready"`
	PID        int    `json:"pid,omitempty"`
	Version    string `json:"version,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	IndexCount int    `json:"index_count,omitempty"`
	Dimension  int    `json:"dimension,omitempty"`
}

// GetStatus gets the daemon status
func (c *Client) GetStatus(ctx context.Context) (*DaemonStatus, error) {
	result, err := c.sendCommand(ctx, "status", nil)
	if err != nil {
		return nil, err
	}

	status := &DaemonStatus{
		Running: true,
		Ready:   true,
	}

	if v, ok := result["version"].(string); ok {
		status.Version = v
	}
	if v, ok := result["provider"].(string); ok {
		status.Provider = v
	}
	if v, ok := result["model"].(string); ok {
		status.Model = v
	}
	if v, ok := result["index_count"].(float64); ok {
		status.IndexCount = int(v)
	}
	if v, ok := result["dimension"].(float64); ok {
		status.Dimension = int(v)
	}

	return status, nil
}

// SearchParams defines parameters for search
type SearchParams struct {
	Query     string  `json:"query"`
	Limit     int     `json:"limit,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
}

// SearchResult represents a search result
type SearchResult struct {
	FilePath   string  `json:"file"`
	LineNumber int     `json:"line"`
	Name       string  `json:"name"`
	Signature  string  `json:"signature"`
	Docstring  string  `json:"docstring"`
	Type       string  `json:"type"`
	Score      float64 `json:"score"`
}

// Search performs a semantic search
func (c *Client) Search(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}

	result, err := c.sendCommand(ctx, "search", params)
	if err != nil {
		return nil, err
	}

	resultsJSON, ok := result["results"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid search results format")
	}

	results := make([]SearchResult, 0, len(resultsJSON))
	for _, r := range resultsJSON {
		rmap, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		sr := SearchResult{}
		if v, ok := rmap["file"].(string); ok {
			sr.FilePath = v
		}
		if v, ok := rmap["line"].(float64); ok {
			sr.LineNumber = int(v)
		}
		if v, ok := rmap["name"].(string); ok {
			sr.Name = v
		}
		if v, ok := rmap["signature"].(string); ok {
			sr.Signature = v
		}
		if v, ok := rmap["docstring"].(string); ok {
			sr.Docstring = v
		}
		if v, ok := rmap["type"].(string); ok {
			sr.Type = v
		}
		if v, ok := rmap["score"].(float64); ok {
			sr.Score = v
		}

		results = append(results, sr)
	}

	return results, nil
}

// ExtractParams defines parameters for extract
type ExtractParams struct {
	Path string `json:"path"`
}

// ExtractResult represents the result of an extract operation
type ExtractResult struct {
	Extracted int `json:"extracted"`
	Total     int `json:"total"`
}

// Extract extracts code context from a path
func (c *Client) Extract(ctx context.Context, params ExtractParams) (*ExtractResult, error) {
	result, err := c.sendCommand(ctx, "extract", params)
	if err != nil {
		return nil, err
	}

	er := &ExtractResult{}
	if v, ok := result["extracted"].(float64); ok {
		er.Extracted = int(v)
	}
	if v, ok := result["total"].(float64); ok {
		er.Total = int(v)
	}

	return er, nil
}

// ContextParams defines parameters for context query
type ContextParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// ContextResult represents the result of a context query
type ContextResult struct {
	Query   string                   `json:"query"`
	Context []map[string]interface{} `json:"context"`
}

// Context gets LLM-ready context from entry point
func (c *Client) Context(ctx context.Context, params ContextParams) (*ContextResult, error) {
	if params.Limit <= 0 {
		params.Limit = 5
	}

	result, err := c.sendCommand(ctx, "context", params)
	if err != nil {
		return nil, err
	}

	cr := &ContextResult{}
	if v, ok := result["query"].(string); ok {
		cr.Query = v
	}

	if ctxList, ok := result["context"].([]interface{}); ok {
		cr.Context = make([]map[string]interface{}, 0, len(ctxList))
		for _, c := range ctxList {
			if cm, ok := c.(map[string]interface{}); ok {
				cr.Context = append(cr.Context, cm)
			}
		}
	}

	return cr, nil
}

// CallsParams defines parameters for call graph queries
type CallsParams struct {
	File string `json:"file"`
	Func string `json:"func"`
	Type string `json:"type,omitempty"`
}

// CallsResult represents the result of a calls query
type CallsResult struct {
	Function string           `json:"function"`
	File     string           `json:"file"`
	Calls    []CalledFunction `json:"calls"`
	Count    int              `json:"count"`
}

// CalledFunction represents a called function
type CalledFunction struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
	Type string `json:"type"`
}

// Calls gets call graph information for a function
func (c *Client) Calls(ctx context.Context, params CallsParams) (*CallsResult, error) {
	result, err := c.sendCommand(ctx, "calls", params)
	if err != nil {
		return nil, err
	}

	cr := &CallsResult{}
	if v, ok := result["function"].(string); ok {
		cr.Function = v
	}
	if v, ok := result["file"].(string); ok {
		cr.File = v
	}
	if v, ok := result["count"].(float64); ok {
		cr.Count = int(v)
	}

	if callsList, ok := result["calls"].([]interface{}); ok {
		cr.Calls = make([]CalledFunction, 0, len(callsList))
		for _, c := range callsList {
			if cm, ok := c.(map[string]interface{}); ok {
				cf := CalledFunction{}
				if n, ok := cm["name"].(string); ok {
					cf.Name = n
				}
				if f, ok := cm["file"].(string); ok {
					cf.File = f
				}
				if l, ok := cm["line"].(float64); ok {
					cf.Line = int(l)
				}
				if t, ok := cm["type"].(string); ok {
					cf.Type = t
				}
				cr.Calls = append(cr.Calls, cf)
			}
		}
	}

	return cr, nil
}

// WarmParams defines parameters for warm/indexing operation
type WarmParams struct {
	Paths []string `json:"paths"`
}

// WarmResult represents the result of a warm operation
type WarmResult struct {
	Extracted int      `json:"extracted"`
	Paths     []string `json:"paths"`
}

// Warm builds the semantic index for specified paths
func (c *Client) Warm(ctx context.Context, params WarmParams) (*WarmResult, error) {
	result, err := c.sendCommand(ctx, "warm", params)
	if err != nil {
		return nil, err
	}

	wr := &WarmResult{}
	if v, ok := result["extracted"].(float64); ok {
		wr.Extracted = int(v)
	}
	if paths, ok := result["paths"].([]interface{}); ok {
		wr.Paths = make([]string, 0, len(paths))
		for _, p := range paths {
			if ps, ok := p.(string); ok {
				wr.Paths = append(wr.Paths, ps)
			}
		}
	}

	return wr, nil
}

// IsConnected returns whether the client is connected to the daemon
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}
