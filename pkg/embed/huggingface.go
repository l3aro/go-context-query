package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strings"
	"sync"
)

// DefaultGemmaModel is the default embedding model
const DefaultGemmaModel = "google/gemma-2-2b-it"

// HuggingFaceEndpoint is the base URL for HuggingFace Inference API
const HuggingFaceEndpoint = "https://router.huggingface.co/hf-inference/models"

// DefaultBatchSize is the default batch size for requests
const DefaultBatchSize = 32

// hfRequest represents the request payload for HuggingFace Inference API
type hfRequest struct {
	Inputs interface{} `json:"inputs"`
}

// hfResponse represents the response from HuggingFace Inference API
type hfResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// HuggingFaceProvider implements the Provider interface for HuggingFace Inference API
type HuggingFaceProvider struct {
	config     *Config
	httpClient *http.Client
	mu         sync.RWMutex
}

// NewHuggingFaceProvider creates a new HuggingFace embedding provider
func NewHuggingFaceProvider(cfg *Config) (*HuggingFaceProvider, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	// Set defaults
	if cfg.Endpoint == "" {
		cfg.Endpoint = HuggingFaceEndpoint
	}
	if cfg.Model == "" {
		cfg.Model = DefaultGemmaModel
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultBatchSize
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if cfg.APIKey == "" {
		return nil, ErrAPIKeyMissing
	}

	return &HuggingFaceProvider{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 120, // 2 minutes timeout for large batches
		},
	}, nil
}

// Config returns the provider configuration
func (p *HuggingFaceProvider) Config() *Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// Embed generates embeddings for the given texts using HuggingFace Inference API
func (p *HuggingFaceProvider) Embed(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Validate inputs
	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("%w: text at index %d is empty", ErrInvalidInput, i)
		}
	}

	// Use batch processing for efficiency
	return p.EmbedBatch(texts, p.config.BatchSize)
}

// EmbedBatch generates embeddings for texts in batches
func (p *HuggingFaceProvider) EmbedBatch(texts []string, batchSize int) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}

	// Process in batches
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := p.embedBatchRequest(batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d-%d: %w", i, end, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	// Normalize embeddings with L2 norm
	return normalizeEmbeddings(allEmbeddings), nil
}

// embedBatchRequest sends a single batch request to HuggingFace Inference API
func (p *HuggingFaceProvider) embedBatchRequest(texts []string) ([][]float32, error) {
	// Build request
	reqBody, err := json.Marshal(hfRequest{
		Inputs: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/%s", p.config.Endpoint, p.config.Model)

	req, err := http.NewRequestWithContext(context.Background(), "POST", endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.config.APIKey))
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d: %s", ErrProviderUnavailable, resp.StatusCode, string(body))
	}

	// Parse response
	var result hfResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Validate response
	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("%w: expected %d embeddings, got %d", ErrProviderUnavailable, len(texts), len(result.Embeddings))
	}

	return result.Embeddings, nil
}

// normalizeEmbeddings normalizes each embedding vector using L2 norm
func normalizeEmbeddings(embeddings [][]float32) [][]float32 {
	result := make([][]float32, len(embeddings))

	for i, emb := range embeddings {
		result[i] = l2Normalize(emb)
	}

	return result
}

// l2Normalize normalizes a vector using L2 norm
func l2Normalize(vector []float32) []float32 {
	// Calculate L2 norm
	var sumSq float64
	for _, v := range vector {
		sumSq += float64(v) * float64(v)
	}
	norm := math.Sqrt(sumSq)

	// Avoid division by zero
	if norm == 0 {
		return vector
	}

	// Normalize
	normalized := make([]float32, len(vector))
	for i, v := range vector {
		normalized[i] = float32(float64(v) / norm)
	}

	return normalized
}

// Ensure HuggingFaceProvider implements Provider
var _ Provider = (*HuggingFaceProvider)(nil)

// Ensure HuggingFaceProvider implements BatchProvider
var _ BatchProvider = (*HuggingFaceProvider)(nil)

// Unit vectors check - helper to verify normalization
func isUnitVector(vector []float32, tolerance float64) bool {
	var sumSq float64
	for _, v := range vector {
		sumSq += float64(v) * float64(v)
	}
	return math.Abs(sumSq-1.0) < tolerance
}

// Similarity returns the cosine similarity between two vectors
func Similarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
	}

	return dotProduct
}

// EmbedWithMetadata returns embeddings with additional metadata
func (p *HuggingFaceProvider) EmbedWithMetadata(texts []string) (*EmbedResult, error) {
	embeddings, err := p.Embed(texts)
	if err != nil {
		return nil, err
	}

	// Estimate tokens (rough approximation: ~4 chars per token)
	var totalTokens int
	for _, text := range texts {
		totalTokens += len(text) / 4
	}

	return &EmbedResult{
		Embeddings: embeddings,
		Model:      p.config.Model,
		Tokens:     totalTokens,
	}, nil
}

// BatchWithMetadata processes texts in batches and returns embeddings with metadata
func (p *HuggingFaceProvider) BatchWithMetadata(texts []string, batchSize int) (*EmbedResult, error) {
	embeddings, err := p.EmbedBatch(texts, batchSize)
	if err != nil {
		return nil, err
	}

	// Estimate tokens (rough approximation: ~4 chars per token)
	var totalTokens int
	for _, text := range texts {
		totalTokens += len(text) / 4
	}

	return &EmbedResult{
		Embeddings: embeddings,
		Model:      p.config.Model,
		Tokens:     totalTokens,
	}, nil
}

// TruncateText truncates text to fit within model's context window
// This is a simple implementation - models may have different limits
func (p *HuggingFaceProvider) TruncateText(text string, maxTokens int) string {
	// Rough estimate: 1 token ≈ 4 characters
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars]
}

// EmbedSingle generates embedding for a single text
func (p *HuggingFaceProvider) EmbedSingle(text string) ([]float32, error) {
	embeddings, err := p.Embed([]string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, errors.New("no embedding returned")
	}

	return embeddings[0], nil
}

// Dimension returns the embedding dimension (will be known after first call)
func (p *HuggingFaceProvider) Dimension() (int, error) {
	// Try to get dimension from a test embedding
	testEmbed, err := p.EmbedSingle("test")
	if err != nil {
		return 0, err
	}

	return len(testEmbed), nil
}

// SortBySimilarity returns indices sorted by similarity to query in descending order
func SortBySimilarity(queryEmbedding []float32, candidates [][]float32) []int {
	similarities := make([]struct {
		idx        int
		similarity float64
	}, len(candidates))

	for i, cand := range candidates {
		similarities[i] = struct {
			idx        int
			similarity float64
		}{
			idx:        i,
			similarity: float64(Similarity(queryEmbedding, cand)),
		}
	}

	// Sort by similarity descending
	slices.SortFunc(similarities, func(a, b struct {
		idx        int
		similarity float64
	}) int {
		if a.similarity > b.similarity {
			return -1
		}
		if a.similarity < b.similarity {
			return 1
		}
		return 0
	})

	// Extract indices
	indices := make([]int, len(candidates))
	for i, s := range similarities {
		indices[i] = s.idx
	}

	return indices
}
