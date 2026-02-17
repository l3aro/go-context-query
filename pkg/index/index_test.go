package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

func TestNewVectorIndex(t *testing.T) {
	tests := []struct {
		name      string
		dimension int
		wantDim   int
		wantCnt   int
	}{
		{"zero dimension", 0, 0, 0},
		{"3 dimension", 3, 3, 0},
		{"128 dimension", 128, 128, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := NewVectorIndex(tt.dimension)
			if idx.Dimension() != tt.wantDim {
				t.Errorf("Dimension() = %d, want %d", idx.Dimension(), tt.wantDim)
			}
			if idx.Count() != tt.wantCnt {
				t.Errorf("Count() = %d, want %d", idx.Count(), tt.wantCnt)
			}
		})
	}
}

func TestVectorIndexAdd(t *testing.T) {
	idx := NewVectorIndex(3)

	// Test adding valid vectors
	err := idx.Add("doc1", []float32{1.0, 0.0, 0.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "main.py"},
	})
	if err != nil {
		t.Fatalf("Add() unexpected error: %v", err)
	}

	err = idx.Add("doc2", []float32{0.0, 1.0, 0.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "utils.py"},
	})
	if err != nil {
		t.Fatalf("Add() unexpected error: %v", err)
	}

	if idx.Count() != 2 {
		t.Errorf("Count() = %d, want 2", idx.Count())
	}

	// Test dimension mismatch error
	err = idx.Add("doc3", []float32{1.0, 0.0}, types.EmbeddingUnit{})
	if err == nil {
		t.Error("Add() expected error for dimension mismatch, got nil")
	}

	// Test empty vector
	err = idx.Add("doc4", []float32{}, types.EmbeddingUnit{})
	if err == nil {
		t.Error("Add() expected error for empty vector, got nil")
	}
}

func TestVectorIndexSearch(t *testing.T) {
	idx := NewVectorIndex(3)

	// Add test vectors
	idx.Add("doc1", []float32{1.0, 0.0, 0.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "main.py"},
	})
	idx.Add("doc2", []float32{0.0, 1.0, 0.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "utils.py"},
	})
	idx.Add("doc3", []float32{0.0, 0.0, 1.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "helpers.py"},
	})

	// Test search with query close to doc1
	results, err := idx.Search([]float32{0.9, 0.1, 0.0}, 2)
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2", len(results))
	}

	// First result should be doc1 (highest similarity to [0.9, 0.1, 0.0])
	if results[0].ID != "doc1" {
		t.Errorf("results[0].ID = %q, want %q", results[0].ID, "doc1")
	}

	// Score should be high for doc1
	if results[0].Score < 0.9 {
		t.Errorf("results[0].Score = %v, want >= 0.9", results[0].Score)
	}

	// Test search with dimension mismatch
	_, err = idx.Search([]float32{0.9, 0.1}, 2)
	if err == nil {
		t.Error("Search() expected error for dimension mismatch, got nil")
	}

	// Test search with k <= 0
	_, err = idx.Search([]float32{0.9, 0.1, 0.0}, 0)
	if err == nil {
		t.Error("Search() expected error for k <= 0, got nil")
	}

	// Test search with k > count (should return all)
	results, err = idx.Search([]float32{0.9, 0.1, 0.0}, 100)
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("len(results) = %d, want 3", len(results))
	}
}

func TestVectorIndexSearchCosineSimilarity(t *testing.T) {
	// Test that cosine similarity is computed correctly
	// For orthogonal vectors (90 degrees), similarity should be 0
	// For identical normalized vectors, similarity should be 1
	// For similar vectors, similarity should be high

	idx := NewVectorIndex(3)

	// Add normalized vectors
	idx.Add("x", []float32{1.0, 0.0, 0.0}, types.EmbeddingUnit{})
	idx.Add("y", []float32{0.0, 1.0, 0.0}, types.EmbeddingUnit{})
	idx.Add("z", []float32{0.0, 0.0, 1.0}, types.EmbeddingUnit{})

	// Search for x-axis should return x first with high score
	results, _ := idx.Search([]float32{1.0, 0.0, 0.0}, 1)
	if results[0].ID != "x" {
		t.Errorf("Expected x, got %s", results[0].ID)
	}
	if results[0].Score < 0.99 {
		t.Errorf("Score for identical vector = %v, want ~1.0", results[0].Score)
	}

	// Search for y-axis should return y first
	results, _ = idx.Search([]float32{0.0, 1.0, 0.0}, 1)
	if results[0].ID != "y" {
		t.Errorf("Expected y, got %s", results[0].ID)
	}

	// Search for diagonal should return closest match
	results, _ = idx.Search([]float32{0.577, 0.577, 0.577}, 3)
	// All three should have similar scores since it's equidistant
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}

func TestVectorIndexGet(t *testing.T) {
	idx := NewVectorIndex(3)

	idx.Add("doc1", []float32{1.0, 2.0, 3.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "main.py"},
	})

	// Test getting existing vector (returns normalized vector)
	vector, metadata, found := idx.Get("doc1")
	if !found {
		t.Error("Get() expected to find doc1")
	}
	if len(vector) != 3 {
		t.Errorf("vector length = %d, want 3", len(vector))
	}
	// Vector is normalized: [1,2,3] -> [0.267, 0.535, 0.802]
	expected := []float32{0.26726124, 0.5345225, 0.8017837}
	if vector[0] != expected[0] || vector[1] != expected[1] || vector[2] != expected[2] {
		t.Errorf("vector = %v, want %v", vector, expected)
	}
	if metadata.L1Data.Path != "main.py" {
		t.Errorf("metadata.L1Data.Path = %q, want %q", metadata.L1Data.Path, "main.py")
	}

	// Test getting non-existing vector
	_, _, found = idx.Get("nonexistent")
	if found {
		t.Error("Get() expected not to find nonexistent")
	}
}

