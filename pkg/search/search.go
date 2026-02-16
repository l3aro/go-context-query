// Package search provides semantic search functionality for code context.
// It embeds queries and searches a vector index to return ranked results.
package search

import (
	"fmt"
	"strings"

	"github.com/user/go-context-query/pkg/embed"
	"github.com/user/go-context-query/pkg/index"
)

// GemmaQueryPrefix is the instruction prefix for Gemma models
const GemmaQueryPrefix = "Given a codebase, find code that: "

// SearchResult represents a single search result with metadata
type SearchResult struct {
	// FilePath is the path to the file containing this code unit
	FilePath string `json:"file_path"`
	// LineNumber is the line where this code unit is defined
	LineNumber int `json:"line_number"`
	// Name is the name of the function/method/class
	Name string `json:"name"`
	// Signature is the function signature
	Signature string `json:"signature"`
	// Docstring is the docstring/comment
	Docstring string `json:"docstring"`
	// Type is the type of unit (function, method, class)
	Type string `json:"type"`
	// Score is the similarity score (0-1, higher is better)
	Score float32 `json:"score"`
}

// Searcher provides semantic search over indexed code
type Searcher struct {
	embedProvider embed.Provider
	vectorIndex   *index.VectorIndex
}

// NewSearcher creates a new Searcher with the given embedding provider and vector index
func NewSearcher(embedProvider embed.Provider, vectorIndex *index.VectorIndex) *Searcher {
	return &Searcher{
		embedProvider: embedProvider,
		vectorIndex:   vectorIndex,
	}
}

// EmbedQuery embeds a search query with an instruction prefix for Gemma models
func (s *Searcher) EmbedQuery(query string) ([]float32, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	prefixedQuery := GemmaQueryPrefix + query

	embeddings, err := s.embedProvider.Embed([]string{prefixedQuery})
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return embeddings[0], nil
}

// Search performs semantic search and returns top-k results
func (s *Searcher) Search(query string, k int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	if k <= 0 {
		return nil, fmt.Errorf("k must be positive, got %d", k)
	}

	queryEmbedding, err := s.EmbedQuery(query)
	if err != nil {
		return nil, err
	}

	indexResults, err := s.vectorIndex.Search(queryEmbedding, k)
	if err != nil {
		return nil, fmt.Errorf("searching index: %w", err)
	}

	results := make([]SearchResult, len(indexResults))
	for i, res := range indexResults {
		results[i] = s.convertResult(res)
	}

	return results, nil
}

// SearchWithEmbedding performs search using a pre-computed query embedding
// This is useful when the same query embedding is used multiple times
func (s *Searcher) SearchWithEmbedding(queryEmbedding []float32, k int) ([]SearchResult, error) {
	if k <= 0 {
		return nil, fmt.Errorf("k must be positive, got %d", k)
	}

	if len(queryEmbedding) != s.vectorIndex.Dimension() {
		return nil, fmt.Errorf("embedding dimension mismatch: expected %d, got %d",
			s.vectorIndex.Dimension(), len(queryEmbedding))
	}

	indexResults, err := s.vectorIndex.Search(queryEmbedding, k)
	if err != nil {
		return nil, fmt.Errorf("searching index: %w", err)
	}

	results := make([]SearchResult, len(indexResults))
	for i, res := range indexResults {
		results[i] = s.convertResult(res)
	}

	return results, nil
}

// convertResult converts an index.SearchResult to a SearchResult
func (s *Searcher) convertResult(res index.SearchResult) SearchResult {
	parts := strings.SplitN(res.ID, ":", 2)
	filePath := ""
	name := ""
	if len(parts) == 2 {
		filePath = parts[0]
		name = parts[1]
	} else {
		name = res.ID
	}

	lineNumber := 0
	signature := ""
	docstring := ""
	codeType := "function"

	if res.Metadata.L1Data.Path != "" {
		filePath = res.Metadata.L1Data.Path
	}

	if res.Metadata.L1Data.LineNumber > 0 {
		lineNumber = res.Metadata.L1Data.LineNumber
	}
	if res.Metadata.L1Data.Signature != "" {
		signature = res.Metadata.L1Data.Signature
	}
	if res.Metadata.L1Data.Docstring != "" {
		docstring = res.Metadata.L1Data.Docstring
	}
	if res.Metadata.L1Data.Type != "" {
		codeType = res.Metadata.L1Data.Type
	}

	if len(res.Metadata.L1Data.Functions) > 0 {
		fn := res.Metadata.L1Data.Functions[0]
		if lineNumber == 0 && fn.LineNumber > 0 {
			lineNumber = fn.LineNumber
		}
		if signature == "" && fn.Params != "" {
			signature = fmt.Sprintf("def %s(%s)", fn.Name, fn.Params)
			if fn.ReturnType != "" {
				signature += " -> " + fn.ReturnType
			}
		}
		if docstring == "" && fn.Docstring != "" {
			docstring = fn.Docstring
		}
	}

	return SearchResult{
		FilePath:   filePath,
		LineNumber: lineNumber,
		Name:       name,
		Signature:  signature,
		Docstring:  docstring,
		Type:       codeType,
		Score:      res.Score,
	}
}

// SearchWithThreshold performs semantic search with a minimum similarity threshold
func (s *Searcher) SearchWithThreshold(query string, k int, threshold float32) ([]SearchResult, error) {
	results, err := s.Search(query, k)
	if err != nil {
		return nil, err
	}

	filtered := make([]SearchResult, 0)
	for _, r := range results {
		if r.Score >= threshold {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

// IndexStats returns statistics about the vector index
func (s *Searcher) IndexStats() (int, int) {
	return s.vectorIndex.Count(), s.vectorIndex.Dimension()
}

// EmbedTexts embeds multiple texts (for batch processing)
func (s *Searcher) EmbedTexts(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("text at index %d is empty", i)
		}
	}

	return s.embedProvider.Embed(texts)
}
