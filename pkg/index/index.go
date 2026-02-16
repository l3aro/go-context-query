// Package index provides a simple in-memory vector index with cosine similarity search.
// It is used for semantic code search over embedded code units.
package index

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/l3aro/go-context-query/pkg/types"
	"github.com/vmihailenco/msgpack/v5"
)

// VectorIndex is an in-memory vector store with brute-force search
type VectorIndex struct {
	vectors   []float32 // Flattened vectors: [v1[0], v1[1], ..., v1[dim-1], v2[0], ...]
	metadata  []types.EmbeddingUnit
	ids       []string
	dimension int
}

// SearchResult represents a single search result
type SearchResult struct {
	ID       string
	Metadata types.EmbeddingUnit
	Score    float32
}

// NewVectorIndex creates a new VectorIndex with the specified dimension
func NewVectorIndex(dimension int) *VectorIndex {
	return &VectorIndex{
		dimension: dimension,
		vectors:   make([]float32, 0, dimension*100), // Pre-allocate for 100 vectors
		metadata:  make([]types.EmbeddingUnit, 0, 100),
		ids:       make([]string, 0, 100),
	}
}

// Dimension returns the vector dimension
func (v *VectorIndex) Dimension() int {
	return v.dimension
}

// Count returns the number of vectors in the index
func (v *VectorIndex) Count() int {
	return len(v.ids)
}

// Add adds a vector with metadata to the index
func (v *VectorIndex) Add(id string, vector []float32, metadata types.EmbeddingUnit) error {
	if len(vector) != v.dimension {
		return fmt.Errorf("vector dimension mismatch: expected %d, got %d", v.dimension, len(vector))
	}

	v.ids = append(v.ids, id)
	v.vectors = append(v.vectors, vector...)
	v.metadata = append(v.metadata, metadata)

	return nil
}

// normalize computes the L2 norm of a vector
func normalize(vector []float32) float32 {
	var sum float32
	for _, v := range vector {
		sum += v * v
	}
	if sum == 0 {
		return 0
	}
	return float32(1.0 / float64(sum))
}

// cosineSimilarity computes cosine similarity between two vectors
// Both vectors should be normalized for true cosine similarity
func cosineSimilarity(a, b []float32) float32 {
	var dotProduct float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
	}
	return dotProduct
}

// cosineSimilarityWithNorm computes cosine similarity without pre-normalization
func cosineSimilarityWithNorm(a, b []float32) float32 {
	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	norm := float32(float64(normA) * float64(normB))
	if norm == 0 {
		return 0
	}
	return dotProduct / float32(float64(normA)*float64(normB))
}

