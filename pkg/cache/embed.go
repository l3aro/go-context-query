// Package cache provides caching utilities for the application.
// This file contains embedding-specific cache implementations.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/vmihailenco/msgpack/v5"
)

// Embedding represents a vector embedding.
type Embedding []float32

// Hash generates a SHA256 hash of the embedding content.
func (e Embedding) Hash() string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", e)))
	return hex.EncodeToString(h.Sum(nil))
}

// EmbeddingEntry represents a cached embedding with metadata.
type EmbeddingEntry struct {
	Vector      Embedding `msgpack:"vector"`
	ContentHash string    `msgpack:"content_hash"`
	Dimensions  int       `msgpack:"dimensions"`
	Model       string    `msgpack:"model"`
	CreatedAt   int64     `msgpack:"created_at"`
}

// EmbeddingCache is a specialized cache for embedding vectors.
// It uses content hashing for efficient lookups.
type EmbeddingCache struct {
	cache *LRUCache
	mu    sync.RWMutex
	model string
	dim   int
}

// EmbeddingCacheOptions configures the embedding cache.
type EmbeddingCacheOptions struct {
	MaxEmbeddings  int
	MaxMemoryBytes int64
	Model          string
	Dimensions     int
	OnEvict        func(hash string, embedding Embedding)
}

// NewEmbeddingCache creates a new embedding cache.
func NewEmbeddingCache(opts EmbeddingCacheOptions) *EmbeddingCache {
	maxBytes := opts.MaxMemoryBytes
	if maxBytes == 0 {
		maxBytes = 100 * 1024 * 1024
	}
	if opts.MaxEmbeddings == 0 {
		opts.MaxEmbeddings = 10000
	}

	ec := &EmbeddingCache{
		model: opts.Model,
		dim:   opts.Dimensions,
		cache: New(Options{
			MaxSize:  opts.MaxEmbeddings,
			MaxBytes: maxBytes,
			OnEvict: func(key string, value interface{}) {
				if opts.OnEvict != nil {
					if entry, ok := value.(EmbeddingEntry); ok {
						opts.OnEvict(key, entry.Vector)
					}
				}
			},
		}),
	}
	return ec
}

// Get retrieves an embedding by content hash.
func (ec *EmbeddingCache) Get(contentHash string) (Embedding, bool) {
	entry, found := ec.cache.Get(contentHash)
	if !found {
		return nil, false
	}
	if e, ok := entry.(EmbeddingEntry); ok {
		return e.Vector, true
	}
	return nil, false
}

// Set stores an embedding in the cache.
func (ec *EmbeddingCache) Set(contentHash string, vector Embedding) {
	entry := EmbeddingEntry{
		Vector:      vector,
		ContentHash: contentHash,
		Dimensions:  len(vector),
		Model:       ec.model,
		CreatedAt:   0,
	}
	ec.cache.Set(contentHash, entry)
}

// Delete removes an embedding from the cache.
func (ec *EmbeddingCache) Delete(contentHash string) {
	ec.cache.Delete(contentHash)
}

// Clear removes all embeddings from the cache.
func (ec *EmbeddingCache) Clear() {
	ec.cache.Clear()
}

// Len returns the number of cached embeddings.
func (ec *EmbeddingCache) Len() int {
	return ec.cache.Len()
}

// Model returns the embedding model name.
func (ec *EmbeddingCache) Model() string {
	return ec.model
}

// Dimensions returns the embedding dimension.
func (ec *EmbeddingCache) Dimensions() int {
	return ec.dim
}

// Stats returns cache statistics.
func (ec *EmbeddingCache) Stats() Stats {
	return Stats{
		Length:       ec.cache.Len(),
		CurrentBytes: ec.cache.CurrentBytes(),
	}
}

