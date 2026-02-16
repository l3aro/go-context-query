package embed

import (
	"errors"
	"testing"
)

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
