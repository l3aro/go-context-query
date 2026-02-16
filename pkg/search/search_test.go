package search

import (
	"errors"
	"hash/fnv"
	"testing"

	"github.com/l3aro/go-context-query/pkg/embed"
	"github.com/l3aro/go-context-query/pkg/index"
	"github.com/l3aro/go-context-query/pkg/types"
)

// mockProvider implements embed.Provider for testing
type mockProvider struct {
	dimension int
}

func (m *mockProvider) Embed(texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		results[i] = generateMockEmbedding(text, m.dimension)
	}
	return results, nil
}

func (m *mockProvider) Config() *embed.Config {
	return &embed.Config{
		Model:      "mock",
		Dimensions: m.dimension,
	}
}

// generateMockEmbedding creates a deterministic embedding based on text hash
func generateMockEmbedding(text string, dimension int) []float32 {
	h := fnv.New32a()
	h.Write([]byte(text))
	seed := h.Sum32()

	// Generate deterministic pseudo-random values based on seed
	rng := func(i uint32) float32 {
		x := seed + i*1103515245 + 12345
		x = (x >> 15) ^ x
		return float32(x%1000) / 1000.0
	}

	embedding := make([]float32, dimension)
	for i := 0; i < dimension; i++ {
		embedding[i] = rng(uint32(i))
	}
	return embedding
}

// createTestIndex creates a VectorIndex with test data
func createTestIndex(dimension int) *index.VectorIndex {
	idx := index.NewVectorIndex(dimension)

	// Add test vectors with different similarity profiles
	testData := []struct {
		id       string
		vector   []float32
		metadata types.EmbeddingUnit
	}{
		{
			id:     "main.go:handleRequest",
			vector: generateMockEmbedding("handle request function", dimension),
			metadata: types.EmbeddingUnit{
				L1Data: types.ModuleInfo{
					Path:       "main.go",
					LineNumber: 10,
					Signature:  "handleRequest(req *http.Request)",
					Docstring:  "Handles incoming HTTP requests",
					Type:       "function",
				},
			},
		},
		{
			id:     "utils/helper.go:formatResponse",
			vector: generateMockEmbedding("format response helper", dimension),
			metadata: types.EmbeddingUnit{
				L1Data: types.ModuleInfo{
					Path:       "utils/helper.go",
					LineNumber: 25,
					Signature:  "formatResponse(data interface{}) string",
					Docstring:  "Formats response as JSON",
					Type:       "function",
				},
			},
		},
		{
			id:     "services/auth.go:validateToken",
			vector: generateMockEmbedding("validate token auth", dimension),
			metadata: types.EmbeddingUnit{
				L1Data: types.ModuleInfo{
					Path:       "services/auth.go",
					LineNumber: 42,
					Signature:  "validateToken(token string) (bool, error)",
					Docstring:  "Validates authentication token",
					Type:       "function",
				},
			},
		},
		{
			id:     "models/user.go:User",
			vector: generateMockEmbedding("user model class", dimension),
			metadata: types.EmbeddingUnit{
				L1Data: types.ModuleInfo{
					Path:       "models/user.go",
					LineNumber: 1,
					Signature:  "type User struct",
					Docstring:  "User model",
					Type:       "class",
				},
			},
		},
		{
			id:     "db/connection.go:connect",
			vector: generateMockEmbedding("database connect", dimension),
			metadata: types.EmbeddingUnit{
				L1Data: types.ModuleInfo{
					Path:       "db/connection.go",
					LineNumber: 15,
					Signature:  "connect() (*sql.DB, error)",
					Docstring:  "Connects to database",
					Type:       "function",
				},
			},
		},
	}

	for _, td := range testData {
		_ = idx.Add(td.id, td.vector, td.metadata)
	}

	return idx
}

func TestNewSearcher(t *testing.T) {
	dimension := 3
	provider := &mockProvider{dimension: dimension}
	idx := index.NewVectorIndex(dimension)

	searcher := NewSearcher(provider, idx)

	if searcher == nil {
		t.Fatal("NewSearcher returned nil")
	}
	if searcher.embedProvider != provider {
		t.Error("embedProvider not set correctly")
	}
	if searcher.vectorIndex != idx {
		t.Error("vectorIndex not set correctly")
	}
}

