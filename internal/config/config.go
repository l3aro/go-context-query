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
	// Provider specifies which embedding provider to use (for backward compatibility)
	Provider ProviderType `yaml:"provider" env:"GCQ_PROVIDER"`

	// WarmProvider specifies which embedding provider to use for warming/indexing
	WarmProvider ProviderType `yaml:"warm_provider" env:"GCQ_WARM_PROVIDER"`

	// SearchProvider specifies which embedding provider to use for semantic search
	SearchProvider ProviderType `yaml:"search_provider" env:"GCQ_SEARCH_PROVIDER"`

	// HuggingFace specific settings (used by single provider mode)
	HFModel string `yaml:"hf_model" env:"GCQ_HF_MODEL"`
	HFToken string `yaml:"hf_token" env:"GCQ_HF_TOKEN"`

	// Warm provider HuggingFace settings
	WarmHFModel string `yaml:"warm_hf_model" env:"GCQ_WARM_HF_MODEL"`
	WarmHFToken string `yaml:"warm_hf_token" env:"GCQ_WARM_HF_TOKEN"`

	// Search provider HuggingFace settings
	SearchHFModel string `yaml:"search_hf_model" env:"GCQ_SEARCH_HF_MODEL"`
	SearchHFToken string `yaml:"search_hf_token" env:"GCQ_SEARCH_HF_TOKEN"`

	// Ollama specific settings (used by single provider mode)
	OllamaModel   string `yaml:"ollama_model" env:"GCQ_OLLAMA_MODEL"`
	OllamaBaseURL string `yaml:"ollama_base_url" env:"GCQ_OLLAMA_BASE_URL"`
	OllamaAPIKey  string `yaml:"ollama_api_key" env:"GCQ_OLLAMA_API_KEY"`

	// Warm provider Ollama settings
	WarmOllamaModel   string `yaml:"warm_ollama_model" env:"GCQ_WARM_OLLAMA_MODEL"`
	WarmOllamaBaseURL string `yaml:"warm_ollama_base_url" env:"GCQ_WARM_OLLAMA_BASE_URL"`
	WarmOllamaAPIKey  string `yaml:"warm_ollama_api_key" env:"GCQ_WARM_OLLAMA_API_KEY"`

	// Search provider Ollama settings
	SearchOllamaModel   string `yaml:"search_ollama_model" env:"GCQ_SEARCH_OLLAMA_MODEL"`
	SearchOllamaBaseURL string `yaml:"search_ollama_base_url" env:"GCQ_SEARCH_OLLAMA_BASE_URL"`
	SearchOllamaAPIKey  string `yaml:"search_ollama_api_key" env:"GCQ_SEARCH_OLLAMA_API_KEY"`

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
		WarmProvider:        "", // Empty by default - use Provider for backward compatibility
		SearchProvider:      "", // Empty by default - use Provider for backward compatibility
		HFModel:             "sentence-transformers/all-MiniLM-L6-v2",
		HFToken:             "",
		WarmHFModel:         "",
		WarmHFToken:         "",
		SearchHFModel:       "",
		SearchHFToken:       "",
		OllamaModel:         "nomic-embed-text",
		OllamaBaseURL:       "http://localhost:11434",
		OllamaAPIKey:        "",
		WarmOllamaModel:     "",
		WarmOllamaBaseURL:   "",
		WarmOllamaAPIKey:    "",
		SearchOllamaModel:   "",
		SearchOllamaBaseURL: "",
		SearchOllamaAPIKey:  "",
		SocketPath:          "/tmp/gcq.sock",
		ThresholdSimilarity: 0.7,
		ThresholdMinScore:   0.5,
		MaxContextChunks:    10,
		ChunkOverlap:        100,
		ChunkSize:           512,
		Verbose:             false,
	}
}

// globalConfigFilePath returns the global config file path (~/.gcq/config.yaml)
func globalConfigFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gcq/config.yaml"
	}
	return filepath.Join(home, ".gcq", "config.yaml")
}

// projectConfigFilePath returns the project-level config file path (./.gcq/config.yaml)
func projectConfigFilePath() string {
	return ".gcq/config.yaml"
}