// Search finds the top-k most similar vectors using cosine similarity
func (v *VectorIndex) Search(query []float32, k int) ([]SearchResult, error) {
	if len(query) != v.dimension {
		return nil, fmt.Errorf("query dimension mismatch: expected %d, got %d", v.dimension, len(query))
	}

	if k <= 0 {
		return nil, fmt.Errorf("k must be positive, got %d", k)
	}

	if k > v.Count() {
		k = v.Count()
	}

	// Compute similarity for all vectors
	type scoredIndex struct {
		index int
		score float32
	}

	scores := make([]scoredIndex, v.Count())
	for i := 0; i < v.Count(); i++ {
		start := i * v.dimension
		end := start + v.dimension
		vector := v.vectors[start:end]

		scores[i] = scoredIndex{
			index: i,
			score: cosineSimilarityWithNorm(query, vector),
		}
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Take top-k
	results := make([]SearchResult, k)
	for i := 0; i < k; i++ {
		idx := scores[i].index
		results[i] = SearchResult{
			ID:       v.ids[idx],
			Metadata: v.metadata[idx],
			Score:    scores[i].score,
		}
	}

	return results, nil
}

// indexData is the serialized structure for persistence
type indexData struct {
	Dimension int                   `msgpack:"d"`
	IDs       []string              `msgpack:"ids"`
	Vectors   []float32             `msgpack:"vecs"`
	Metadata  []types.EmbeddingUnit `msgpack:"meta"`
}

// Save persists the index to a file using msgpack
func (v *VectorIndex) Save(path string) error {
	data := indexData{
		Dimension: v.dimension,
		IDs:       v.ids,
		Vectors:   v.vectors,
		Metadata:  v.metadata,
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := msgpack.NewEncoder(file)

	if err := encoder.Encode(&data); err != nil {
		return fmt.Errorf("failed to encode index: %w", err)
	}

	return nil
}

// Load restores the index from a file using msgpack
func (v *VectorIndex) Load(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	decoder := msgpack.NewDecoder(file)

	var data indexData
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode index: %w", err)
	}

	v.dimension = data.Dimension
	v.ids = data.IDs
	v.vectors = data.Vectors
	v.metadata = data.Metadata

	return nil
}

// LoadOrNew loads an index from a file or creates a new one with the given dimension
func LoadOrNew(path string, dimension int) (*VectorIndex, error) {
	index := NewVectorIndex(dimension)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index, nil
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	file.Close()

	if err := index.Load(path); err != nil {
		return nil, err
	}

	return index, nil
}

// Get returns the vector and metadata for a given ID
func (v *VectorIndex) Get(id string) ([]float32, types.EmbeddingUnit, bool) {
	for i, existingID := range v.ids {
		if existingID == id {
			start := i * v.dimension
			end := start + v.dimension
			vector := make([]float32, v.dimension)
			copy(vector, v.vectors[start:end])
			return vector, v.metadata[i], true
		}
	}
	return nil, types.EmbeddingUnit{}, false
}

// Delete removes a vector by ID
func (v *VectorIndex) Delete(id string) bool {
	for i, existingID := range v.ids {
		if existingID == id {
			// Remove from all slices
			v.ids = append(v.ids[:i], v.ids[i+1:]...)
			v.metadata = append(v.metadata[:i], v.metadata[i+1:]...)

			// Remove vector (shift all vectors after this one)
			start := i * v.dimension
			end := start + v.dimension
			v.vectors = append(v.vectors[:start], v.vectors[end:]...)

			return true
		}
	}
	return false
}

// Clear removes all vectors from the index
func (v *VectorIndex) Clear() {
	v.ids = v.ids[:0]
	v.metadata = v.metadata[:0]
	v.vectors = v.vectors[:0]
}

// IterVectors iterates over all vectors with their IDs and metadata
func (v *VectorIndex) IterVectors(fn func(id string, vector []float32, metadata types.EmbeddingUnit) bool) {
	for i := 0; i < v.Count(); i++ {
		start := i * v.dimension
		end := start + v.dimension
		vector := v.vectors[start:end]

		if !fn(v.ids[i], vector, v.metadata[i]) {
			break
		}
	}
}

// WriteTo writes the index to an io.Writer in msgpack format
func (v *VectorIndex) WriteTo(w io.Writer) (int64, error) {
	data := indexData{
		Dimension: v.dimension,
		IDs:       v.ids,
		Vectors:   v.vectors,
		Metadata:  v.metadata,
	}

	encoder := msgpack.NewEncoder(w)

	if err := encoder.Encode(&data); err != nil {
		return 0, fmt.Errorf("failed to encode index: %w", err)
	}

	return 0, nil
}

// ReadFrom reads the index from an io.Reader in msgpack format
func (v *VectorIndex) ReadFrom(r io.Reader) (int64, error) {
	decoder := msgpack.NewDecoder(r)

	var data indexData
	if err := decoder.Decode(&data); err != nil {
		return 0, fmt.Errorf("failed to decode index: %w", err)
	}

	v.dimension = data.Dimension
	v.ids = data.IDs
	v.vectors = data.Vectors
	v.metadata = data.Metadata

	return int64(len(v.ids)), nil
}

// ExampleVectorIndex demonstrates basic VectorIndex usage
func ExampleVectorIndex() {
	// Create a new index with 3-dimensional vectors
	idx := NewVectorIndex(3)

	// Add vectors with metadata
	idx.Add("doc1", []float32{1.0, 0.0, 0.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "main.py"},
	})
	idx.Add("doc2", []float32{0.0, 1.0, 0.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "utils.py"},
	})
	idx.Add("doc3", []float32{0.0, 0.0, 1.0}, types.EmbeddingUnit{
		L1Data: types.ModuleInfo{Path: "helpers.py"},
	})

	// Search for similar vectors
	results, _ := idx.Search([]float32{0.9, 0.1, 0.0}, 2)

	fmt.Printf("Found %d results\n", len(results))
	fmt.Printf("Top result: %s (score: %.2f)\n", results[0].ID, results[0].Score)

	// Output:
	// Found 2 results
	// Top result: doc1 (score: 0.99)
}
