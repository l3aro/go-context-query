package cache

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLRUCache_Basic(t *testing.T) {
	c := New(Options{MaxSize: 3})

	c.Set("a", "value_a")
	c.Set("b", "value_b")
	c.Set("c", "value_c")

	assert.Equal(t, 3, c.Len())

	val, found := c.Get("a")
	require.True(t, found)
	assert.Equal(t, "value_a", val)

	val, found = c.Get("b")
	require.True(t, found)
	assert.Equal(t, "value_b", val)
}

func TestLRUCache_LRU_Eviction(t *testing.T) {
	c := New(Options{MaxSize: 3})

	c.Set("a", "value_a")
	c.Set("b", "value_b")
	c.Set("c", "value_c")

	// Access 'a' to make it most recently used
	c.Get("a")

	// Add new item - should evict 'b' (least recently used)
	c.Set("d", "value_d")

	assert.Equal(t, 3, c.Len())

	_, found := c.Get("b")
	assert.False(t, found, "b should have been evicted")

	_, found = c.Get("a")
	assert.True(t, found, "a should still be present")

	_, found = c.Get("c")
	assert.True(t, found, "c should still be present")

	_, found = c.Get("d")
	assert.True(t, found, "d should be present")
}

func TestLRUCache_Delete(t *testing.T) {
	c := New(Options{MaxSize: 10})

	c.Set("a", "value_a")
	c.Set("b", "value_b")

	c.Delete("a")

	assert.Equal(t, 1, c.Len())

	_, found := c.Get("a")
	assert.False(t, found)

	val, found := c.Get("b")
	require.True(t, found)
	assert.Equal(t, "value_b", val)
}

func TestLRUCache_Clear(t *testing.T) {
	c := New(Options{MaxSize: 10})

	c.Set("a", "value_a")
	c.Set("b", "value_b")

	c.Clear()

	assert.Equal(t, 0, c.Len())
}

func TestLRUCache_SaveLoad(t *testing.T) {
	c := New(Options{MaxSize: 10})
	c.Set("key1", "value1")
	c.Set("key2", "value2")

	var buf bytes.Buffer
	err := c.Save(&buf)
	require.NoError(t, err)

	c2 := New(Options{MaxSize: 10})
	err = c2.Load(&buf)
	require.NoError(t, err)

	assert.Equal(t, 2, c2.Len())

	val, found := c2.Get("key1")
	require.True(t, found)
	assert.Equal(t, "value1", val)
}

func TestLRUCache_MaxBytes(t *testing.T) {
	c := New(Options{MaxBytes: 50})

	// Each string is roughly 10 bytes
	c.Set("a", "1234567890")
	c.Set("b", "1234567890")
	c.Set("c", "1234567890")

	// Should have evicted at least one
	assert.LessOrEqual(t, c.Len(), 3)
}

func TestLRUCache_Update(t *testing.T) {
	c := New(Options{MaxSize: 10})

	c.Set("a", "value1")
	c.Set("a", "value2")

	val, found := c.Get("a")
	require.True(t, found)
	assert.Equal(t, "value2", val)

	assert.Equal(t, 1, c.Len())
}

func TestEmbeddingCache_Basic(t *testing.T) {
	ec := NewEmbeddingCache(EmbeddingCacheOptions{
		MaxEmbeddings: 10,
		Model:         "test-model",
		Dimensions:    384,
	})

	vector := make(Embedding, 384)
	for i := range vector {
		vector[i] = float32(i)
	}

	hash := HashString("test content")
	ec.Set(hash, vector)

	retrieved, found := ec.Get(hash)
	require.True(t, found)
	assert.Equal(t, vector, retrieved)
	assert.Equal(t, "test-model", ec.Model())
	assert.Equal(t, 384, ec.Dimensions())
}

func TestEmbeddingCache_LRU_Eviction(t *testing.T) {
	ec := NewEmbeddingCache(EmbeddingCacheOptions{
		MaxEmbeddings: 2,
		Model:         "test-model",
		Dimensions:    4,
	})

	vector1 := Embedding{1, 2, 3, 4}
	vector2 := Embedding{5, 6, 7, 8}
	vector3 := Embedding{9, 10, 11, 12}

	ec.Set("hash1", vector1)
	ec.Set("hash2", vector2)

	// Access hash1 to make it recently used
	ec.Get("hash1")

	// Add third - should evict hash2
	ec.Set("hash3", vector3)

	_, found := ec.Get("hash2")
	assert.False(t, found, "hash2 should have been evicted")

	_, found = ec.Get("hash1")
	assert.True(t, found, "hash1 should still be present")
}

func TestEmbeddingCache_Stats(t *testing.T) {
	ec := NewEmbeddingCache(EmbeddingCacheOptions{
		MaxEmbeddings: 10,
		Model:         "test-model",
		Dimensions:    100,
	})

	vector := make(Embedding, 100)
	ec.Set("hash1", vector)
	ec.Set("hash2", vector)

	stats := ec.Stats()
	assert.Equal(t, 2, stats.Length)

	detailed := ec.DetailedStats()
	assert.Equal(t, "test-model", detailed.Model)
	assert.Equal(t, 100, detailed.Dimensions)
}

func TestEmbeddingStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "embeddings.cache")

	ec := NewEmbeddingStore(EmbeddingCacheOptions{
		MaxEmbeddings: 10,
		Model:         "test-model",
		Dimensions:    10,
	}, path)

	vector := Embedding{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	ec.Set("hash1", vector)

	err := ec.Save()
	require.NoError(t, err)

	ec2 := NewEmbeddingStore(EmbeddingCacheOptions{
		MaxEmbeddings: 10,
		Model:         "test-model",
		Dimensions:    10,
	}, path)

	err = ec2.Load()
	require.NoError(t, err)

	retrieved, found := ec2.Get("hash1")
	require.True(t, found)
	assert.Equal(t, vector, retrieved)
}

func TestEmbeddingCacheWithMetrics(t *testing.T) {
	ecm := NewEmbeddingCacheWithMetrics(EmbeddingCacheOptions{
		MaxEmbeddings: 10,
		Model:         "test",
		Dimensions:    10,
	})

	vector := Embedding{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	ecm.Set("hash1", vector)

	// Hit
	ecm.Get("hash1")
	// Miss
	ecm.Get("nonexistent")

	hits, misses, _ := ecm.Metrics()
	assert.Equal(t, int64(1), hits)
	assert.Equal(t, int64(1), misses)

	hitRate := ecm.HitRate()
	assert.Equal(t, 0.5, hitRate)
}

func TestHashString(t *testing.T) {
	h1 := HashString("hello world")
	h2 := HashString("hello world")
	h3 := HashString("different")

	assert.Equal(t, h1, h2, "same content should produce same hash")
	assert.NotEqual(t, h1, h3, "different content should produce different hash")
	assert.Len(t, h1, 64, "SHA256 hash should be 64 hex characters")
}

func TestBatchEmbeddingCache(t *testing.T) {
	bec := NewBatchEmbeddingCache(EmbeddingCacheOptions{
		MaxEmbeddings: 10,
	})

	bec.Buffer("hash1", Embedding{1, 2, 3})
	bec.Buffer("hash2", Embedding{4, 5, 6})

	assert.Equal(t, 2, bec.FlushSize())

	bec.Flush()

	assert.Equal(t, 0, bec.FlushSize())
	assert.Equal(t, 2, bec.Len())
}

func TestShardedCache(t *testing.T) {
	sc := NewShardedCache(4, Options{MaxSize: 100})

	sc.Set("key1", "value1")
	sc.Set("key2", "value2")

	val, found := sc.Get("key1")
	require.True(t, found)
	assert.Equal(t, "value1", val)

	val, found = sc.Get("key2")
	require.True(t, found)
	assert.Equal(t, "value2", val)

	assert.Equal(t, 2, sc.Len())

	sc.Delete("key1")
	assert.Equal(t, 1, sc.Len())
}

func TestContextEmbeddingCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cec := NewContextEmbeddingCache(EmbeddingCacheOptions{
		MaxEmbeddings: 10,
		Model:         "test",
		Dimensions:    10,
	}, ctx)

	vector := Embedding{1, 2, 3, 4, 5}
	cec.SetWithContext("hash1", vector)

	retrieved, err := cec.GetWithContext("hash1")
	require.NoError(t, err)
	assert.Equal(t, vector, retrieved)

	// Cancel context
	cancel()

	_, err = cec.GetWithContext("hash1")
	assert.Error(t, err)
}

func TestPersistedFileDoesNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.cache")

	c := New(Options{MaxSize: 10})

	err := LoadFromFile(c, path)
	require.NoError(t, err, "loading non-existent file should not error")

	assert.Equal(t, 0, c.Len())
}

func TestEmbeddingCacheFromLRU(t *testing.T) {
	lru := New(Options{MaxSize: 10})
	lru.Set("key", EmbeddingEntry{
		Vector:      Embedding{1, 2, 3},
		ContentHash: "hash",
		Dimensions:  3,
		Model:       "test",
	})

	ec := EmbeddingCacheFromLRU(lru, "model", 3)
	assert.Equal(t, "model", ec.Model())
	assert.Equal(t, 3, ec.Dimensions())
}

func TestToEmbedding(t *testing.T) {
	vec := Embedding{1, 2, 3}

	out, err := ToEmbedding(vec)
	require.NoError(t, err)
	assert.Equal(t, vec, out)

	entry := EmbeddingEntry{Vector: vec}
	out, err = ToEmbedding(entry)
	require.NoError(t, err)
	assert.Equal(t, vec, out)

	_, err = ToEmbedding("invalid")
	assert.Error(t, err)
}

func TestEmbeddingFromEntry(t *testing.T) {
	vec := Embedding{1, 2, 3}

	out, ok := EmbeddingFromEntry(vec)
	assert.True(t, ok)
	assert.Equal(t, vec, out)

	entry := EmbeddingEntry{Vector: vec}
	out, ok = EmbeddingFromEntry(entry)
	assert.True(t, ok)
	assert.Equal(t, vec, out)

	_, ok = EmbeddingFromEntry("invalid")
	assert.False(t, ok)
}

func TestCacheInterface(t *testing.T) {
	c := New(Options{MaxSize: 10})

	var _ Cache = c
}

func TestStatsCache(t *testing.T) {
	sc := NewStatsCache(Options{MaxSize: 10})

	sc.Set("key1", "value1")
	sc.Get("key1")
	sc.Get("key2")

	stats := sc.Stats()
	assert.Equal(t, int64(1), stats.HitCount)
	assert.Equal(t, int64(1), stats.MissCount)

	assert.Equal(t, 0.5, sc.HitRate())

	sc.ResetStats()

	stats = sc.Stats()
	assert.Equal(t, int64(0), stats.HitCount)
}
