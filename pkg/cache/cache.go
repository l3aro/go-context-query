// Package cache provides LRU caching with disk persistence.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

// ErrKeyNotFound is returned when a key is not found in the cache.
var ErrKeyNotFound = errors.New("key not found")

// Cache defines the interface for a cache with basic operations.
type Cache interface {
	// Get retrieves a value by key.
	// Returns (value, true) if found, (nil, false) otherwise.
	Get(key string) (interface{}, bool)

	// Set stores a key-value pair in the cache.
	// If the cache is full, LRU eviction will occur.
	Set(key string, value interface{})

	// Delete removes a key from the cache.
	Delete(key string)

	// Clear removes all entries from the cache.
	Clear()

	// Len returns the number of entries in the cache.
	Len() int

	// Save persists the cache to the given writer.
	Save(w io.Writer) error

	// Load restores the cache from the given reader.
	Load(r io.Reader) error
}

// Entry represents a cache entry with metadata.
type Entry struct {
	Key        string
	Value      interface{}
	AccessedAt time.Time
	CreatedAt  time.Time
	Size       int // estimated size in bytes
}

// LRUCache is an in-memory LRU cache with optional disk persistence.
type LRUCache struct {
	mu           sync.RWMutex
	items        map[string]*listItem
	lru          *list // doubly-linked list (most recent at front)
	maxSize      int
	maxBytes     int64
	currentBytes int64
	onEvict      func(key string, value interface{})
}

// listItem is an item in the doubly-linked list.
type listItem struct {
	Entry
	prev *listItem
	next *listItem
}

// list represents a doubly-linked list.
type list struct {
	head *listItem // most recently accessed
	tail *listItem // least recently accessed
	len  int
}

// newList creates a new doubly-linked list.
func newList() *list {
	return &list{}
}

// moveToFront moves an item to the front (most recently used).
func (l *list) moveToFront(item *listItem) {
	if item == l.head {
		return
	}

	// Remove from current position
	if item.prev != nil {
		item.prev.next = item.next
	}
	if item.next != nil {
		item.next.prev = item.prev
	}
	if item == l.tail {
		l.tail = item.prev
	}

	// Add to front
	item.prev = nil
	item.next = l.head
	if l.head != nil {
		l.head.prev = item
	}
	l.head = item

	if l.tail == nil {
		l.tail = item
	}
}

// removeBack removes and returns the least recently used item.
func (l *list) removeBack() *listItem {
	if l.tail == nil {
		return nil
	}

	item := l.tail
	l.tail = item.prev
	if l.tail != nil {
		l.tail.next = nil
	} else {
		l.head = nil
	}
	l.len--
	return item
}

// pushFront adds an item to the front of the list.
func (l *list) pushFront(item *listItem) {
	item.next = l.head
	item.prev = nil
	if l.head != nil {
		l.head.prev = item
	}
	l.head = item
	if l.tail == nil {
		l.tail = item
	}
	l.len++
}

// Options configures the LRU cache.
type Options struct {
	// MaxSize is the maximum number of entries.
	// 0 means unlimited.
	MaxSize int

	// MaxBytes is the approximate maximum size in bytes.
	// 0 means unlimited.
	MaxBytes int64

	// OnEvict is called when an entry is evicted.
	OnEvict func(key string, value interface{})
}

// New creates a new LRU cache with the given options.
func New(opts Options) *LRUCache {
	c := &LRUCache{
		items:    make(map[string]*listItem),
		lru:      newList(),
		maxSize:  opts.MaxSize,
		maxBytes: opts.MaxBytes,
		onEvict:  opts.OnEvict,
	}
	return c
}

// Get retrieves a value from the cache.
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, found := c.items[key]
	if !found {
		return nil, false
	}

	// Update access time and move to front
	item.AccessedAt = time.Now()
	c.lru.moveToFront(item)
	return item.Value, true
}

