package embed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/l3aro/go-context-query/internal/config"
)

// RetryConfig holds configuration for retry behavior.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (default: 3)
	MaxRetries int
	// InitialBackoff is the initial delay before first retry (default: 100ms)
	InitialBackoff time.Duration
	// BackoffMultiplier is the factor to multiply backoff by each retry (default: 2)
	BackoffMultiplier float64
	// MaxBackoff is the maximum delay between retries (default: 2s)
	MaxBackoff time.Duration
}

// DefaultRetryConfig returns a RetryConfig with sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		MaxBackoff:        2 * time.Second,
	}
}

// isRetryableError determines if an error is transient and should be retried.
// Returns false for client errors (invalid input, auth errors) and true for
// transient errors (network issues, server errors).
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Don't retry invalid input errors
	if errors.Is(err, ErrInvalidInput) {
		return false
	}

	// Don't retry missing API key errors
	if errors.Is(err, ErrAPIKeyMissing) {
		return false
	}

	// Don't retry invalid model errors
	if errors.Is(err, ErrInvalidModel) {
		return false
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Check for HTTP status codes in error message
	errStr := err.Error()

	// Retry on 5xx server errors
	if strings.Contains(errStr, "status 5") {
		return true
	}

	// Retry on 429 rate limit
	if strings.Contains(errStr, "status 429") {
		return true
	}

	// Retry on specific network error indicators
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "request failed") {
		return true
	}

	// Retry on provider unavailable errors (could be transient)
	if errors.Is(err, ErrProviderUnavailable) {
		return true
	}

	return false
}

// embedWithRetry attempts to generate embeddings with retry logic and exponential backoff.
func (s *EmbeddingService) embedWithRetry(ctx context.Context, provider Provider, texts []string) ([][]float32, error) {
	backoff := s.retryCfg.InitialBackoff
	var lastErr error

	for attempt := 0; attempt <= s.retryCfg.MaxRetries; attempt++ {
		// Check for context cancellation before each attempt
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		embeddings, err := provider.Embed(texts)
		if err == nil {
			return embeddings, nil
		}

		lastErr = err

		// Don't retry if error is not retryable
		if !isRetryableError(err) {
			return nil, err
		}

		// Don't wait after the last attempt
		if attempt < s.retryCfg.MaxRetries {
			// Wait with backoff
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-timer.C:
				// Continue to next retry
			}

			// Increase backoff for next attempt, capped at MaxBackoff
			backoff = time.Duration(float64(backoff) * s.retryCfg.BackoffMultiplier)
			if backoff > s.retryCfg.MaxBackoff {
				backoff = s.retryCfg.MaxBackoff
			}
		}
	}

	return nil, fmt.Errorf("all %d retries exhausted: %w", s.retryCfg.MaxRetries, lastErr)
}

// EmbeddingService wraps embedding providers with caching support.
// It manages two providers: warm for indexing and search for queries.
type EmbeddingService struct {
	warmProvider   Provider
	searchProvider Provider
	cache          map[string][]float32
	mu             sync.RWMutex
	retryCfg       RetryConfig
}

// NewEmbeddingService creates a new EmbeddingService with providers based on config.
// It initializes both warm and search providers from the configuration.
func NewEmbeddingService(cfg *config.Config) (*EmbeddingService, error) {
	return NewEmbeddingServiceWithRetry(cfg, DefaultRetryConfig())
}

// NewEmbeddingServiceWithRetry creates a new EmbeddingService with custom retry configuration.
func NewEmbeddingServiceWithRetry(cfg *config.Config, retryCfg RetryConfig) (*EmbeddingService, error) {
	warmProvider, err := createProviderFromConfig(cfg, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create warm provider: %w", err)
	}

	searchProvider, err := createProviderFromConfig(cfg, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create search provider: %w", err)
	}

	return &EmbeddingService{
		warmProvider:   warmProvider,
		searchProvider: searchProvider,
		cache:          make(map[string][]float32),
		retryCfg:       retryCfg,
	}, nil
}

