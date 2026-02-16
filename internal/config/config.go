package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProviderType represents the embedding provider
type ProviderType string

const (
	ProviderHuggingFace ProviderType = "huggingface"
	ProviderOllama      ProviderType = "ollama"
)

// Config holds all configuration for go-context-query
type Config struct {
	// Provider specifies which embedding provider to use
	Provider ProviderType `yaml:"provider" env:"GCQ_PROVIDER"`

	// HuggingFace specific settings
	HFModel string `yaml:"hf_model" env:"GCQ_HF_MODEL"`
	HFToken string `yaml:"hf_token" env:"GCQ_HF_TOKEN"`

	// Ollama specific settings
	OllamaModel   string `yaml:"ollama_model" env:"GCQ_OLLAMA_MODEL"`
	OllamaBaseURL string `yaml:"ollama_base_url" env:"GCQ_OLLAMA_BASE_URL"`
	OllamaAPIKey  string `yaml:"ollama_api_key" env:"GCQ_OLLAMA_API_KEY"`

	// Socket path for IPC communication
	SocketPath string `yaml:"socket_path" env:"GCQ_SOCKET_PATH"`

	// Thresholds for context gathering
	ThresholdSimilarity float64 `yaml:"threshold_similarity" env:"GCQ_THRESHOLD_SIMILARITY"`
	ThresholdMinScore   float64 `yaml:"threshold_min_score" env:"GCQ_THRESHOLD_MIN_SCORE"`

	// Context gathering settings
	MaxContextChunks int `yaml:"max_context_chunks" env:"GCQ_MAX_CONTEXT_CHUNKS"`
	ChunkOverlap     int `yaml:"chunk_overlap" env:"GCQ_CHUNK_OVERLAP"`
	ChunkSize        int `yaml:"chunk_size" env:"GCQ_CHUNK_SIZE"`

	// Logging
	Verbose bool `yaml:"verbose" env:"GCQ_VERBOSE"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Provider:            ProviderOllama,
		HFModel:             "sentence-transformers/all-MiniLM-L6-v2",
		OllamaModel:         "nomic-embed-text",
		OllamaBaseURL:       "http://localhost:11434",
		OllamaAPIKey:        "",
		SocketPath:          "/tmp/gcq.sock",
		ThresholdSimilarity: 0.7,
		ThresholdMinScore:   0.5,
		MaxContextChunks:    10,
		ChunkOverlap:        100,
		ChunkSize:           512,
		Verbose:             false,
	}
}

// configFilePath returns the default config file path
func configFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gcq/config.yaml"
	}
	return filepath.Join(home, ".gcq", "config.yaml")
}

// Load reads configuration from YAML file and applies environment variable overrides
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Try to load from YAML file
	configPath := configFilePath()
	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
		}
	}

	// Override with environment variables
	applyEnvOverrides(cfg)

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadFromFile reads configuration from a specific YAML file path
func LoadFromFile(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Load from specified file
	if data, err := os.ReadFile(path); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	} else if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Override with environment variables
	applyEnvOverrides(cfg)

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GCQ_PROVIDER"); v != "" {
		cfg.Provider = ProviderType(v)
	}
	if v := os.Getenv("GCQ_HF_MODEL"); v != "" {
		cfg.HFModel = v
	}
	if v := os.Getenv("GCQ_HF_TOKEN"); v != "" {
		cfg.HFToken = v
	}
	if v := os.Getenv("GCQ_OLLAMA_MODEL"); v != "" {
		cfg.OllamaModel = v
	}
	if v := os.Getenv("GCQ_OLLAMA_BASE_URL"); v != "" {
		cfg.OllamaBaseURL = v
	}
	if v := os.Getenv("GCQ_OLLAMA_API_KEY"); v != "" {
		cfg.OllamaAPIKey = v
	}
	if v := os.Getenv("GCQ_SOCKET_PATH"); v != "" {
		cfg.SocketPath = v
	}
	if v := os.Getenv("GCQ_THRESHOLD_SIMILARITY"); v != "" {
		if f := parseFloat(v); f > 0 {
			cfg.ThresholdSimilarity = f
		}
	}
	if v := os.Getenv("GCQ_THRESHOLD_MIN_SCORE"); v != "" {
		if f := parseFloat(v); f > 0 {
			cfg.ThresholdMinScore = f
		}
	}
	if v := os.Getenv("GCQ_MAX_CONTEXT_CHUNKS"); v != "" {
		if i := parseInt(v); i > 0 {
			cfg.MaxContextChunks = i
		}
	}
	if v := os.Getenv("GCQ_CHUNK_OVERLAP"); v != "" {
		if i := parseInt(v); i > 0 {
			cfg.ChunkOverlap = i
		}
	}
	if v := os.Getenv("GCQ_CHUNK_SIZE"); v != "" {
		if i := parseInt(v); i > 0 {
			cfg.ChunkSize = i
		}
	}
	if v := os.Getenv("GCQ_VERBOSE"); v != "" {
		cfg.Verbose = v == "true" || v == "1" || v == "yes"
	}
}

// Validate checks that the configuration has valid required fields
func (c *Config) Validate() error {
	// Validate provider
	switch c.Provider {
	case ProviderHuggingFace, ProviderOllama:
		// Valid
	default:
		return fmt.Errorf("invalid provider: %s (must be 'huggingface' or 'ollama')", c.Provider)
	}

	// Validate provider-specific settings
	if c.Provider == ProviderHuggingFace && c.HFModel == "" {
		return fmt.Errorf("hf_model is required when provider is huggingface")
	}

	if c.Provider == ProviderOllama {
		if c.OllamaModel == "" {
			return fmt.Errorf("ollama_model is required when provider is ollama")
		}
		if c.OllamaBaseURL == "" {
			return fmt.Errorf("ollama_base_url is required when provider is ollama")
		}
	}

	// Validate thresholds
	if c.ThresholdSimilarity < 0 || c.ThresholdSimilarity > 1 {
		return fmt.Errorf("threshold_similarity must be between 0 and 1")
	}
	if c.ThresholdMinScore < 0 || c.ThresholdMinScore > 1 {
		return fmt.Errorf("threshold_min_score must be between 0 and 1")
	}

	// Validate chunk settings
	if c.ChunkSize <= 0 {
		return fmt.Errorf("chunk_size must be positive")
	}
	if c.ChunkOverlap < 0 {
		return fmt.Errorf("chunk_overlap must be non-negative")
	}
	if c.MaxContextChunks <= 0 {
		return fmt.Errorf("max_context_chunks must be positive")
	}

	return nil
}

// parseFloat attempts to parse a string as float64
func parseFloat(s string) float64 {
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0
	}
	return f
}

// parseInt attempts to parse a string as int
func parseInt(s string) int {
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err != nil {
		return 0
	}
	return i
}
