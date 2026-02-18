package embed

import (
	"errors"
	"log"
	"testing"
)

// mockDimensionedProvider implements DimensionedProvider for testing
type mockDimensionedProvider struct {
	dimension    int
	dimensionErr error
}

func (m *mockDimensionedProvider) Embed(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = make([]float32, m.dimension)
	}
	return embeddings, nil
}

func (m *mockDimensionedProvider) Config() *Config {
	return &Config{Model: "test-model", Dimensions: m.dimension}
}

func (m *mockDimensionedProvider) Dimension() (int, error) {
	return m.dimension, m.dimensionErr
}

// mockNonDimensionedProvider implements Provider but not DimensionedProvider
type mockNonDimensionedProvider struct{}

func (m *mockNonDimensionedProvider) Embed(texts []string) ([][]float32, error) {
	return [][]float32{{0.1, 0.2, 0.3}}, nil
}

func (m *mockNonDimensionedProvider) Config() *Config {
	return &Config{Model: "test-model", Dimensions: 3}
}

// TestGetDimension tests the GetDimension function
func TestGetDimension(t *testing.T) {
	tests := []struct {
		name        string
		provider    Provider
		wantDim     int
		wantErr     bool
		errContains string
	}{
		{
			name:     "dimensioned provider returns dimension",
			provider: &mockDimensionedProvider{dimension: 768},
			wantDim:  768,
			wantErr:  false,
		},
		{
			name:        "dimensioned provider returns error",
			provider:    &mockDimensionedProvider{dimension: 0, dimensionErr: errors.New("provider error")},
			wantDim:     0,
			wantErr:     true,
			errContains: "provider error",
		},
		{
			name:        "non-dimensioned provider returns error",
			provider:    &mockNonDimensionedProvider{},
			wantDim:     0,
			wantErr:     true,
			errContains: "does not support dimension reporting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dim, err := GetDimension(tt.provider)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if dim != tt.wantDim {
					t.Errorf("dimension = %d, want %d", dim, tt.wantDim)
				}
			}
		})
	}
}

