package embed

import (
	"errors"
	"fmt"
	"log"
)

// ErrInvalidInput is returned when the input is invalid
var ErrInvalidInput = errors.New("invalid input")

// ErrProviderUnavailable is returned when the embedding provider is unavailable
var ErrProviderUnavailable = errors.New("provider unavailable")

// ErrAPIKeyMissing is returned when the API key is missing
var ErrAPIKeyMissing = errors.New("API key missing")

// ErrInvalidModel is returned when the model is invalid
var ErrInvalidModel = errors.New("invalid model")

// ErrDimensionMismatch is returned when provider dimensions don't match
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")

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

// DimensionedProvider is an optional interface for providers that can report their embedding dimension
type DimensionedProvider interface {
	// Dimension returns the embedding dimension. May require a test embedding call.
	Dimension() (int, error)
}

// GetDimension returns the embedding dimension for a provider.
// Returns 0 and error if the provider doesn't support dimension reporting.
func GetDimension(p Provider) (int, error) {
	if dp, ok := p.(DimensionedProvider); ok {
		return dp.Dimension()
	}
	return 0, fmt.Errorf("provider does not support dimension reporting")
}

// CompatibleDimensions checks if two providers have compatible embedding dimensions.
// Returns true if both have the same dimension, or if either cannot report dimension.
func CompatibleDimensions(p1, p2 Provider) (bool, int, error) {
	dim1, err1 := GetDimension(p1)
	dim2, err2 := GetDimension(p2)

	// If neither can report, assume compatible
	if err1 != nil && err2 != nil {
		return true, 0, nil
	}

	// If only one can report, warn but assume compatible
	if err1 != nil {
		return true, dim2, nil
	}
	if err2 != nil {
		return true, dim1, nil
	}

	// Both can report - check compatibility
	if dim1 != dim2 {
		return false, 0, fmt.Errorf("%w: provider1=%d, provider2=%d", ErrDimensionMismatch, dim1, dim2)
	}

	return true, dim1, nil
}

// WarnDimensionMismatch logs a warning if two providers have different dimensions.
// This should be called when initializing search with a different provider than indexing.
func WarnDimensionMismatch(indexDim int, searchProvider Provider) {
	searchDim, err := GetDimension(searchProvider)
	if err != nil {
		log.Printf("Warning: cannot determine search provider dimension: %v", err)
		return
	}

	if indexDim != searchDim {
		log.Printf("Warning: dimension mismatch - index dimension=%d, search dimension=%d. Search results may be incorrect.",
			indexDim, searchDim)
	}
}

// ValidateSearchCompatibility returns an error if the search provider is incompatible with the index dimension.
// This should be called before performing search to ensure dimension compatibility.
func ValidateSearchCompatibility(indexDim int, searchProvider Provider) error {
	searchDim, err := GetDimension(searchProvider)
	if err != nil {
		return fmt.Errorf("cannot determine search provider dimension: %w", err)
	}

	if indexDim != searchDim {
		return fmt.Errorf("%w: index=%d, search=%d", ErrDimensionMismatch, indexDim, searchDim)
	}

	return nil
}