// Set stores a value in the cache.
func (c *LRUCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Calculate entry size
	size := estimateSize(value)

	// Check if key already exists
	if item, exists := c.items[key]; exists {
		// Update existing entry
		c.currentBytes -= int64(item.Size)
		item.Value = value
		item.Size = size
		item.AccessedAt = time.Now()
		c.currentBytes += int64(size)
		c.lru.moveToFront(item)
		c.evictIfNeeded()
		return
	}

	// Create new entry
	item := &listItem{
		Entry: Entry{
			Key:        key,
			Value:      value,
			AccessedAt: time.Now(),
			CreatedAt:  time.Now(),
			Size:       size,
		},
	}

	c.items[key] = item
	c.lru.pushFront(item)
	c.currentBytes += int64(size)

	c.evictIfNeeded()
}

// Delete removes a key from the cache.
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, found := c.items[key]
	if !found {
		return
	}

	// Remove from linked list
	if item.prev != nil {
		item.prev.next = item.next
	} else {
		c.lru.head = item.next
	}
	if item.next != nil {
		item.next.prev = item.prev
	} else {
		c.lru.tail = item.prev
	}
	c.lru.len--

	// Remove from map
	delete(c.items, key)
	c.currentBytes -= int64(item.Size)

	// Call eviction callback
	if c.onEvict != nil {
		c.onEvict(key, item.Value)
	}
}

// Clear removes all entries from the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*listItem)
	c.lru = newList()
	c.currentBytes = 0
}

// Len returns the number of entries in the cache.
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// CurrentBytes returns the approximate current size in bytes.
func (c *LRUCache) CurrentBytes() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentBytes
}

// evictIfNeeded evicts entries if the cache exceeds its limits.
func (c *LRUCache) evictIfNeeded() {
	for c.shouldEvict() {
		item := c.lru.removeBack()
		if item == nil {
			break
		}
		delete(c.items, item.Key)
		c.currentBytes -= int64(item.Size)

		if c.onEvict != nil {
			c.onEvict(item.Key, item.Value)
		}
	}
}

// shouldEvict returns true if the cache should evict entries.
func (c *LRUCache) shouldEvict() bool {
	if c.maxSize > 0 && c.lru.len > c.maxSize {
		return true
	}
	if c.maxBytes > 0 && c.currentBytes >= c.maxBytes {
		return true
	}
	return false
}

// Save persists the cache to a writer using msgpack.
func (c *LRUCache) Save(w io.Writer) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Convert to serializable format
	entries := make([]Entry, 0, len(c.items))
	for _, item := range c.items {
		entries = append(entries, item.Entry)
	}

	enc := msgpack.NewEncoder(w)
	return enc.Encode(entries)
}

// Load restores the cache from a reader using msgpack.
func (c *LRUCache) Load(r io.Reader) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var entries []Entry
	dec := msgpack.NewDecoder(r)
	if err := dec.Decode(&entries); err != nil {
		return fmt.Errorf("failed to decode cache: %w", err)
	}

	// Clear existing items
	c.items = make(map[string]*listItem)
	c.lru = newList()
	c.currentBytes = 0

	// Restore items
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		item := &listItem{Entry: entry}
		c.items[entry.Key] = item
		c.lru.pushFront(item)
		c.currentBytes += int64(entry.Size)
	}

	return nil
}

// SaveJSON persists the cache to a writer using JSON.
func (c *LRUCache) SaveJSON(w io.Writer) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]Entry, 0, len(c.items))
	for _, item := range c.items {
		entries = append(entries, item.Entry)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

// LoadJSON restores the cache from a reader using JSON.
func (c *LRUCache) LoadJSON(r io.Reader) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var entries []Entry
	dec := json.NewDecoder(r)
	if err := dec.Decode(&entries); err != nil {
		return fmt.Errorf("failed to decode cache: %w", err)
	}

	c.items = make(map[string]*listItem)
	c.lru = newList()
	c.currentBytes = 0

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		item := &listItem{Entry: entry}
		c.items[entry.Key] = item
		c.lru.pushFront(item)
		c.currentBytes += int64(entry.Size)
	}

	return nil
}

// PersistToFile saves the cache to a file.
func PersistToFile(c Cache, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer f.Close()

	if lru, ok := c.(*LRUCache); ok {
		return lru.Save(f)
	}
	return c.Save(f)
}

// LoadFromFile loads the cache from a file.
func LoadFromFile(c *LRUCache, path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file is not an error
		}
		return fmt.Errorf("failed to open cache file: %w", err)
	}
	defer f.Close()

	return c.Load(f)
}