// TestCompatibleDimensions tests the CompatibleDimensions function
func TestCompatibleDimensions(t *testing.T) {
	tests := []struct {
		name       string
		provider1  Provider
		provider2  Provider
		wantCompat bool
		wantDim    int
		wantErr    bool
	}{
		{
			name:       "both providers same dimension",
			provider1:  &mockDimensionedProvider{dimension: 768},
			provider2:  &mockDimensionedProvider{dimension: 768},
			wantCompat: true,
			wantDim:    768,
			wantErr:    false,
		},
		{
			name:       "both providers different dimensions",
			provider1:  &mockDimensionedProvider{dimension: 768},
			provider2:  &mockDimensionedProvider{dimension: 384},
			wantCompat: false,
			wantDim:    0,
			wantErr:    true,
		},
		{
			name:       "neither provider supports dimension reporting",
			provider1:  &mockNonDimensionedProvider{},
			provider2:  &mockNonDimensionedProvider{},
			wantCompat: true,
			wantDim:    0,
			wantErr:    false,
		},
		{
			name:       "only provider1 supports dimension reporting",
			provider1:  &mockDimensionedProvider{dimension: 512},
			provider2:  &mockNonDimensionedProvider{},
			wantCompat: true,
			wantDim:    512,
			wantErr:    false,
		},
		{
			name:       "only provider2 supports dimension reporting",
			provider1:  &mockNonDimensionedProvider{},
			provider2:  &mockDimensionedProvider{dimension: 256},
			wantCompat: true,
			wantDim:    256,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compat, dim, err := CompatibleDimensions(tt.provider1, tt.provider2)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if !errors.Is(err, ErrDimensionMismatch) {
					t.Errorf("expected ErrDimensionMismatch, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
			if compat != tt.wantCompat {
				t.Errorf("compatible = %v, want %v", compat, tt.wantCompat)
			}
			if dim != tt.wantDim {
				t.Errorf("dimension = %d, want %d", dim, tt.wantDim)
			}
		})
	}
}

// TestValidateSearchCompatibility tests the ValidateSearchCompatibility function
func TestValidateSearchCompatibility(t *testing.T) {
	tests := []struct {
		name           string
		indexDim       int
		searchProvider Provider
		wantErr        bool
		errContains    string
	}{
		{
			name:           "matching dimensions",
			indexDim:       768,
			searchProvider: &mockDimensionedProvider{dimension: 768},
			wantErr:        false,
		},
		{
			name:           "mismatching dimensions returns error",
			indexDim:       768,
			searchProvider: &mockDimensionedProvider{dimension: 384},
			wantErr:        true,
			errContains:    "dimension mismatch",
		},
		{
			name:           "search provider doesn't support dimensions",
			indexDim:       768,
			searchProvider: &mockNonDimensionedProvider{},
			wantErr:        true,
			errContains:    "cannot determine search provider dimension",
		},
		{
			name:           "zero index dimension with matching provider",
			indexDim:       0,
			searchProvider: &mockDimensionedProvider{dimension: 512},
			wantErr:        true,
			errContains:    "dimension mismatch",
		},
		{
			name:           "zero index dimension with non-dimensioned provider",
			indexDim:       0,
			searchProvider: &mockNonDimensionedProvider{},
			wantErr:        true,
			errContains:    "cannot determine search provider dimension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSearchCompatibility(tt.indexDim, tt.searchProvider)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestWarnDimensionMismatch tests the WarnDimensionMismatch function
func TestWarnDimensionMismatch(t *testing.T) {
	tests := []struct {
		name           string
		indexDim       int
		searchProvider Provider
		wantWarn       bool
		warnContains   string
	}{
		{
			name:           "matching dimensions - no warning",
			indexDim:       768,
			searchProvider: &mockDimensionedProvider{dimension: 768},
			wantWarn:       false,
		},
		{
			name:           "mismatching dimensions - warning logged",
			indexDim:       768,
			searchProvider: &mockDimensionedProvider{dimension: 384},
			wantWarn:       true,
			warnContains:   "dimension mismatch",
		},
		{
			name:           "non-dimensioned provider - warning about cannot determine",
			indexDim:       768,
			searchProvider: &mockNonDimensionedProvider{},
			wantWarn:       true,
			warnContains:   "cannot determine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output
			var logOutput string
			log.SetOutput(&logWriter{&logOutput})
			defer log.SetOutput(nil)

			WarnDimensionMismatch(tt.indexDim, tt.searchProvider)

			if tt.wantWarn {
				if !contains(logOutput, tt.warnContains) {
					t.Errorf("expected log to contain %v, got %v", tt.warnContains, logOutput)
				}
			} else {
				if logOutput != "" {
					t.Errorf("expected no log output, got %v", logOutput)
				}
			}
		})
	}
}

// logWriter captures log output for testing
type logWriter struct {
	output *string
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	*lw.output += string(p)
	return len(p), nil
}

// contains is a simple substring check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestErrorVariables tests the exported error variables
func TestErrorVariables(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{
			name:    "ErrInvalidInput",
			err:     ErrInvalidInput,
			wantMsg: "invalid input",
		},
		{
			name:    "ErrProviderUnavailable",
			err:     ErrProviderUnavailable,
			wantMsg: "provider unavailable",
		},
		{
			name:    "ErrAPIKeyMissing",
			err:     ErrAPIKeyMissing,
			wantMsg: "API key missing",
		},
		{
			name:    "ErrInvalidModel",
			err:     ErrInvalidModel,
			wantMsg: "invalid model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %v, want %v", got, tt.wantMsg)
			}
		})
	}
}

