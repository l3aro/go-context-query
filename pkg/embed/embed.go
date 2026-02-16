package embed

import (
	"errors"
	"fmt"
)

// ErrInvalidInput is returned when the input is invalid
var ErrInvalidInput = errors.New("invalid input")

// ErrProviderUnavailable is returned when the embedding provider is unavailable
var ErrProviderUnavailable = errors.New("provider unavailable")

// ErrAPIKeyMissing is returned when the API key is missing
var ErrAPIKeyMissing = errors.New("API key missing")

// ErrInvalidModel is returned when the model is invalid
var ErrInvalidModel = errors.New("invalid model")

// EmbeddingError wraps provider-specific errors
type EmbeddingError struct {
	Provider string
	Message  string
	Err      error
}

func (e *EmbeddingError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("embedding error (%s): %s: %v", e.Provider, e.Message, e.Err)
	}
	return fmt.Sprintf("embedding error (%s): %s", e.Provider, e.Message)
}

func (e *EmbeddingError) Unwrap() error {
	return e.Err
}

// Config holds configuration for embedding providers
type Config struct {
	// Endpoint is the base URL for the embedding API
	Endpoint string

	// APIKey is the authentication token
	APIKey string

	// Model is the embedding model to use
	Model string

	// BatchSize is the maximum number of texts to embed in a single request
	// 0 means use provider default
	BatchSize int

	// Dimensions is the expected embedding dimension (for models that support
	// dimensionality reduction)
	// 0 means use model default
	Dimensions int
}

// Validate checks that the configuration has valid required fields
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if c.Model == "" {
		return errors.New("model is required")
	}
	return nil
}

// Provider defines the interface for embedding providers
type Provider interface {
	// Embed generates embeddings for the given texts.
	// The returned slice has the same length as the input texts.
	// Each embedding is a slice of float32 values.
	Embed(texts []string) ([][]float32, error)

	// Config returns the provider configuration
	Config() *Config
}

// BatchProvider is an optional interface for providers that support
// efficient batch processing with custom batch sizes
type BatchProvider interface {
	// EmbedBatch generates embeddings for texts in batches.
	// This is useful for providers with large batch limits or
	// when memory efficiency is important.
	EmbedBatch(texts []string, batchSize int) ([][]float32, error)
}

// EmbedResult holds the result of an embedding operation
type EmbedResult struct {
	// Embeddings is the slice of embedding vectors
	Embeddings [][]float32

	// Model used for embedding
	Model string

	// Tokens used (if available)
	Tokens int
}