func TestVectorIndexDelete(t *testing.T) {
	idx := NewVectorIndex(3)

	idx.Add("doc1", []float32{1.0, 0.0, 0.0}, types.EmbeddingUnit{})
	idx.Add("doc2", []float32{0.0, 1.0, 0.0}, types.EmbeddingUnit{})

	if idx.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", idx.Count())
	}

	// Delete existing vector
	deleted := idx.Delete("doc1")
	if !deleted {
		t.Error("Delete() expected to return true for existing vector")
	}
	if idx.Count() != 1 {
		t.Errorf("Count() after delete = %d, want 1", idx.Count())
	}

	// Verify doc1 is gone
	_, _, found := idx.Get("doc1")
	if found {
		t.Error("Get() found deleted vector")
	}

	// Verify doc2 still exists
	_, _, found = idx.Get("doc2")
	if !found {
		t.Error("Get() should still find doc2")
	}

	// Delete non-existing vector
	deleted = idx.Delete("nonexistent")
	if deleted {
		t.Error("Delete() expected to return false for nonexistent vector")
	}
}

func TestVectorIndexClear(t *testing.T) {
	idx := NewVectorIndex(3)

	idx.Add("doc1", []float32{1.0, 0.0, 0.0}, types.EmbeddingUnit{})
	idx.Add("doc2", []float32{0.0, 1.0, 0.0}, types.EmbeddingUnit{})

	if idx.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", idx.Count())
	}

	idx.Clear()

	if idx.Count() != 0 {
		t.Errorf("Count() after Clear() = %d, want 0", idx.Count())
	}

	// Verify vectors are gone
	_, _, found := idx.Get("doc1")
	if found {
		t.Error("Get() found vector after Clear()")
	}
}

func TestVectorIndexSaveLoad(t *testing.T) {
	idx := NewVectorIndex(3)

	// Add test data
	idx.Add("doc1", []float32{1.0, 0.0, 0.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "main.py"},
	})
	idx.Add("doc2", []float32{0.0, 1.0, 0.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "utils.py"},
	})

	// Save to temp file
	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "test_index.msgpack")

	err := idx.Save(savePath)
	if err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Error("Save() did not create file")
	}

	// Load into new index
	idx2 := NewVectorIndex(3)
	err = idx2.Load(savePath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	// Verify data integrity
	if idx2.Count() != 2 {
		t.Errorf("Count() after load = %d, want 2", idx2.Count())
	}
	if idx2.Dimension() != 3 {
		t.Errorf("Dimension() after load = %d, want 3", idx2.Dimension())
	}

	// Verify vector values
	vector, _, found := idx2.Get("doc1")
	if !found {
		t.Error("Loaded index missing doc1")
	}
	if vector[0] != 1.0 || vector[1] != 0.0 || vector[2] != 0.0 {
		t.Errorf("doc1 vector = %v, want [1.0, 0.0, 0.0]", vector)
	}

	// Verify metadata
	_, metadata, _ := idx2.Get("doc2")
	if metadata.L1Data.Path != "utils.py" {
		t.Errorf("doc2 metadata.Path = %q, want %q", metadata.L1Data.Path, "utils.py")
	}

	// Verify search works on loaded index
	results, err := idx2.Search([]float32{0.9, 0.1, 0.0}, 1)
	if err != nil {
		t.Fatalf("Search() on loaded index failed: %v", err)
	}
	if results[0].ID != "doc1" {
		t.Errorf("Search() on loaded index returned %q, want %q", results[0].ID, "doc1")
	}
}

func TestVectorIndexLoadOrNew(t *testing.T) {
	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "new_index.msgpack")

	// Test creating new index when file doesn't exist
	idx, err := LoadOrNew(savePath, 3)
	if err != nil {
		t.Fatalf("LoadOrNew() unexpected error: %v", err)
	}
	if idx.Dimension() != 3 {
		t.Errorf("Dimension() = %d, want 3", idx.Dimension())
	}
	if idx.Count() != 0 {
		t.Errorf("Count() = %d, want 0", idx.Count())
	}

	// Add data and save
	idx.Add("doc1", []float32{1.0, 0.0, 0.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "main.py"},
	})
	err = idx.Save(savePath)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Test loading existing index
	idx2, err := LoadOrNew(savePath, 3)
	if err != nil {
		t.Fatalf("LoadOrNew() unexpected error: %v", err)
	}
	if idx2.Count() != 1 {
		t.Errorf("Count() after load = %d, want 1", idx2.Count())
	}

	// Verify data
	vector, _, found := idx2.Get("doc1")
	if !found {
		t.Error("LoadOrNew() did not load existing data")
	}
	if vector[0] != 1.0 {
		t.Errorf("doc1 vector[0] = %v, want 1.0", vector[0])
	}
}

func TestVectorIndexCountDimension(t *testing.T) {
	idx := NewVectorIndex(128)

	if idx.Dimension() != 128 {
		t.Errorf("Dimension() = %d, want 128", idx.Dimension())
	}
	if idx.Count() != 0 {
		t.Errorf("Count() = %d, want 0", idx.Count())
	}

	// Add some vectors
	for i := 0; i < 10; i++ {
		vector := make([]float32, 128)
		vector[i] = 1.0
		idx.Add(string(rune('a'+i)), vector, types.EmbeddingUnit{})
	}

	if idx.Count() != 10 {
		t.Errorf("Count() = %d, want 10", idx.Count())
	}
	if idx.Dimension() != 128 {
		t.Errorf("Dimension() = %d, want 128", idx.Dimension())
	}
}
