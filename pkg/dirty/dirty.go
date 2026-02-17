// Package dirty provides file dirty tracking functionality.
// It tracks which files have changed based on content hashing,
// allowing rebuild systems to determine which files need reprocessing.
package dirty

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultCacheDir is the default directory for storing dirty state.
const DefaultCacheDir = ".gcq/cache"

// DefaultCacheFile is the default filename for dirty state.
const DefaultCacheFile = "dirty.json"

// fileState represents the dirty state of a single file.
type fileState struct {
	Path     string `json:"path"`
	Hash     string `json:"hash"`
	IsDirty  bool   `json:"is_dirty"`
	LastSeen int64  `json:"last_seen"` // Unix timestamp
}

// dirtyData is the on-disk JSON structure.
type dirtyData struct {
	Version int         `json:"version"`
	Files   []fileState `json:"files"`
}

// Tracker tracks dirty files based on content hashing.
type Tracker struct {
	mu        sync.RWMutex
	files     map[string]fileState
	cacheDir  string
	cacheFile string
}

// Option configures a Tracker.
type Option func(*Tracker)

// WithCacheDir sets the cache directory.
func WithCacheDir(dir string) Option {
	return func(t *Tracker) {
		t.cacheDir = dir
	}
}

// WithCacheFile sets the cache filename.
func WithCacheFile(file string) Option {
	return func(t *Tracker) {
		t.cacheFile = file
	}
}

// New creates a new Tracker with optional configuration.
func New(opts ...Option) *Tracker {
	t := &Tracker{
		files:     make(map[string]fileState),
		cacheDir:  DefaultCacheDir,
		cacheFile: DefaultCacheFile,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// NewFromCache loads a Tracker from the default cache file.
func NewFromCache() (*Tracker, error) {
	t := New()
	if err := t.Load(); err != nil {
		return nil, err
	}
	return t, nil
}

// computeHash computes SHA256 hash of file contents.
func computeHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("failed to hash file %s: %w", path, err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// MarkDirty marks a file as dirty by computing its hash.
// If the file has changed (different hash), it's marked dirty.
func (t *Tracker) MarkDirty(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	hash, err := computeHash(absPath)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	existing, exists := t.files[absPath]
	if exists && existing.Hash == hash && !existing.IsDirty {
		// File hasn't changed, keep existing state
		return nil
	}

	// Mark as dirty due to new hash or first time
	t.files[absPath] = fileState{
		Path:     absPath,
		Hash:     hash,
		IsDirty:  true,
		LastSeen: time.Now().Unix(),
	}

	return nil
}

// MarkDirtyContext is MarkDirty with context support.
func (t *Tracker) MarkDirtyContext(ctx context.Context, path string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return t.MarkDirty(path)
	}
}

// IsDirty checks if a file is currently marked as dirty.
func (t *Tracker) IsDirty(path string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	state, exists := t.files[absPath]
	return exists && state.IsDirty
}

// GetDirtyFiles returns all files currently marked as dirty.
func (t *Tracker) GetDirtyFiles() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]string, 0, len(t.files))
	for _, state := range t.files {
		if state.IsDirty {
			result = append(result, state.Path)
		}
	}
	return result
}

// ClearDirty clears the dirty flag for specified files after rebuild.
// If no files are provided, all files are cleared.
func (t *Tracker) ClearDirty(files []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(files) == 0 {
		// Clear all
		for i := range t.files {
			state := t.files[i]
			state.IsDirty = false
			t.files[i] = state
		}
		return
	}

	// Clear specific files
	for _, path := range files {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if state, exists := t.files[absPath]; exists {
			state.IsDirty = false
			t.files[absPath] = state
		}
	}
}

// Count returns the number of dirty files.
func (t *Tracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	for _, state := range t.files {
		if state.IsDirty {
			count++
		}
	}
	return count
}

// TotalCount returns the total number of tracked files.
func (t *Tracker) TotalCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.files)
}

// GetHash returns the current hash for a tracked file.
func (t *Tracker) GetHash(path string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}

	state, exists := t.files[absPath]
	return state.Hash, exists
}

// Remove removes a file from tracking.
func (t *Tracker) Remove(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}
	delete(t.files, absPath)
}

// Clear removes all tracked files.
func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.files = make(map[string]fileState)
}

// cachePath returns the full path to the cache file.
func (t *Tracker) cachePath() string {
	return filepath.Join(t.cacheDir, t.cacheFile)
}

// Save persists the dirty state to the cache file.
func (t *Tracker) Save() error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Ensure cache directory exists
	if err := os.MkdirAll(t.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Convert map to slice for JSON
	files := make([]fileState, 0, len(t.files))
	for _, state := range t.files {
		files = append(files, state)
	}

	data := dirtyData{
		Version: 1,
		Files:   files,
	}

	f, err := os.Create(t.cachePath())
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("failed to encode dirty data: %w", err)
	}

	return nil
}

// Load restores the dirty state from the cache file.
func (t *Tracker) Load() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	path := t.cachePath()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file is not an error
		}
		return fmt.Errorf("failed to open cache file: %w", err)
	}
	defer f.Close()

	var data dirtyData
	dec := json.NewDecoder(f)
	if err := dec.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode dirty data: %w", err)
	}

	// Rebuild map
	t.files = make(map[string]fileState, len(data.Files))
	for _, state := range data.Files {
		t.files[state.Path] = state
	}

	return nil
}

// SaveTo writes the dirty state to the given writer.
func (t *Tracker) SaveTo(w io.Writer) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	files := make([]fileState, 0, len(t.files))
	for _, state := range t.files {
		files = append(files, state)
	}

	data := dirtyData{
		Version: 1,
		Files:   files,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// LoadFrom reads the dirty state from the given reader.
func (t *Tracker) LoadFrom(r io.Reader) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var data dirtyData
	dec := json.NewDecoder(r)
	if err := dec.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode dirty data: %w", err)
	}

	t.files = make(map[string]fileState, len(data.Files))
	for _, state := range data.Files {
		t.files[state.Path] = state
	}

	return nil
}

// CheckAndMark checks if a file has changed and marks it dirty if so.
// Returns true if the file was marked dirty (content changed).
func (t *Tracker) CheckAndMark(path string) (bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("failed to get absolute path: %w", err)
	}

	hash, err := computeHash(absPath)
	if err != nil {
		return false, err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	existing, exists := t.files[absPath]
	if exists && existing.Hash == hash {
		// No change, clear dirty flag
		existing.IsDirty = false
		t.files[absPath] = existing
		return false, nil
	}

	// File is new or changed - mark dirty
	t.files[absPath] = fileState{
		Path:     absPath,
		Hash:     hash,
		IsDirty:  true,
		LastSeen: time.Now().Unix(),
	}
	return true, nil
}

// Ensure Tracker implements interface checks
var _ interface{} = (*Tracker)(nil)