// estimateSize estimates the size of a value in bytes.
func estimateSize(value interface{}) int {
	switch v := value.(type) {
	case string:
		return len(v)
	case []byte:
		return len(v)
	case []float32:
		return len(v) * 4
	case []float64:
		return len(v) * 8
	case []int:
		return len(v) * 8
	case []int32:
		return len(v) * 4
	case []int64:
		return len(v) * 8
	default:
		// Rough estimate for complex types
		b, _ := json.Marshal(v)
		return len(b)
	}
}

// cacheData is used for JSON serialization.
type cacheData struct {
	Entries []Entry `json:"entries"`
	Version int     `json:"version"`
}

// SaveWithMetadata saves cache with metadata.
func (c *LRUCache) SaveWithMetadata(w io.Writer) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data := cacheData{
		Version: 1,
		Entries: make([]Entry, 0, len(c.items)),
	}

	for _, item := range c.items {
		data.Entries = append(data.Entries, item.Entry)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// LoadWithMetadata loads cache with metadata.
func (c *LRUCache) LoadWithMetadata(r io.Reader) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var data cacheData
	dec := json.NewDecoder(r)
	if err := dec.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode cache: %w", err)
	}

	c.items = make(map[string]*listItem)
	c.lru = newList()
	c.currentBytes = 0

	for i := len(data.Entries) - 1; i >= 0; i-- {
		entry := data.Entries[i]
		item := &listItem{Entry: entry}
		c.items[entry.Key] = item
		c.lru.pushFront(item)
		c.currentBytes += int64(entry.Size)
	}

	return nil
}

// WithContext wraps a cache with context support for cancelable operations.
func WithContext(ctx context.Context, c Cache) *ContextCache {
	return &ContextCache{
		Cache: c,
		ctx:   ctx,
	}
}

// ContextCache wraps a cache with context support.
type ContextCache struct {
	Cache
	ctx context.Context
}

// GetWithContext retrieves a value with context cancellation support.
func (c *ContextCache) GetWithContext(key string) (interface{}, bool, error) {
	select {
	case <-c.ctx.Done():
		return nil, false, c.ctx.Err()
	default:
		val, found := c.Cache.Get(key)
		return val, found, nil
	}
}

// SetWithContext stores a value with context cancellation support.
func (c *ContextCache) SetWithContext(key string, value interface{}) error {
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		c.Cache.Set(key, value)
		return nil
	}
}

// NewWithContext creates a new context-aware cache.
func NewWithContext(ctx context.Context, opts Options) *ContextCache {
	return WithContext(ctx, New(opts))
}

// Stats returns cache statistics.
type Stats struct {
	Length       int   `json:"length"`
	CurrentBytes int64 `json:"current_bytes"`
	HitCount     int64 `json:"hit_count"`
	MissCount    int64 `json:"miss_count"`
}

// NewStatsCache creates a cache that tracks statistics.
func NewStatsCache(opts Options) *StatsCache {
	sc := &StatsCache{
		LRUCache: New(opts),
	}
	return sc
}

// StatsCache wraps an LRU cache with statistics tracking.
type StatsCache struct {
	*LRUCache
	mu        sync.RWMutex
	hitCount  int64
	missCount int64
}

// Get retrieves a value and updates statistics.
func (c *StatsCache) Get(key string) (interface{}, bool) {
	val, found := c.LRUCache.Get(key)
	c.mu.Lock()
	if found {
		c.hitCount++
	} else {
		c.missCount++
	}
	c.mu.Unlock()
	return val, found
}

// Stats returns the current cache statistics.
func (c *StatsCache) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Stats{
		Length:       c.LRUCache.Len(),
		CurrentBytes: c.LRUCache.CurrentBytes(),
		HitCount:     c.hitCount,
		MissCount:    c.missCount,
	}
}

// HitRate returns the cache hit rate.
func (c *StatsCache) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := c.hitCount + c.missCount
	if total == 0 {
		return 0
	}
	return float64(c.hitCount) / float64(total)
}

// ResetStats resets the statistics counters.
func (c *StatsCache) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hitCount = 0
	c.missCount = 0
}