// HashString generates a SHA256 hash of a string.
func HashString(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

// HashBytes generates a SHA256 hash of bytes.
func HashBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// Hasher interface for different hash algorithms.
type Hasher interface {
	Hash(data []byte) string
}

// SHA256Hasher uses SHA256 for hashing.
type SHA256Hasher struct{}

func (h *SHA256Hasher) Hash(data []byte) string {
	hasher := sha256.New()
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}

// NewSHA256Hasher creates a new SHA256 hasher.
func NewSHA256Hasher() *SHA256Hasher {
	return &SHA256Hasher{}
}

// EmbeddingCacheWithHasher wraps EmbeddingCache with a custom hasher.
type EmbeddingCacheWithHasher struct {
	*EmbeddingCache
	hasher Hasher
}

// NewEmbeddingCacheWithHasher creates an embedding cache with a custom hasher.
func NewEmbeddingCacheWithHasher(opts EmbeddingCacheOptions, hasher Hasher) *EmbeddingCacheWithHasher {
	return &EmbeddingCacheWithHasher{
		EmbeddingCache: NewEmbeddingCache(opts),
		hasher:         hasher,
	}
}

// GetByContent retrieves an embedding by content (auto-hashes).
func (ech *EmbeddingCacheWithHasher) GetByContent(content string) (Embedding, bool) {
	hash := ech.hasher.Hash([]byte(content))
	return ech.EmbeddingCache.Get(hash)
}

// SetByContent stores an embedding by content (auto-hashes).
func (ech *EmbeddingCacheWithHasher) SetByContent(content string, vector Embedding) {
	hash := ech.hasher.Hash([]byte(content))
	ech.EmbeddingCache.Set(hash, vector)
}

// EmbeddingStore provides a thread-safe embedding storage with persistence.
type EmbeddingStore struct {
	cache *EmbeddingCache
	mu    sync.RWMutex
	path  string
}

// NewEmbeddingStore creates a new embedding store with optional persistence.
func NewEmbeddingStore(opts EmbeddingCacheOptions, path string) *EmbeddingStore {
	es := &EmbeddingStore{
		cache: NewEmbeddingCache(opts),
		path:  path,
	}
	return es
}

// Get retrieves an embedding by hash.
func (es *EmbeddingStore) Get(hash string) (Embedding, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.cache.Get(hash)
}

// Set stores an embedding.
func (es *EmbeddingStore) Set(hash string, vector Embedding) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.cache.Set(hash, vector)
}

// Save persists the store to disk.
func (es *EmbeddingStore) Save() error {
	if es.path == "" {
		return errors.New("no persistence path set")
	}
	es.mu.RLock()
	defer es.mu.RUnlock()

	f, err := os.Create(es.path)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer f.Close()

	return es.saveToFile(f)
}

// Load restores the store from disk.
func (es *EmbeddingStore) Load() error {
	if es.path == "" {
		return nil
	}

	f, err := os.Open(es.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open cache file: %w", err)
	}
	defer f.Close()

	return es.loadFromFile(f)
}

func (es *EmbeddingStore) saveToFile(w io.Writer) error {
	type storeData struct {
		Model      string                    `msgpack:"model"`
		Dimensions int                       `msgpack:"dimensions"`
		Entries    map[string]EmbeddingEntry `msgpack:"entries"`
	}

	data := storeData{
		Model:      es.cache.model,
		Dimensions: es.cache.dim,
		Entries:    make(map[string]EmbeddingEntry),
	}

	// Iterate through the cache entries
	lruCache := es.cache.cache
	lruCache.mu.RLock()
	for _, item := range lruCache.items {
		if entry, ok := item.Value.(EmbeddingEntry); ok {
			data.Entries[entry.ContentHash] = entry
		}
	}
	lruCache.mu.RUnlock()

	enc := msgpack.NewEncoder(w)
	return enc.Encode(data)
}

func (es *EmbeddingStore) loadFromFile(r io.Reader) error {
	type storeData struct {
		Model      string                    `msgpack:"model"`
		Dimensions int                       `msgpack:"dimensions"`
		Entries    map[string]EmbeddingEntry `msgpack:"entries"`
	}

	var data storeData
	dec := msgpack.NewDecoder(r)
	if err := dec.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode store: %w", err)
	}

	for hash, entry := range data.Entries {
		es.cache.Set(hash, entry.Vector)
	}

	return nil
}

// ContextEmbeddingCache provides context-aware embedding cache operations.
type ContextEmbeddingCache struct {
	cache *EmbeddingCache
	ctx   context.Context
}

// NewContextEmbeddingCache creates a new context-aware embedding cache.
func NewContextEmbeddingCache(opts EmbeddingCacheOptions, ctx context.Context) *ContextEmbeddingCache {
	return &ContextEmbeddingCache{
		cache: NewEmbeddingCache(opts),
		ctx:   ctx,
	}
}

// GetWithContext retrieves an embedding with context cancellation.
func (cec *ContextEmbeddingCache) GetWithContext(contentHash string) (Embedding, error) {
	select {
	case <-cec.ctx.Done():
		return nil, cec.ctx.Err()
	default:
		vector, found := cec.cache.Get(contentHash)
		if !found {
			return nil, ErrKeyNotFound
		}
		return vector, nil
	}
}

// SetWithContext stores an embedding with context cancellation.
func (cec *ContextEmbeddingCache) SetWithContext(contentHash string, vector Embedding) error {
	select {
	case <-cec.ctx.Done():
		return cec.ctx.Err()
	default:
		cec.cache.Set(contentHash, vector)
		return nil
	}
}

// BatchEmbeddingCache provides batch operations for embeddings.
type BatchEmbeddingCache struct {
	cache  *EmbeddingCache
	buffer []struct {
		hash   string
		vector Embedding
	}
	mu sync.Mutex
}

// NewBatchEmbeddingCache creates a new batch embedding cache.
func NewBatchEmbeddingCache(opts EmbeddingCacheOptions) *BatchEmbeddingCache {
	return &BatchEmbeddingCache{
		cache: NewEmbeddingCache(opts),
		buffer: make([]struct {
			hash   string
			vector Embedding
		}, 0, 100),
	}
}

