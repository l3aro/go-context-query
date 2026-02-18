package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// DefaultOllamaModel is the default embedding model for Ollama
const DefaultOllamaModel = "nomic-embed-text"

// DefaultOllamaEndpoint is the default base URL for Ollama API
const DefaultOllamaEndpoint = "http://localhost:11434"

// DefaultOllamaBatchSize is the default batch size for Ollama requests
const DefaultOllamaBatchSize = 1 // Ollama /api/embeddings only supports single prompt

// ollamaRequest represents the request payload for Ollama embeddings API
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaResponse represents the response from Ollama embeddings API
type ollamaResponse struct {
	Embedding []float32 `json:"embedding"`
}

// OllamaProvider implements the Provider interface for Ollama API
type OllamaProvider struct {
	config     *Config
	httpClient *http.Client
	mu         sync.RWMutex
}

// NewOllamaProvider creates a new Ollama embedding provider
func NewOllamaProvider(cfg *Config) (*OllamaProvider, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	// Set defaults
	if cfg.Endpoint == "" {
		cfg.Endpoint = DefaultOllamaEndpoint
	}
	if cfg.Model == "" {
		cfg.Model = DefaultOllamaModel
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultOllamaBatchSize
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Note: Ollama doesn't require API key for local instances
	// but supports bearer token authentication for remote instances

	return &OllamaProvider{
		config:     cfg,
		httpClient: http.DefaultClient,
	}, nil
}

// Config returns the provider configuration
func (p *OllamaProvider) Config() *Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// Embed generates embeddings for the given texts using Ollama API
func (p *OllamaProvider) Embed(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Validate inputs
	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("%w: text at index %d is empty", ErrInvalidInput, i)
		}
	}

	// Use batch processing
	return p.EmbedBatch(texts, p.config.BatchSize)
}

// EmbedBatch generates embeddings for texts in batches
func (p *OllamaProvider) EmbedBatch(texts []string, batchSize int) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	if batchSize <= 0 {
		batchSize = DefaultOllamaBatchSize
	}

	// Ollama /api/embeddings only supports single prompt per request
	// so we process one at a time regardless of batchSize
	if batchSize > 1 {
		batchSize = 1
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

	return allEmbeddings, nil
}

// embedBatchRequest sends a single batch request to Ollama API
func (p *OllamaProvider) embedBatchRequest(texts []string) ([][]float32, error) {
	// Ollama API only supports single prompt, so we only take the first one
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	text := texts[0]

	// Build request
	reqBody, err := json.Marshal(ollamaRequest{
		Model:  p.config.Model,
		Prompt: text,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := p.config.Endpoint + "/api/embeddings"

	req, err := http.NewRequestWithContext(context.Background(), "POST", endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Add bearer token authentication if API key is provided
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.config.APIKey))
	}

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
	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Validate response
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("%w: empty embedding returned", ErrProviderUnavailable)
	}

	return [][]float32{result.Embedding}, nil
}

// Ensure OllamaProvider implements Provider
var _ Provider = (*OllamaProvider)(nil)

// Ensure OllamaProvider implements BatchProvider
var _ BatchProvider = (*OllamaProvider)(nil)

// EmbedSingle generates embedding for a single text
func (p *OllamaProvider) EmbedSingle(text string) ([]float32, error) {
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
func (p *OllamaProvider) Dimension() (int, error) {
	// Try to get dimension from a test embedding
	testEmbed, err := p.EmbedSingle("test")
	if err != nil {
		return 0, err
	}

	return len(testEmbed), nil
}

// EmbedWithMetadata returns embeddings with additional metadata
func (p *OllamaProvider) EmbedWithMetadata(texts []string) (*EmbedResult, error) {
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

// TruncateText truncates text to fit within model's context window
// This is a simple implementation - models may have different limits
func (p *OllamaProvider) TruncateText(text string, maxTokens int) string {
	// Rough estimate: 1 token ≈ 4 characters
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars]
}