// NewShardedCache creates a cache with multiple shards for better concurrency.
func NewShardedCache(numShards int, opts Options) *ShardedCache {
	shards := make([]*LRUCache, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = New(opts)
	}
	return &ShardedCache{
		shards: shards,
	}
}

// ShardedCache is a sharded LRU cache for better concurrency.
type ShardedCache struct {
	shards []*LRUCache
}

// shardIndex returns the shard index for a key.
func (s *ShardedCache) shardIndex(key string) uint32 {
	// Simple hash function
	var hash uint32
	for _, c := range key {
		hash = hash*31 + uint32(c)
	}
	return hash % uint32(len(s.shards))
}

// Get retrieves a value from the appropriate shard.
func (s *ShardedCache) Get(key string) (interface{}, bool) {
	idx := s.shardIndex(key)
	return s.shards[idx].Get(key)
}

// Set sets a value in the appropriate shard.
func (s *ShardedCache) Set(key string, value interface{}) {
	idx := s.shardIndex(key)
	s.shards[idx].Set(key, value)
}

// Delete deletes a key from the appropriate shard.
func (s *ShardedCache) Delete(key string) {
	idx := s.shardIndex(key)
	s.shards[idx].Delete(key)
}

// Clear clears all shards.
func (s *ShardedCache) Clear() {
	for _, shard := range s.shards {
		shard.Clear()
	}
}

// Len returns the total number of entries across all shards.
func (s *ShardedCache) Len() int {
	total := 0
	for _, shard := range s.shards {
		total += shard.Len()
	}
	return total
}

// Save saves all shards to a writer.
func (s *ShardedCache) Save(w io.Writer) error {
	enc := msgpack.NewEncoder(w)

	type shardData struct {
		Entries []Entry `msgpack:"entries"`
	}

	allData := make([]shardData, len(s.shards))
	for i, shard := range s.shards {
		shard.mu.RLock()
		entries := make([]Entry, 0, len(shard.items))
		for _, item := range shard.items {
			entries = append(entries, item.Entry)
		}
		allData[i] = shardData{Entries: entries}
		shard.mu.RUnlock()
	}

	return enc.Encode(allData)
}

// Load loads all shards from a reader.
func (s *ShardedCache) Load(r io.Reader) error {
	type shardData struct {
		Entries []Entry `msgpack:"entries"`
	}

	var allData []shardData
	dec := msgpack.NewDecoder(r)
	if err := dec.Decode(&allData); err != nil {
		return err
	}

	for i, data := range allData {
		if i >= len(s.shards) {
			break
		}
		shard := s.shards[i]
		shard.mu.Lock()
		shard.items = make(map[string]*listItem)
		shard.lru = newList()
		shard.currentBytes = 0

		for j := len(data.Entries) - 1; j >= 0; j-- {
			entry := data.Entries[j]
			item := &listItem{Entry: entry}
			shard.items[entry.Key] = item
			shard.lru.pushFront(item)
			shard.currentBytes += int64(entry.Size)
		}
		shard.mu.Unlock()
	}

	return nil
}

// Ensure LRUCache implements Cache interface
var _ Cache = (*LRUCache)(nil)

// Ensure ContextCache implements extended functionality
var _ interface{} = (*ContextCache)(nil)

// Ensure StatsCache implements extended functionality
var _ interface{} = (*StatsCache)(nil)

// Ensure ShardedCache implements Cache interface
var _ Cache = (*ShardedCache)(nil)

// CalculateEmbeddingSize calculates the size of an embedding vector in bytes.
func CalculateEmbeddingSize(dimensions int) int {
	return dimensions * 4 // float32 = 4 bytes
}

// EmbeddingSizeFromVector calculates size from an actual vector.
func EmbeddingSizeFromVector(v []float32) int {
	return len(v) * 4
}

// DefaultMaxEmbeddings returns a reasonable default for max embeddings
// given available memory.
func DefaultMaxEmbeddings(availableMemoryMB int) int {
	// Assume average embedding is 768 dimensions (3KB)
	// Use 80% of available memory
	bytesPerEmbedding := 3 * 1024
	maxEmbeddings := (availableMemoryMB * 1024 * 1024 * 80 / 100) / bytesPerEmbedding
	return int(math.Min(float64(maxEmbeddings), 100000))
}