// Buffer adds an embedding to the write buffer.
func (bec *BatchEmbeddingCache) Buffer(hash string, vector Embedding) {
	bec.mu.Lock()
	defer bec.mu.Unlock()
	bec.buffer = append(bec.buffer, struct {
		hash   string
		vector Embedding
	}{hash, vector})
}

// Flush writes all buffered embeddings to the cache.
func (bec *BatchEmbeddingCache) Flush() {
	bec.mu.Lock()
	defer bec.mu.Unlock()
	for _, item := range bec.buffer {
		bec.cache.Set(item.hash, item.vector)
	}
	bec.buffer = bec.buffer[:0]
}

// FlushSize returns the current buffer size.
func (bec *BatchEmbeddingCache) FlushSize() int {
	bec.mu.Lock()
	defer bec.mu.Unlock()
	return len(bec.buffer)
}

// Len returns the number of cached embeddings.
func (bec *BatchEmbeddingCache) Len() int {
	bec.mu.Lock()
	defer bec.mu.Unlock()
	return bec.cache.Len() + len(bec.buffer)
}

// EmbeddingCacheStats provides detailed statistics for embedding cache.
type EmbeddingCacheStats struct {
	Count        int     `json:"count"`
	MemoryBytes  int64   `json:"memory_bytes"`
	Model        string  `json:"model"`
	Dimensions   int     `json:"dimensions"`
	AvgVectorLen float64 `json:"avg_vector_length"`
}

// DetailedStats returns detailed embedding cache statistics.
func (ec *EmbeddingCache) DetailedStats() EmbeddingCacheStats {
	stats := EmbeddingCacheStats{
		Count:       ec.cache.Len(),
		MemoryBytes: ec.cache.CurrentBytes(),
		Model:       ec.model,
		Dimensions:  ec.dim,
	}
	if stats.Count > 0 {
		stats.AvgVectorLen = float64(stats.MemoryBytes) / float64(stats.Count) / 4
	}
	return stats
}

// EmbeddingCacheWithMetrics wraps EmbeddingCache with metrics collection.
type EmbeddingCacheWithMetrics struct {
	*EmbeddingCache
	mu        sync.RWMutex
	hits      int64
	misses    int64
	evictions int64
}

// NewEmbeddingCacheWithMetrics creates an embedding cache with metrics.
func NewEmbeddingCacheWithMetrics(opts EmbeddingCacheOptions) *EmbeddingCacheWithMetrics {
	return &EmbeddingCacheWithMetrics{
		EmbeddingCache: NewEmbeddingCache(opts),
	}
}

// Get retrieves an embedding and records metrics.
func (ecm *EmbeddingCacheWithMetrics) Get(hash string) (Embedding, bool) {
	vector, found := ecm.EmbeddingCache.Get(hash)
	ecm.mu.Lock()
	if found {
		ecm.hits++
	} else {
		ecm.misses++
	}
	ecm.mu.Unlock()
	return vector, found
}

// Set stores an embedding.
func (ecm *EmbeddingCacheWithMetrics) Set(hash string, vector Embedding) {
	ecm.EmbeddingCache.Set(hash, vector)
}

// Metrics returns current cache metrics.
func (ecm *EmbeddingCacheWithMetrics) Metrics() (hits, misses, evictions int64) {
	ecm.mu.RLock()
	defer ecm.mu.RUnlock()
	return ecm.hits, ecm.misses, ecm.evictions
}

// HitRate calculates the cache hit rate.
func (ecm *EmbeddingCacheWithMetrics) HitRate() float64 {
	ecm.mu.RLock()
	defer ecm.mu.RUnlock()
	total := ecm.hits + ecm.misses
	if total == 0 {
		return 0
	}
	return float64(ecm.hits) / float64(total)
}

// EmbeddingCacheFromLRU creates an EmbeddingCache from an existing LRU cache.
func EmbeddingCacheFromLRU(cache *LRUCache, model string, dim int) *EmbeddingCache {
	return &EmbeddingCache{
		cache: cache,
		model: model,
		dim:   dim,
	}
}

// ToEmbedding converts a generic interface{} to Embedding.
func ToEmbedding(value interface{}) (Embedding, error) {
	if e, ok := value.(Embedding); ok {
		return e, nil
	}
	if e, ok := value.(EmbeddingEntry); ok {
		return e.Vector, nil
	}
	if e, ok := value.([]float32); ok {
		return e, nil
	}
	return nil, errors.New("invalid embedding type")
}

// EmbeddingFromEntry extracts embedding from a cache entry.
func EmbeddingFromEntry(entry interface{}) (Embedding, bool) {
	switch e := entry.(type) {
	case Embedding:
		return e, true
	case EmbeddingEntry:
		return e.Vector, true
	case []float32:
		return e, true
	default:
		return nil, false
	}
}