// createProviderFromConfig creates a provider from config for warm or search.
func createProviderFromConfig(cfg *config.Config, isWarm bool) (Provider, error) {
	var providerType config.ProviderType
	var model, baseURL, token string

	if isWarm {
		providerType = cfg.EffectiveWarmProvider()
		if cfg.Warm.Model != "" {
			model = cfg.Warm.Model
		} else if cfg.OllamaModel != "" {
			model = cfg.OllamaModel
		}
		if cfg.Warm.BaseURL != "" {
			baseURL = cfg.Warm.BaseURL
		} else if cfg.OllamaBaseURL != "" {
			baseURL = cfg.OllamaBaseURL
		}
		if cfg.Warm.Token != "" {
			token = cfg.Warm.Token
		} else if cfg.HFToken != "" {
			token = cfg.HFToken
		}
	} else {
		providerType = cfg.EffectiveSearchProvider()
		if cfg.Search.Model != "" {
			model = cfg.Search.Model
		} else if cfg.OllamaModel != "" {
			model = cfg.OllamaModel
		}
		if cfg.Search.BaseURL != "" {
			baseURL = cfg.Search.BaseURL
		} else if cfg.OllamaBaseURL != "" {
			baseURL = cfg.OllamaBaseURL
		}
		if cfg.Search.Token != "" {
			token = cfg.Search.Token
		} else if cfg.HFToken != "" {
			token = cfg.HFToken
		}
	}

	embedConfig := &Config{
		Endpoint: baseURL,
		APIKey:   token,
		Model:    model,
	}

	return NewProvider(providerType, embedConfig)
}

// Embed generates embeddings for the given texts using the specified purpose.
// Purpose must be either "warm" (for indexing) or "search" (for queries).
// Results are cached to avoid redundant API calls.
func (s *EmbeddingService) Embed(ctx context.Context, purpose string, texts []string) ([][]float32, error) {
	var provider Provider
	switch purpose {
	case "warm":
		provider = s.warmProvider
	case "search":
		provider = s.searchProvider
	default:
		return nil, fmt.Errorf("unknown purpose: %s (must be 'warm' or 'search')", purpose)
	}

	if provider == nil {
		return nil, errors.New("provider not initialized")
	}

	if len(texts) == 0 {
		return nil, ErrInvalidInput
	}

	results := make([][]float32, len(texts))
	var textsToEmbed []string
	var indicesToEmbed []int

	s.mu.RLock()
	for i, text := range texts {
		key := hashText(text)
		if cached, ok := s.cache[key]; ok {
			results[i] = cached
		} else {
			textsToEmbed = append(textsToEmbed, text)
			indicesToEmbed = append(indicesToEmbed, i)
		}
	}
	s.mu.RUnlock()

	if len(textsToEmbed) == 0 {
		return results, nil
	}

	// Attempt embedding with retries and exponential backoff
	embeddings, err := s.embedWithRetry(ctx, provider, textsToEmbed)
	if err != nil {
		return nil, fmt.Errorf("embedding failed after %d retries: %w", s.retryCfg.MaxRetries, err)
	}

	if len(embeddings) != len(textsToEmbed) {
		return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(textsToEmbed), len(embeddings))
	}

	s.mu.Lock()
	for i, embedding := range embeddings {
		key := hashText(textsToEmbed[i])
		s.cache[key] = embedding
		results[indicesToEmbed[i]] = embedding
	}
	s.mu.Unlock()

	return results, nil
}

// hashText creates a SHA256 hash of the text for use as a cache key.
func hashText(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

// ClearCache removes all cached embeddings.
func (s *EmbeddingService) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string][]float32)
}

// CacheSize returns the number of cached embeddings.
func (s *EmbeddingService) CacheSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cache)
}

// WarmProvider returns the warm provider used by the service.
// This can be used to pass the provider to functions expecting embed.Provider.
func (s *EmbeddingService) WarmProvider() Provider {
	return s.warmProvider
}

// SearchProvider returns the search provider used by the service.
// This can be used to pass the provider to functions expecting embed.Provider.
func (s *EmbeddingService) SearchProvider() Provider {
	return s.searchProvider
}