// TestEmbeddingError tests the EmbeddingError struct and its methods
func TestEmbeddingError(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		message    string
		err        error
		wantMsg    string
		wantUnwrap error
	}{
		{
			name:       "without wrapped error",
			provider:   "ollama",
			message:    "connection failed",
			err:        nil,
			wantMsg:    "embedding error (ollama): connection failed",
			wantUnwrap: nil,
		},
		{
			name:       "with wrapped error",
			provider:   "huggingface",
			message:    "API request failed",
			err:        errors.New("timeout"),
			wantMsg:    "embedding error (huggingface): API request failed: timeout",
			wantUnwrap: errors.New("timeout"),
		},
		{
			name:       "empty provider and message",
			provider:   "",
			message:    "",
			err:        nil,
			wantMsg:    "embedding error (): ",
			wantUnwrap: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EmbeddingError{
				Provider: tt.provider,
				Message:  tt.message,
				Err:      tt.err,
			}

			if got := e.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %v, want %v", got, tt.wantMsg)
			}

			gotUnwrap := e.Unwrap()
			if tt.wantUnwrap == nil && gotUnwrap != nil {
				t.Errorf("Unwrap() = %v, want nil", gotUnwrap)
			}
			if tt.wantUnwrap != nil && gotUnwrap == nil {
				t.Errorf("Unwrap() = nil, want %v", tt.wantUnwrap)
			}
			if tt.wantUnwrap != nil && gotUnwrap != nil && gotUnwrap.Error() != tt.wantUnwrap.Error() {
				t.Errorf("Unwrap() = %v, want %v", gotUnwrap, tt.wantUnwrap)
			}
		})
	}
}

// TestConfigValidate tests the Config.Validate method
func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				Endpoint: "http://localhost:11434",
				Model:    "nomic-embed-text",
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: Config{
				Endpoint:   "http://localhost:11434",
				APIKey:     "secret-key",
				Model:      "nomic-embed-text",
				BatchSize:  32,
				Dimensions: 768,
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			config: Config{
				Model: "nomic-embed-text",
			},
			wantErr: true,
			errMsg:  "endpoint is required",
		},
		{
			name: "missing model",
			config: Config{
				Endpoint: "http://localhost:11434",
			},
			wantErr: true,
			errMsg:  "model is required",
		},
		{
			name:    "both endpoint and model missing",
			config:  Config{},
			wantErr: true,
			errMsg:  "endpoint is required",
		},
		{
			name: "empty endpoint and model",
			config: Config{
				Endpoint: "",
				Model:    "",
			},
			wantErr: true,
			errMsg:  "endpoint is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestConfigFields tests the Config struct fields
func TestConfigFields(t *testing.T) {
	config := Config{
		Endpoint:   "http://localhost:11434",
		APIKey:     "test-key",
		Model:      "nomic-embed-text",
		BatchSize:  32,
		Dimensions: 768,
	}

	if config.Endpoint != "http://localhost:11434" {
		t.Errorf("Endpoint = %v, want http://localhost:11434", config.Endpoint)
	}
	if config.APIKey != "test-key" {
		t.Errorf("APIKey = %v, want test-key", config.APIKey)
	}
	if config.Model != "nomic-embed-text" {
		t.Errorf("Model = %v, want nomic-embed-text", config.Model)
	}
	if config.BatchSize != 32 {
		t.Errorf("BatchSize = %v, want 32", config.BatchSize)
	}
	if config.Dimensions != 768 {
		t.Errorf("Dimensions = %v, want 768", config.Dimensions)
	}
}

// mockProvider is a mock implementation of Provider for testing
type mockProvider struct {
	embedFunc func(texts []string) ([][]float32, error)
	config    *Config
}

func (m *mockProvider) Embed(texts []string) ([][]float32, error) {
	return m.embedFunc(texts)
}

func (m *mockProvider) Config() *Config {
	return m.config
}

// TestProviderInterface tests that mockProvider implements Provider interface
func TestProviderInterface(t *testing.T) {
	var _ Provider = (*mockProvider)(nil)
}

// TestMockProviderEmbed tests the mockProvider.Embed method
func TestMockProviderEmbed(t *testing.T) {
	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}

	mp := &mockProvider{
		embedFunc: func(texts []string) ([][]float32, error) {
			if len(texts) != 2 {
				t.Errorf("expected 2 texts, got %d", len(texts))
			}
			return embeddings, nil
		},
		config: &Config{
			Endpoint: "http://localhost:11434",
			Model:    "test-model",
		},
	}

	result, err := mp.Embed([]string{"hello", "world"})
	if err != nil {
		t.Errorf("Embed() unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Embed() returned %d embeddings, want 2", len(result))
	}
}