func TestEmbedQuery(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		expectError bool
	}{
		{
			name:        "valid query",
			query:       "handle request",
			expectError: false,
		},
		{
			name:        "empty query",
			query:       "",
			expectError: true,
		},
		{
			name:        "whitespace only query",
			query:       "   ",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dimension := 3
			provider := &mockProvider{dimension: dimension}
			idx := index.NewVectorIndex(dimension)
			searcher := NewSearcher(provider, idx)

			embedding, err := searcher.EmbedQuery(tt.query)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(embedding) != dimension {
				t.Errorf("expected embedding dimension %d, got %d", dimension, len(embedding))
			}
		})
	}
}

func TestSearch(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		k           int
		expectError bool
		wantCount   int
	}{
		{
			name:        "valid search",
			query:       "handle request",
			k:           2,
			expectError: false,
			wantCount:   2,
		},
		{
			name:        "k larger than index",
			query:       "validate token",
			k:           100,
			expectError: false,
			wantCount:   5, // index has 5 vectors
		},
		{
			name:        "empty query",
			query:       "",
			k:           2,
			expectError: true,
		},
		{
			name:        "k is zero",
			query:       "test",
			k:           0,
			expectError: true,
		},
		{
			name:        "k is negative",
			query:       "test",
			k:           -1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dimension := 3
			provider := &mockProvider{dimension: dimension}
			idx := createTestIndex(dimension)
			searcher := NewSearcher(provider, idx)

			results, err := searcher.Search(tt.query, tt.k)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(results))
			}
			// Verify results are sorted by score (descending)
			for i := 1; i < len(results); i++ {
				if results[i].Score > results[i-1].Score {
					t.Error("results not sorted by score descending")
				}
			}
		})
	}
}

func TestSearchWithEmbedding(t *testing.T) {
	tests := []struct {
		name        string
		embedding   []float32
		k           int
		expectError bool
		wantCount   int
	}{
		{
			name:        "valid search with embedding",
			embedding:   []float32{0.5, 0.3, 0.2},
			k:           2,
			expectError: false,
			wantCount:   2,
		},
		{
			name:        "dimension mismatch",
			embedding:   []float32{0.5, 0.3}, // wrong dimension
			k:           2,
			expectError: true,
		},
		{
			name:        "k is zero",
			embedding:   []float32{0.5, 0.3, 0.2},
			k:           0,
			expectError: true,
		},
		{
			name:        "k is negative",
			embedding:   []float32{0.5, 0.3, 0.2},
			k:           -1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dimension := 3
			provider := &mockProvider{dimension: dimension}
			idx := createTestIndex(dimension)
			searcher := NewSearcher(provider, idx)

			results, err := searcher.SearchWithEmbedding(tt.embedding, tt.k)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(results))
			}
		})
	}
}

func TestSearchWithThreshold(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		k            int
		threshold    float32
		wantMinCount int
		expectError  bool
	}{
		{
			name:         "threshold filters results",
			query:        "handle request",
			k:            5,
			threshold:    0.99,
			wantMinCount: 0,
			expectError:  false,
		},
		{
			name:         "low threshold returns all",
			query:        "handle request",
			k:            5,
			threshold:    0.0,
			wantMinCount: 5,
			expectError:  false,
		},
		{
			name:         "very high threshold filters most",
			query:        "validate token",
			k:            5,
			threshold:    0.999,
			wantMinCount: 1,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dimension := 3
			provider := &mockProvider{dimension: dimension}
			idx := createTestIndex(dimension)
			searcher := NewSearcher(provider, idx)

			results, err := searcher.SearchWithThreshold(tt.query, tt.k, tt.threshold)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) < tt.wantMinCount {
				t.Errorf("expected at least %d results, got %d", tt.wantMinCount, len(results))
			}
			// Verify all results meet threshold
			for _, r := range results {
				if r.Score < tt.threshold {
					t.Errorf("result %s has score %f below threshold %f", r.Name, r.Score, tt.threshold)
				}
			}
		})
	}
}