// Load reads configuration with the following priority (highest to lowest):
// 1. Project-level config (./.gcq/config.yaml)
// 2. Environment variables
// 3. Global config (~/.gcq/config.yaml)
// 4. Defaults
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// 1. Load global config (~/.gcq/config.yaml)
	globalConfigPath := globalConfigFilePath()
	if data, err := os.ReadFile(globalConfigPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", globalConfigPath, err)
		}
	}

	// 2. Load project-level config (./.gcq/config.yaml) - overrides global
	projectConfigPath := projectConfigFilePath()
	if data, err := os.ReadFile(projectConfigPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", projectConfigPath, err)
		}
	}

	// 3. Override with environment variables
	applyEnvOverrides(cfg)

	cfg.MigrateFromLegacy()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadFromFile reads configuration from a specific YAML file path
func LoadFromFile(path string) (*Config, error) {
	cfg := DefaultConfig()

	if data, err := os.ReadFile(path); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	} else if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	applyEnvOverrides(cfg)

	cfg.MigrateFromLegacy()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the configuration to the specified YAML file path.
// It creates parent directories if they don't exist.
func (c *Config) Save(path string) error {
	// Create parent directories if they don't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to file with 0644 permissions
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to the config
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GCQ_PROVIDER"); v != "" {
		cfg.Provider = ProviderType(v)
	}
	if v := os.Getenv("GCQ_WARM_PROVIDER"); v != "" {
		cfg.WarmProvider = ProviderType(v)
	}
	if v := os.Getenv("GCQ_SEARCH_PROVIDER"); v != "" {
		cfg.SearchProvider = ProviderType(v)
	}
	if v := os.Getenv("GCQ_HF_MODEL"); v != "" {
		cfg.HFModel = v
	}
	if v := os.Getenv("GCQ_HF_TOKEN"); v != "" {
		cfg.HFToken = v
	}
	if v := os.Getenv("GCQ_WARM_HF_MODEL"); v != "" {
		cfg.WarmHFModel = v
	}
	if v := os.Getenv("GCQ_WARM_HF_TOKEN"); v != "" {
		cfg.WarmHFToken = v
	}
	if v := os.Getenv("GCQ_SEARCH_HF_MODEL"); v != "" {
		cfg.SearchHFModel = v
	}
	if v := os.Getenv("GCQ_SEARCH_HF_TOKEN"); v != "" {
		cfg.SearchHFToken = v
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
	if v := os.Getenv("GCQ_WARM_OLLAMA_MODEL"); v != "" {
		cfg.WarmOllamaModel = v
	}
	if v := os.Getenv("GCQ_WARM_OLLAMA_BASE_URL"); v != "" {
		cfg.WarmOllamaBaseURL = v
	}
	if v := os.Getenv("GCQ_WARM_OLLAMA_API_KEY"); v != "" {
		cfg.WarmOllamaAPIKey = v
	}
	if v := os.Getenv("GCQ_SEARCH_OLLAMA_MODEL"); v != "" {
		cfg.SearchOllamaModel = v
	}
	if v := os.Getenv("GCQ_SEARCH_OLLAMA_BASE_URL"); v != "" {
		cfg.SearchOllamaBaseURL = v
	}
	if v := os.Getenv("GCQ_SEARCH_OLLAMA_API_KEY"); v != "" {
		cfg.SearchOllamaAPIKey = v
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
	// Determine if we're in dual-provider mode
	isDualMode := c.WarmProvider != "" || c.SearchProvider != ""

	if isDualMode {
		// Validate dual-provider mode
		if err := c.validateDualProviderMode(); err != nil {
			return err
		}
	} else {
		// Validate single-provider mode (backward compatibility)
		if err := c.validateSingleProviderMode(); err != nil {
			return err
		}
	}

	// Validate thresholds (common to both modes)
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

// validateSingleProviderMode validates the legacy single-provider configuration
func (c *Config) validateSingleProviderMode() error {
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

	return nil
}

// validateDualProviderMode validates the dual-provider configuration
func (c *Config) validateDualProviderMode() error {
	// Use Provider as fallback if WarmProvider/SearchProvider not set
	warmProvider := c.WarmProvider
	if warmProvider == "" {
		warmProvider = c.Provider
	}

	searchProvider := c.SearchProvider
	if searchProvider == "" {
		searchProvider = c.Provider
	}

	// Validate warm provider
	if warmProvider != "" {
		switch warmProvider {
		case ProviderHuggingFace, ProviderOllama:
			// Valid
		default:
			return fmt.Errorf("invalid warm_provider: %s (must be 'huggingface' or 'ollama')", warmProvider)
		}

		if warmProvider == ProviderHuggingFace && c.WarmHFModel == "" && c.HFModel == "" {
			return fmt.Errorf("warm_hf_model is required when warm_provider is huggingface")
		}

		if warmProvider == ProviderOllama {
			if c.WarmOllamaModel == "" && c.OllamaModel == "" {
				return fmt.Errorf("warm_ollama_model is required when warm_provider is ollama")
			}
			if c.WarmOllamaBaseURL == "" && c.OllamaBaseURL == "" {
				return fmt.Errorf("warm_ollama_base_url is required when warm_provider is ollama")
			}
		}
	}

	// Validate search provider
	if searchProvider != "" {
		switch searchProvider {
		case ProviderHuggingFace, ProviderOllama:
			// Valid
		default:
			return fmt.Errorf("invalid search_provider: %s (must be 'huggingface' or 'ollama')", searchProvider)
		}

		if searchProvider == ProviderHuggingFace && c.SearchHFModel == "" && c.HFModel == "" {
			return fmt.Errorf("search_hf_model is required when search_provider is huggingface")
		}

		if searchProvider == ProviderOllama {
			if c.SearchOllamaModel == "" && c.OllamaModel == "" {
				return fmt.Errorf("search_ollama_model is required when search_provider is ollama")
			}
			if c.SearchOllamaBaseURL == "" && c.OllamaBaseURL == "" {
				return fmt.Errorf("search_ollama_base_url is required when search_provider is ollama")
			}
		}
	}

	return nil
}

// MigrateFromLegacy detects legacy single-provider config and populates
// dual-provider fields from it. This ensures old configs work unchanged
// while new dual-provider configs take precedence.
// Call this after loading config but before using provider settings.
func (c *Config) MigrateFromLegacy() {
	if c.WarmProvider != "" && c.SearchProvider != "" {
		return
	}

	if c.Provider != "" {
		if c.WarmProvider == "" {
			c.WarmProvider = c.Provider
		}
		if c.SearchProvider == "" {
			c.SearchProvider = c.Provider
		}
	}

	c.migrateHuggingFaceSettings()
	c.migrateOllamaSettings()
}

func (c *Config) migrateHuggingFaceSettings() {
	if c.WarmHFModel == "" && c.HFModel != "" {
		c.WarmHFModel = c.HFModel
	}
	if c.WarmHFToken == "" && c.HFToken != "" {
		c.WarmHFToken = c.HFToken
	}
	if c.SearchHFModel == "" && c.HFModel != "" {
		c.SearchHFModel = c.HFModel
	}
	if c.SearchHFToken == "" && c.HFToken != "" {
		c.SearchHFToken = c.HFToken
	}
}

func (c *Config) migrateOllamaSettings() {
	if c.WarmOllamaModel == "" && c.OllamaModel != "" {
		c.WarmOllamaModel = c.OllamaModel
	}
	if c.WarmOllamaBaseURL == "" && c.OllamaBaseURL != "" {
		c.WarmOllamaBaseURL = c.OllamaBaseURL
	}
	if c.WarmOllamaAPIKey == "" && c.OllamaAPIKey != "" {
		c.WarmOllamaAPIKey = c.OllamaAPIKey
	}
	if c.SearchOllamaModel == "" && c.OllamaModel != "" {
		c.SearchOllamaModel = c.OllamaModel
	}
	if c.SearchOllamaBaseURL == "" && c.OllamaBaseURL != "" {
		c.SearchOllamaBaseURL = c.OllamaBaseURL
	}
	if c.SearchOllamaAPIKey == "" && c.OllamaAPIKey != "" {
		c.SearchOllamaAPIKey = c.OllamaAPIKey
	}
}

// EffectiveWarmProvider returns the provider to use for warming/indexing.
// Falls back to the legacy Provider field if WarmProvider is not set.
func (c *Config) EffectiveWarmProvider() ProviderType {
	if c.WarmProvider != "" {
		return c.WarmProvider
	}
	return c.Provider
}

// EffectiveSearchProvider returns the provider to use for semantic search.
// Falls back to the legacy Provider field if SearchProvider is not set.
func (c *Config) EffectiveSearchProvider() ProviderType {
	if c.SearchProvider != "" {
		return c.SearchProvider
	}
	return c.Provider
}

// IsDualProviderMode returns true if warm and search use different providers
func (c *Config) IsDualProviderMode() bool {
	warm := c.EffectiveWarmProvider()
	search := c.EffectiveSearchProvider()
	return warm != search
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