// TestMockProviderConfig tests the mockProvider.Config method
func TestMockProviderConfig(t *testing.T) {
	expectedConfig := &Config{
		Endpoint:   "http://localhost:11434",
		APIKey:     "test-key",
		Model:      "test-model",
		BatchSize:  16,
		Dimensions: 384,
	}

	mp := &mockProvider{
		embedFunc: func(texts []string) ([][]float32, error) {
			return nil, nil
		},
		config: expectedConfig,
	}

	cfg := mp.Config()
	if cfg != expectedConfig {
		t.Errorf("Config() = %v, want %v", cfg, expectedConfig)
	}
}

// mockBatchProvider is a mock implementation of BatchProvider for testing
type mockBatchProvider struct {
	mockProvider
	embedBatchFunc func(texts []string, batchSize int) ([][]float32, error)
}

func (m *mockBatchProvider) EmbedBatch(texts []string, batchSize int) ([][]float32, error) {
	return m.embedBatchFunc(texts, batchSize)
}

// TestBatchProviderInterface tests that mockBatchProvider implements BatchProvider interface
func TestBatchProviderInterface(t *testing.T) {
	var _ BatchProvider = (*mockBatchProvider)(nil)
}

// TestMockBatchProviderEmbedBatch tests the mockBatchProvider.EmbedBatch method
func TestMockBatchProviderEmbedBatch(t *testing.T) {
	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
		{0.7, 0.8, 0.9},
	}

	mp := &mockBatchProvider{
		mockProvider: mockProvider{
			embedFunc: func(texts []string) ([][]float32, error) {
				return embeddings, nil
			},
			config: &Config{
				Endpoint: "http://localhost:11434",
				Model:    "test-model",
			},
		},
		embedBatchFunc: func(texts []string, batchSize int) ([][]float32, error) {
			if batchSize != 2 {
				t.Errorf("expected batchSize 2, got %d", batchSize)
			}
			return embeddings, nil
		},
	}

	result, err := mp.EmbedBatch([]string{"a", "b", "c"}, 2)
	if err != nil {
		t.Errorf("EmbedBatch() unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("EmbedBatch() returned %d embeddings, want 3", len(result))
	}
}

// TestEmbedResult tests the EmbedResult struct
func TestEmbedResult(t *testing.T) {
	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}

	result := EmbedResult{
		Embeddings: embeddings,
		Model:      "nomic-embed-text",
		Tokens:     100,
	}

	if len(result.Embeddings) != 2 {
		t.Errorf("Embeddings length = %d, want 2", len(result.Embeddings))
	}
	if result.Model != "nomic-embed-text" {
		t.Errorf("Model = %v, want nomic-embed-text", result.Model)
	}
	if result.Tokens != 100 {
		t.Errorf("Tokens = %d, want 100", result.Tokens)
	}
}

// TestProviderInterfaceEmbeddingError tests Provider error wrapping with EmbeddingError
func TestProviderInterfaceEmbeddingError(t *testing.T) {
	mp := &mockProvider{
		embedFunc: func(texts []string) ([][]float32, error) {
			return nil, &EmbeddingError{
				Provider: "test-provider",
				Message:  "failed to get embeddings",
				Err:      ErrProviderUnavailable,
			}
		},
		config: &Config{
			Endpoint: "http://localhost:11434",
			Model:    "test-model",
		},
	}

	_, err := mp.Embed([]string{"test"})
	if err == nil {
		t.Errorf("expected error, got nil")
	}

	var embedErr *EmbeddingError
	if !errors.As(err, &embedErr) {
		t.Errorf("expected EmbeddingError, got %T", err)
	}

	if embedErr.Provider != "test-provider" {
		t.Errorf("Provider = %v, want test-provider", embedErr.Provider)
	}
	if embedErr.Unwrap() != ErrProviderUnavailable {
		t.Errorf("Unwrap() = %v, want ErrProviderUnavailable", embedErr.Unwrap())
	}
}