func TestIndexStats(t *testing.T) {
	dimension := 3
	provider := &mockProvider{dimension: dimension}
	idx := createTestIndex(dimension)
	searcher := NewSearcher(provider, idx)

	count, dim := searcher.IndexStats()

	if count != 5 {
		t.Errorf("expected count 5, got %d", count)
	}
	if dim != dimension {
		t.Errorf("expected dimension %d, got %d", dimension, dim)
	}
}

func TestEmbedTexts(t *testing.T) {
	tests := []struct {
		name        string
		texts       []string
		expectError bool
		wantCount   int
	}{
		{
			name:        "valid texts",
			texts:       []string{"text1", "text2", "text3"},
			expectError: false,
			wantCount:   3,
		},
		{
			name:        "empty slice",
			texts:       []string{},
			expectError: false,
			wantCount:   0,
		},
		{
			name:        "one text",
			texts:       []string{"single text"},
			expectError: false,
			wantCount:   1,
		},
		{
			name:        "text with empty string",
			texts:       []string{"valid", ""},
			expectError: true,
		},
		{
			name:        "text with whitespace only",
			texts:       []string{"valid", "   "},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dimension := 3
			provider := &mockProvider{dimension: dimension}
			idx := index.NewVectorIndex(dimension)
			searcher := NewSearcher(provider, idx)

			embeddings, err := searcher.EmbedTexts(tt.texts)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(embeddings) != tt.wantCount {
				t.Errorf("expected %d embeddings, got %d", tt.wantCount, len(embeddings))
			}
			// Verify dimensions
			for i, emb := range embeddings {
				if len(emb) != dimension {
					t.Errorf("embedding %d has dimension %d, expected %d", i, len(emb), dimension)
				}
			}
		})
	}
}

func TestSearchResultConversion(t *testing.T) {
	dimension := 3
	provider := &mockProvider{dimension: dimension}
	idx := createTestIndex(dimension)
	searcher := NewSearcher(provider, idx)

	results, err := searcher.Search("handle request", 1)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	result := results[0]

	// Check that result has required fields
	if result.FilePath == "" {
		t.Error("FilePath should not be empty")
	}
	if result.Name == "" {
		t.Error("Name should not be empty")
	}
	// Score can be any positive value from the similarity function
	if result.Score < 0 {
		t.Errorf("Score should be non-negative, got %f", result.Score)
	}
}

// mockProviderWithError is a mock provider that returns errors
type mockProviderWithError struct {
	mockProvider
	err error
}

func (m *mockProviderWithError) Embed(texts []string) ([][]float32, error) {
	return nil, m.err
}

func TestSearchWithProviderError(t *testing.T) {
	testErr := errors.New("provider error")
	dimension := 3
	provider := &mockProviderWithError{
		mockProvider: mockProvider{dimension: dimension},
		err:          testErr,
	}
	idx := createTestIndex(dimension)
	searcher := NewSearcher(provider, idx)

	_, err := searcher.Search("test query", 2)

	if err == nil {
		t.Error("expected error from provider, got nil")
	}
}

func TestGemmaQueryPrefix(t *testing.T) {
	// Verify the constant is defined correctly
	expectedPrefix := "Given a codebase, find code that: "
	if GemmaQueryPrefix != expectedPrefix {
		t.Errorf("GemmaQueryPrefix = %q, want %q", GemmaQueryPrefix, expectedPrefix)
	}

	// Verify it's used in EmbedQuery
	dimension := 3
	provider := &mockProvider{dimension: dimension}
	idx := createTestIndex(dimension)
	searcher := NewSearcher(provider, idx)

	// EmbedQuery should add the prefix to the query
	// We can't directly test the prefix was added, but we can verify it works
	_, err := searcher.EmbedQuery("test query")
	if err != nil {
		t.Fatalf("EmbedQuery failed: %v", err)
	}
}
