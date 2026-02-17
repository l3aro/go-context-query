// Package client provides command routing with daemon support and fallback.
package client

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const defaultDaemonCacheTTL = 5 * time.Second

// Router routes commands to daemon or executes them directly.
type Router struct {
	client     *Client
	useDaemon  bool
	autoDetect bool

	mu           sync.Mutex
	cachedResult *bool
	cacheTime    time.Time
	cacheTTL     time.Duration
}

// RouterOption is a router option
type RouterOption func(*Router)

// WithDaemon forces using the daemon
func WithDaemon() RouterOption {
	return func(r *Router) {
		r.useDaemon = true
		r.autoDetect = false
	}
}

// WithoutDaemon forces direct execution (no daemon)
func WithoutDaemon() RouterOption {
	return func(r *Router) {
		r.useDaemon = false
		r.autoDetect = false
	}
}

// WithAutoDetect enables automatic daemon detection
func WithAutoDetect() RouterOption {
	return func(r *Router) {
		r.autoDetect = true
	}
}

// NewRouter creates a new command router
func NewRouter(opts ...RouterOption) *Router {
	r := &Router{
		client:     New(),
		useDaemon:  false,
		autoDetect: true, // Default to auto-detect
		cacheTTL:   defaultDaemonCacheTTL,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// ShouldUseDaemon returns true if we should use the daemon
func (r *Router) ShouldUseDaemon() bool {
	if !r.autoDetect {
		return r.useDaemon
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if cache is valid
	if r.cachedResult != nil && time.Since(r.cacheTime) < r.cacheTTL {
		return *r.cachedResult
	}

	// Detect and cache result
	result := IsRunning()
	r.cachedResult = &result
	r.cacheTime = time.Now()
	return result
}

// Search performs a semantic search, routing to daemon or executing directly
func (r *Router) Search(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	if r.ShouldUseDaemon() {
		return r.client.Search(ctx, params)
	}
	return nil, ErrDaemonNotAvailable
}

// Extract extracts code context from a path
func (r *Router) Extract(ctx context.Context, params ExtractParams) (*ExtractResult, error) {
	if r.ShouldUseDaemon() {
		return r.client.Extract(ctx, params)
	}
	return nil, ErrDaemonNotAvailable
}

// Context gets LLM-ready context from entry point
func (r *Router) Context(ctx context.Context, params ContextParams) (*ContextResult, error) {
	if r.ShouldUseDaemon() {
		return r.client.Context(ctx, params)
	}
	return nil, ErrDaemonNotAvailable
}

// Calls gets call graph information for a function
func (r *Router) Calls(ctx context.Context, params CallsParams) (*CallsResult, error) {
	if r.ShouldUseDaemon() {
		return r.client.Calls(ctx, params)
	}
	return nil, ErrDaemonNotAvailable
}

// Warm builds the semantic index for specified paths
func (r *Router) Warm(ctx context.Context, params WarmParams) (*WarmResult, error) {
	if r.ShouldUseDaemon() {
		return r.client.Warm(ctx, params)
	}
	return nil, ErrDaemonNotAvailable
}

// GetStatus gets daemon status
func (r *Router) GetStatus(ctx context.Context) (*DaemonStatus, error) {
	if r.ShouldUseDaemon() {
		return r.client.GetStatus(ctx)
	}
	return nil, ErrDaemonNotAvailable
}

// IsDaemonAvailable checks if daemon is running and available
func (r *Router) IsDaemonAvailable() bool {
	return IsRunning()
}

// GetDaemonInfo gets detailed daemon information
func (r *Router) GetDaemonInfo() (*DaemonInfo, error) {
	return DetectDaemon(nil)
}

// Common errors
var (
	ErrDaemonNotAvailable = fmt.Errorf("daemon not available")
	ErrNotSupported       = fmt.Errorf("operation not supported in direct mode")
)
