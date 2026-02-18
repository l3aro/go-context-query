package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		// Legacy fields are now empty by default (nested config is preferred)
		{"Provider", cfg.Provider, ProviderType("")},
		{"HFModel", cfg.HFModel, ""},
		{"OllamaModel", cfg.OllamaModel, ""},
		{"OllamaBaseURL", cfg.OllamaBaseURL, ""},
		{"OllamaAPIKey", cfg.OllamaAPIKey, ""},
		// Global settings
		{"SocketPath", cfg.SocketPath, "/tmp/gcq.sock"},
		{"ThresholdSimilarity", cfg.ThresholdSimilarity, 0.7},
		{"ThresholdMinScore", cfg.ThresholdMinScore, 0.5},
		{"MaxContextChunks", cfg.MaxContextChunks, 10},
		{"ChunkOverlap", cfg.ChunkOverlap, 100},
		{"ChunkSize", cfg.ChunkSize, 512},
		{"Verbose", cfg.Verbose, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("DefaultConfig().%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid ollama config",
			cfg: &Config{
				Provider:         ProviderOllama,
				OllamaModel:      "nomic-embed-text",
				OllamaBaseURL:    "http://localhost:11434",
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr: false,
		},
		{
			name: "valid huggingface config",
			cfg: &Config{
				Provider:         ProviderHuggingFace,
				HFModel:          "sentence-transformers/all-MiniLM-L6-v2",
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr: false,
		},
		{
			name: "invalid provider",
			cfg: &Config{
				Provider:         "invalid",
				OllamaModel:      "test",
				OllamaBaseURL:    "http://localhost:11434",
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "invalid provider",
		},
		{
			name: "missing hf_model for huggingface",
			cfg: &Config{
				Provider:         ProviderHuggingFace,
				HFModel:          "",
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "hf_model is required",
		},
		{
			name: "missing ollama_model for ollama",
			cfg: &Config{
				Provider:         ProviderOllama,
				OllamaModel:      "",
				OllamaBaseURL:    "http://localhost:11434",
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "ollama_model is required",
		},
		{
			name: "missing ollama_base_url for ollama",
			cfg: &Config{
				Provider:         ProviderOllama,
				OllamaModel:      "test",
				OllamaBaseURL:    "",
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "ollama_base_url is required",
		},
		{
			name: "threshold_similarity out of range",
			cfg: &Config{
				Provider:            ProviderOllama,
				OllamaModel:         "test",
				OllamaBaseURL:       "http://localhost:11434",
				ThresholdSimilarity: 1.5,
				ChunkSize:           512,
				ChunkOverlap:        100,
				MaxContextChunks:    10,
			},
			wantErr:     true,
			errContains: "threshold_similarity must be between 0 and 1",
		},
		{
			name: "threshold_min_score out of range",
			cfg: &Config{
				Provider:          ProviderOllama,
				OllamaModel:       "test",
				OllamaBaseURL:     "http://localhost:11434",
				ThresholdMinScore: -0.1,
				ChunkSize:         512,
				ChunkOverlap:      100,
				MaxContextChunks:  10,
			},
			wantErr:     true,
			errContains: "threshold_min_score must be between 0 and 1",
		},
		{
			name: "invalid chunk_size",
			cfg: &Config{
				Provider:         ProviderOllama,
				OllamaModel:      "test",
				OllamaBaseURL:    "http://localhost:11434",
				ChunkSize:        0,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "chunk_size must be positive",
		},
		{
			name: "invalid chunk_overlap",
			cfg: &Config{
				Provider:         ProviderOllama,
				OllamaModel:      "test",
				OllamaBaseURL:    "http://localhost:11434",
				ChunkSize:        512,
				ChunkOverlap:     -1,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "chunk_overlap must be non-negative",
		},
		{
			name: "invalid max_context_chunks",
			cfg: &Config{
				Provider:         ProviderOllama,
				OllamaModel:      "test",
				OllamaBaseURL:    "http://localhost:11434",
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 0,
			},
			wantErr:     true,
			errContains: "max_context_chunks must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
				} else if !contains(err.Error(), tt.errContains) {
					t.Errorf("Error = %q, should contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		envVars     map[string]string
		checkCfg    func(*testing.T, *Config)
		wantErr     bool
		errContains string
	}{
		{
			name: "load valid config from file",
			configYAML: `
provider: ollama
ollama_model: custom-model
ollama_base_url: http://localhost:8080
socket_path: /custom/path.sock
threshold_similarity: 0.8
threshold_min_score: 0.6
max_context_chunks: 20
chunk_overlap: 150
chunk_size: 1024
verbose: true
`,
			checkCfg: func(t *testing.T, cfg *Config) {
				if cfg.Provider != ProviderOllama {
					t.Errorf("Provider = %v, want %v", cfg.Provider, ProviderOllama)
				}
				if cfg.OllamaModel != "custom-model" {
					t.Errorf("OllamaModel = %v, want custom-model", cfg.OllamaModel)
				}
				if cfg.OllamaBaseURL != "http://localhost:8080" {
					t.Errorf("OllamaBaseURL = %v, want http://localhost:8080", cfg.OllamaBaseURL)
				}
				if cfg.SocketPath != "/custom/path.sock" {
					t.Errorf("SocketPath = %v, want /custom/path.sock", cfg.SocketPath)
				}
				if cfg.ThresholdSimilarity != 0.8 {
					t.Errorf("ThresholdSimilarity = %v, want 0.8", cfg.ThresholdSimilarity)
				}
				if cfg.ThresholdMinScore != 0.6 {
					t.Errorf("ThresholdMinScore = %v, want 0.6", cfg.ThresholdMinScore)
				}
				if cfg.MaxContextChunks != 20 {
					t.Errorf("MaxContextChunks = %v, want 20", cfg.MaxContextChunks)
				}
				if cfg.ChunkOverlap != 150 {
					t.Errorf("ChunkOverlap = %v, want 150", cfg.ChunkOverlap)
				}
				if cfg.ChunkSize != 1024 {
					t.Errorf("ChunkSize = %v, want 1024", cfg.ChunkSize)
				}
				if !cfg.Verbose {
					t.Error("Verbose = false, want true")
				}
			},
			wantErr: false,
		},
		{
			name: "load huggingface config from file",
			configYAML: `
provider: huggingface
hf_model: custom-hf-model
hf_token: secret-token
`,
			checkCfg: func(t *testing.T, cfg *Config) {
				if cfg.Provider != ProviderHuggingFace {
					t.Errorf("Provider = %v, want %v", cfg.Provider, ProviderHuggingFace)
				}
				if cfg.HFModel != "custom-hf-model" {
					t.Errorf("HFModel = %v, want custom-hf-model", cfg.HFModel)
				}
				if cfg.HFToken != "secret-token" {
					t.Errorf("HFToken = %v, want secret-token", cfg.HFToken)
				}
			},
			wantErr: false,
		},
		{
			name: "env var overrides file values",
			configYAML: `
provider: huggingface
hf_model: file-model
ollama_model: file-ollama
`,
			envVars: map[string]string{
				"GCQ_WARM_PROVIDER":        "ollama",
				"GCQ_WARM_OLLAMA_MODEL":    "env-ollama-model",
				"GCQ_WARM_OLLAMA_BASE_URL": "http://localhost:11434",
			},
			checkCfg: func(t *testing.T, cfg *Config) {
				if cfg.Warm.Provider != ProviderOllama {
					t.Errorf("Warm.Provider = %v, want %v (from env)", cfg.Warm.Provider, ProviderOllama)
				}
				if cfg.Warm.Model != "env-ollama-model" {
					t.Errorf("Warm.Model = %v, want env-ollama-model (from env)", cfg.Warm.Model)
				}
				// HFModel should still be from file (legacy field)
				if cfg.HFModel != "file-model" {
					t.Errorf("HFModel = %v, want file-model (from file)", cfg.HFModel)
				}
			},
			wantErr: false,
		},
		{
			name: "invalid yaml",
			configYAML: `
provider: ollama
  invalid: indent
`,
			wantErr:     true,
			errContains: "failed to parse",
		},
		{
			name: "invalid provider in file",
			configYAML: `
warm:
  provider: invalid
  model: test
  base_url: http://localhost:11434
search:
  provider: invalid
  model: test
  base_url: http://localhost:11434
chunk_size: 512
chunk_overlap: 100
max_context_chunks: 10
`,
			wantErr:     true,
			errContains: "invalid warm.provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars if specified
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Create temp config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			err := os.WriteFile(configPath, []byte(tt.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			cfg, err := LoadFromFile(configPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
				} else if !contains(err.Error(), tt.errContains) {
					t.Errorf("Error = %q, should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.checkCfg != nil {
				tt.checkCfg(t, cfg)
			}
		})
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	// Save original env and restore after
	origEnv := os.Environ()
	defer func() {
		os.Unsetenv("GCQ_PROVIDER")
		os.Unsetenv("GCQ_HF_MODEL")
		os.Unsetenv("GCQ_HF_TOKEN")
		os.Unsetenv("GCQ_OLLAMA_MODEL")
		os.Unsetenv("GCQ_OLLAMA_BASE_URL")
		os.Unsetenv("GCQ_OLLAMA_API_KEY")
		os.Unsetenv("GCQ_SOCKET_PATH")
		os.Unsetenv("GCQ_THRESHOLD_SIMILARITY")
		os.Unsetenv("GCQ_THRESHOLD_MIN_SCORE")
		os.Unsetenv("GCQ_MAX_CONTEXT_CHUNKS")
		os.Unsetenv("GCQ_CHUNK_OVERLAP")
		os.Unsetenv("GCQ_CHUNK_SIZE")
		os.Unsetenv("GCQ_VERBOSE")
		for _, e := range origEnv {
			parts := splitEnv(e)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()

	tests := []struct {
		name    string
		envVars map[string]string
		check   func(*testing.T, *Config)
	}{
		{
			name: "override provider",
			envVars: map[string]string{
				"GCQ_PROVIDER": "huggingface",
			},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Provider != ProviderHuggingFace {
					t.Errorf("Provider = %v, want %v", cfg.Provider, ProviderHuggingFace)
				}
			},
		},
		{
			name: "override hf settings",
			envVars: map[string]string{
				"GCQ_HF_MODEL": "custom-hf-model",
				"GCQ_HF_TOKEN": "secret-token",
			},
			check: func(t *testing.T, cfg *Config) {
				if cfg.HFModel != "custom-hf-model" {
					t.Errorf("HFModel = %v, want custom-hf-model", cfg.HFModel)
				}
				if cfg.HFToken != "secret-token" {
					t.Errorf("HFToken = %v, want secret-token", cfg.HFToken)
				}
			},
		},
		{
			name: "override ollama settings",
			envVars: map[string]string{
				"GCQ_OLLAMA_MODEL":    "custom-ollama-model",
				"GCQ_OLLAMA_BASE_URL": "http://localhost:9000",
				"GCQ_OLLAMA_API_KEY":  "api-key-123",
			},
			check: func(t *testing.T, cfg *Config) {
				if cfg.OllamaModel != "custom-ollama-model" {
					t.Errorf("OllamaModel = %v, want custom-ollama-model", cfg.OllamaModel)
				}
				if cfg.OllamaBaseURL != "http://localhost:9000" {
					t.Errorf("OllamaBaseURL = %v, want http://localhost:9000", cfg.OllamaBaseURL)
				}
				if cfg.OllamaAPIKey != "api-key-123" {
					t.Errorf("OllamaAPIKey = %v, want api-key-123", cfg.OllamaAPIKey)
				}
			},
		},
		{
			name: "override numeric values",
			envVars: map[string]string{
				"GCQ_THRESHOLD_SIMILARITY": "0.85",
				"GCQ_THRESHOLD_MIN_SCORE":  "0.45",
				"GCQ_MAX_CONTEXT_CHUNKS":   "50",
				"GCQ_CHUNK_OVERLAP":        "200",
				"GCQ_CHUNK_SIZE":           "2048",
			},
			check: func(t *testing.T, cfg *Config) {
				if cfg.ThresholdSimilarity != 0.85 {
					t.Errorf("ThresholdSimilarity = %v, want 0.85", cfg.ThresholdSimilarity)
				}
				if cfg.ThresholdMinScore != 0.45 {
					t.Errorf("ThresholdMinScore = %v, want 0.45", cfg.ThresholdMinScore)
				}
				if cfg.MaxContextChunks != 50 {
					t.Errorf("MaxContextChunks = %v, want 50", cfg.MaxContextChunks)
				}
				if cfg.ChunkOverlap != 200 {
					t.Errorf("ChunkOverlap = %v, want 200", cfg.ChunkOverlap)
				}
				if cfg.ChunkSize != 2048 {
					t.Errorf("ChunkSize = %v, want 2048", cfg.ChunkSize)
				}
			},
		},
		{
			name: "override verbose with various true values",
			envVars: map[string]string{
				"GCQ_VERBOSE": "true",
			},
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Verbose {
					t.Error("Verbose = false, want true")
				}
			},
		},
		{
			name: "override verbose with 1",
			envVars: map[string]string{
				"GCQ_VERBOSE": "1",
			},
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Verbose {
					t.Error("Verbose = false, want true (from '1')")
				}
			},
		},
		{
			name: "override verbose with yes",
			envVars: map[string]string{
				"GCQ_VERBOSE": "yes",
			},
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Verbose {
					t.Error("Verbose = false, want true (from 'yes')")
				}
			},
		},
		{
			name: "invalid float ignored",
			envVars: map[string]string{
				"GCQ_THRESHOLD_SIMILARITY": "not-a-float",
			},
			check: func(t *testing.T, cfg *Config) {
				// Should keep default value
				if cfg.ThresholdSimilarity != 0.7 {
					t.Errorf("ThresholdSimilarity = %v, want 0.7 (default)", cfg.ThresholdSimilarity)
				}
			},
		},
		{
			name: "invalid int ignored",
			envVars: map[string]string{
				"GCQ_CHUNK_SIZE": "not-an-int",
			},
			check: func(t *testing.T, cfg *Config) {
				// Should keep default value
				if cfg.ChunkSize != 512 {
					t.Errorf("ChunkSize = %v, want 512 (default)", cfg.ChunkSize)
				}
			},
		},
		{
			name: "negative values ignored",
			envVars: map[string]string{
				"GCQ_CHUNK_SIZE": "-100",
			},
			check: func(t *testing.T, cfg *Config) {
				// Should keep default value
				if cfg.ChunkSize != 512 {
					t.Errorf("ChunkSize = %v, want 512 (default)", cfg.ChunkSize)
				}
			},
		},
		{
			name: "socket path override",
			envVars: map[string]string{
				"GCQ_SOCKET_PATH": "/my/custom/socket.sock",
			},
			check: func(t *testing.T, cfg *Config) {
				if cfg.SocketPath != "/my/custom/socket.sock" {
					t.Errorf("SocketPath = %v, want /my/custom/socket.sock", cfg.SocketPath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear any previously set env vars
			os.Unsetenv("GCQ_PROVIDER")
			os.Unsetenv("GCQ_HF_MODEL")
			os.Unsetenv("GCQ_HF_TOKEN")
			os.Unsetenv("GCQ_OLLAMA_MODEL")
			os.Unsetenv("GCQ_OLLAMA_BASE_URL")
			os.Unsetenv("GCQ_OLLAMA_API_KEY")
			os.Unsetenv("GCQ_SOCKET_PATH")
			os.Unsetenv("GCQ_THRESHOLD_SIMILARITY")
			os.Unsetenv("GCQ_THRESHOLD_MIN_SCORE")
			os.Unsetenv("GCQ_MAX_CONTEXT_CHUNKS")
			os.Unsetenv("GCQ_CHUNK_OVERLAP")
			os.Unsetenv("GCQ_CHUNK_SIZE")
			os.Unsetenv("GCQ_VERBOSE")

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg := DefaultConfig()
			applyEnvOverrides(cfg)

			tt.check(t, cfg)
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"0.5", 0.5},
		{"0.7", 0.7},
		{"1.0", 1.0},
		{"100.5", 100.5},
		{"0", 0},
		{"invalid", 0},
		{"", 0},
		{"abc123", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseFloat(tt.input)
			if result != tt.expected {
				t.Errorf("parseFloat(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"100", 100},
		{"512", 512},
		{"invalid", 0},
		{"", 0},
		{"abc123", 0},
		{"10.5", 10}, // Will parse 10 from 10.5
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseInt(tt.input)
			if result != tt.expected {
				t.Errorf("parseInt(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// Helper functions

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

func splitEnv(e string) []string {
	for i := 0; i < len(e); i++ {
		if e[i] == '=' {
			return []string{e[:i], e[i+1:]}
		}
	}
	return []string{e}
}

func TestConfigSave(t *testing.T) {
	// Test saving config to a temp file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{
		Provider:            ProviderOllama,
		OllamaModel:         "test-model",
		OllamaBaseURL:       "http://localhost:11434",
		ThresholdSimilarity: 0.8,
		MaxContextChunks:    15,
		ChunkSize:           1024,
		ChunkOverlap:        200,
	}

	// Test Save
	err := cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file was not created at %s", configPath)
	}

	// Verify roundtrip: load and compare
	loadedCfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() failed: %v", err)
	}

	if loadedCfg.Provider != cfg.Provider {
		t.Errorf("Provider mismatch: got %s, want %s", loadedCfg.Provider, cfg.Provider)
	}
	if loadedCfg.OllamaModel != cfg.OllamaModel {
		t.Errorf("OllamaModel mismatch: got %s, want %s", loadedCfg.OllamaModel, cfg.OllamaModel)
	}
	if loadedCfg.OllamaBaseURL != cfg.OllamaBaseURL {
		t.Errorf("OllamaBaseURL mismatch: got %s, want %s", loadedCfg.OllamaBaseURL, cfg.OllamaBaseURL)
	}
	if loadedCfg.ThresholdSimilarity != cfg.ThresholdSimilarity {
		t.Errorf("ThresholdSimilarity mismatch: got %v, want %v", loadedCfg.ThresholdSimilarity, cfg.ThresholdSimilarity)
	}
	if loadedCfg.MaxContextChunks != cfg.MaxContextChunks {
		t.Errorf("MaxContextChunks mismatch: got %d, want %d", loadedCfg.MaxContextChunks, cfg.MaxContextChunks)
	}
	if loadedCfg.ChunkSize != cfg.ChunkSize {
		t.Errorf("ChunkSize mismatch: got %d, want %d", loadedCfg.ChunkSize, cfg.ChunkSize)
	}
	if loadedCfg.ChunkOverlap != cfg.ChunkOverlap {
		t.Errorf("ChunkOverlap mismatch: got %d, want %d", loadedCfg.ChunkOverlap, cfg.ChunkOverlap)
	}
}

func TestConfigSaveCreatesParentDirs(t *testing.T) {
	// Test that Save creates parent directories if they don't exist
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nested", "dirs", "config.yaml")

	cfg := &Config{
		Provider:         ProviderHuggingFace,
		HFModel:          "test-model",
		ChunkSize:        512,
		ChunkOverlap:     100,
		MaxContextChunks: 10,
	}

	err := cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Save() failed to create parent dirs: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file was not created at %s", configPath)
	}
}

// TestNestedConfigUnmarshal tests the nested YAML format unmarshaling
func TestNestedConfigUnmarshal(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		checkCfg   func(*testing.T, *Config)
		wantErr    bool
	}{
		{
			name: "nested warm config with ollama",
			configYAML: `
warm:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:11434
  token: secret-token
search:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:11434
socket_path: /custom.sock
chunk_size: 512
chunk_overlap: 100
max_context_chunks: 10
`,
			checkCfg: func(t *testing.T, cfg *Config) {
				if cfg.Warm.Provider != ProviderOllama {
					t.Errorf("Warm.Provider = %v, want %v", cfg.Warm.Provider, ProviderOllama)
				}
				if cfg.Warm.Model != "nomic-embed-text" {
					t.Errorf("Warm.Model = %v, want nomic-embed-text", cfg.Warm.Model)
				}
				if cfg.Warm.BaseURL != "http://localhost:11434" {
					t.Errorf("Warm.BaseURL = %v, want http://localhost:11434", cfg.Warm.BaseURL)
				}
				if cfg.Warm.Token != "secret-token" {
					t.Errorf("Warm.Token = %v, want secret-token", cfg.Warm.Token)
				}
				if cfg.Search.Provider != ProviderOllama {
					t.Errorf("Search.Provider = %v, want %v", cfg.Search.Provider, ProviderOllama)
				}
				if cfg.Search.Model != "nomic-embed-text" {
					t.Errorf("Search.Model = %v, want nomic-embed-text", cfg.Search.Model)
				}
				if cfg.SocketPath != "/custom.sock" {
					t.Errorf("SocketPath = %v, want /custom.sock", cfg.SocketPath)
				}
			},
			wantErr: false,
		},
		{
			name: "nested warm config with huggingface",
			configYAML: `
warm:
  provider: huggingface
  model: sentence-transformers/all-MiniLM-L6-v2
  token: hf-token
search:
  provider: huggingface
  model: sentence-transformers/all-MiniLM-L6-v2
chunk_size: 512
chunk_overlap: 100
max_context_chunks: 10
`,
			checkCfg: func(t *testing.T, cfg *Config) {
				if cfg.Warm.Provider != ProviderHuggingFace {
					t.Errorf("Warm.Provider = %v, want %v", cfg.Warm.Provider, ProviderHuggingFace)
				}
				if cfg.Warm.Model != "sentence-transformers/all-MiniLM-L6-v2" {
					t.Errorf("Warm.Model = %v, want sentence-transformers/all-MiniLM-L6-v2", cfg.Warm.Model)
				}
				if cfg.Warm.Token != "hf-token" {
					t.Errorf("Warm.Token = %v, want hf-token", cfg.Warm.Token)
				}
				if cfg.Search.Provider != ProviderHuggingFace {
					t.Errorf("Search.Provider = %v, want %v", cfg.Search.Provider, ProviderHuggingFace)
				}
			},
			wantErr: false,
		},
		{
			name: "dual provider config - different providers",
			configYAML: `
warm:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:11434
search:
  provider: huggingface
  model: sentence-transformers/all-MiniLM-L6-v2
  token: hf-token
chunk_size: 512
chunk_overlap: 100
max_context_chunks: 10
`,
			checkCfg: func(t *testing.T, cfg *Config) {
				if cfg.Warm.Provider != ProviderOllama {
					t.Errorf("Warm.Provider = %v, want %v", cfg.Warm.Provider, ProviderOllama)
				}
				if cfg.Search.Provider != ProviderHuggingFace {
					t.Errorf("Search.Provider = %v, want %v", cfg.Search.Provider, ProviderHuggingFace)
				}
				if !cfg.IsDualProviderMode() {
					t.Error("IsDualProviderMode() = false, want true")
				}
			},
			wantErr: false,
		},
		{
			name: "partial nested config - warm only",
			configYAML: `
warm:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:11434
chunk_size: 512
chunk_overlap: 100
max_context_chunks: 10
`,
			checkCfg: func(t *testing.T, cfg *Config) {
				if cfg.Warm.Provider != ProviderOllama {
					t.Errorf("Warm.Provider = %v, want %v", cfg.Warm.Provider, ProviderOllama)
				}
				if cfg.Warm.Model != "nomic-embed-text" {
					t.Errorf("Warm.Model = %v, want nomic-embed-text", cfg.Warm.Model)
				}
				// Search.Provider remains empty when only warm is specified
				// Use EffectiveSearchProvider() to get the effective provider
				if cfg.Search.Provider != "" {
					t.Errorf("Search.Provider = %v, want empty (should use EffectiveSearchProvider)", cfg.Search.Provider)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			err := os.WriteFile(configPath, []byte(tt.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			cfg, err := LoadFromFile(configPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.checkCfg != nil {
				tt.checkCfg(t, cfg)
			}
		})
	}
}

// TestNestedConfigValidation tests validation for nested config
func TestNestedConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid nested ollama config",
			cfg: &Config{
				Warm: WarmConfig{
					Provider: ProviderOllama,
					Model:    "nomic-embed-text",
					BaseURL:  "http://localhost:11434",
				},
				Search: SearchConfig{
					Provider: ProviderOllama,
					Model:    "nomic-embed-text",
					BaseURL:  "http://localhost:11434",
				},
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr: false,
		},
		{
			name: "valid nested huggingface config",
			cfg: &Config{
				Warm: WarmConfig{
					Provider: ProviderHuggingFace,
					Model:    "sentence-transformers/all-MiniLM-L6-v2",
					Token:    "hf-token",
				},
				Search: SearchConfig{
					Provider: ProviderHuggingFace,
					Model:    "sentence-transformers/all-MiniLM-L6-v2",
					Token:    "hf-token",
				},
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr: false,
		},
		{
			name: "invalid nested provider",
			cfg: &Config{
				Warm: WarmConfig{
					Provider: "invalid",
					Model:    "test",
				},
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "invalid warm.provider",
		},
		{
			name: "missing model for nested huggingface",
			cfg: &Config{
				Warm: WarmConfig{
					Provider: ProviderHuggingFace,
					Model:    "",
				},
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "warm.model is required when warm.provider is huggingface",
		},
		{
			name: "missing model for nested ollama",
			cfg: &Config{
				Warm: WarmConfig{
					Provider: ProviderOllama,
					Model:    "",
					BaseURL:  "http://localhost:11434",
				},
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "warm.model is required when warm.provider is ollama",
		},
		{
			name: "missing base_url for nested ollama",
			cfg: &Config{
				Warm: WarmConfig{
					Provider: ProviderOllama,
					Model:    "nomic-embed-text",
					BaseURL:  "",
				},
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "warm.base_url is required when warm.provider is ollama",
		},
		{
			name: "missing model for search nested huggingface",
			cfg: &Config{
				Warm: WarmConfig{
					Provider: ProviderHuggingFace,
					Model:    "test-model",
				},
				Search: SearchConfig{
					Provider: ProviderHuggingFace,
					Model:    "",
				},
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "search.model is required when search.provider is huggingface",
		},
		{
			name: "missing base_url for search nested ollama",
			cfg: &Config{
				Warm: WarmConfig{
					Provider: ProviderOllama,
					Model:    "test-model",
					BaseURL:  "http://localhost:11434",
				},
				Search: SearchConfig{
					Provider: ProviderOllama,
					Model:    "test-model",
					BaseURL:  "",
				},
				ChunkSize:        512,
				ChunkOverlap:     100,
				MaxContextChunks: 10,
			},
			wantErr:     true,
			errContains: "search.base_url is required when search.provider is ollama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
				} else if !contains(err.Error(), tt.errContains) {
					t.Errorf("Error = %q, should contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
