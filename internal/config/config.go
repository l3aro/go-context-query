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

// WarmConfig holds configuration for the warm (indexing) provider
type WarmConfig struct {
	Provider ProviderType `yaml:"provider" env:"PROVIDER"`
	Model    string       `yaml:"model" env:"MODEL"`
	BaseURL  string       `yaml:"base_url" env:"BASE_URL"`
	Token    string       `yaml:"token" env:"TOKEN"`
}

// SearchConfig holds configuration for the search provider
type SearchConfig struct {
	Provider ProviderType `yaml:"provider" env:"PROVIDER"`
	Model    string       `yaml:"model" env:"MODEL"`
	BaseURL  string       `yaml:"base_url" env:"BASE_URL"`
	Token    string       `yaml:"token" env:"TOKEN"`
}

// Config holds all configuration for go-context-query
type Config struct {
	// Warm provider configuration (indexing)
	Warm WarmConfig `yaml:"warm"`

	// Search provider configuration
	Search SearchConfig `yaml:"search"`

	// Legacy fallback: if Warm/Search not set, these are used
	Provider ProviderType `yaml:"provider,omitempty" env:"GCQ_PROVIDER"`
	HFModel  string       `yaml:"hf_model,omitempty" env:"GCQ_HF_MODEL"`
	HFToken  string       `yaml:"hf_token,omitempty" env:"GCQ_HF_TOKEN"`

	OllamaModel   string `yaml:"ollama_model,omitempty" env:"GCQ_OLLAMA_MODEL"`
	OllamaBaseURL string `yaml:"ollama_base_url,omitempty" env:"GCQ_OLLAMA_BASE_URL"`
	OllamaAPIKey  string `yaml:"ollama_api_key,omitempty" env:"GCQ_OLLAMA_API_KEY"`

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
		Warm:                WarmConfig{},
		Search:              SearchConfig{},
		Provider:            "",
		HFModel:             "",
		HFToken:             "",
		OllamaModel:         "",
		OllamaBaseURL:       "",
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
		cfg.Warm.Provider = ProviderType(v)
	}
	if v := os.Getenv("GCQ_SEARCH_PROVIDER"); v != "" {
		cfg.Search.Provider = ProviderType(v)
	}
	if v := os.Getenv("GCQ_HF_MODEL"); v != "" {
		cfg.HFModel = v
	}
	if v := os.Getenv("GCQ_HF_TOKEN"); v != "" {
		cfg.HFToken = v
	}
	if v := os.Getenv("GCQ_WARM_HF_MODEL"); v != "" {
		cfg.Warm.Model = v
	}
	if v := os.Getenv("GCQ_WARM_HF_TOKEN"); v != "" {
		cfg.Warm.Token = v
	}
	if v := os.Getenv("GCQ_SEARCH_HF_MODEL"); v != "" {
		cfg.Search.Model = v
	}
	if v := os.Getenv("GCQ_SEARCH_HF_TOKEN"); v != "" {
		cfg.Search.Token = v
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
		cfg.Warm.Model = v
	}
	if v := os.Getenv("GCQ_WARM_OLLAMA_BASE_URL"); v != "" {
		cfg.Warm.BaseURL = v
	}
	if v := os.Getenv("GCQ_WARM_OLLAMA_API_KEY"); v != "" {
		cfg.Warm.Token = v
	}
	if v := os.Getenv("GCQ_SEARCH_OLLAMA_MODEL"); v != "" {
		cfg.Search.Model = v
	}
	if v := os.Getenv("GCQ_SEARCH_OLLAMA_BASE_URL"); v != "" {
		cfg.Search.BaseURL = v
	}
	if v := os.Getenv("GCQ_SEARCH_OLLAMA_API_KEY"); v != "" {
		cfg.Search.Token = v
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
	isDualMode := c.Warm.Provider != "" || c.Search.Provider != ""

	if isDualMode {
		if err := c.validateDualProviderMode(); err != nil {
			return err
		}
	} else {
		if err := c.validateSingleProviderMode(); err != nil {
			return err
		}
	}

	if c.ThresholdSimilarity < 0 || c.ThresholdSimilarity > 1 {
		return fmt.Errorf("threshold_similarity must be between 0 and 1")
	}
	if c.ThresholdMinScore < 0 || c.ThresholdMinScore > 1 {
		return fmt.Errorf("threshold_min_score must be between 0 and 1")
	}

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
	warmProvider := c.Warm.Provider
	if warmProvider == "" {
		warmProvider = c.Provider
	}

	searchProvider := c.Search.Provider
	if searchProvider == "" {
		searchProvider = c.Provider
	}

	if warmProvider != "" {
		switch warmProvider {
		case ProviderHuggingFace, ProviderOllama:
		default:
			return fmt.Errorf("invalid warm.provider: %s (must be 'huggingface' or 'ollama')", warmProvider)
		}

		if warmProvider == ProviderHuggingFace && c.Warm.Model == "" && c.HFModel == "" {
			return fmt.Errorf("warm.model is required when warm.provider is huggingface")
		}

		if warmProvider == ProviderOllama {
			if c.Warm.Model == "" && c.OllamaModel == "" {
				return fmt.Errorf("warm.model is required when warm.provider is ollama")
			}
			if c.Warm.BaseURL == "" && c.OllamaBaseURL == "" {
				return fmt.Errorf("warm.base_url is required when warm.provider is ollama")
			}
		}
	}

	if searchProvider != "" {
		switch searchProvider {
		case ProviderHuggingFace, ProviderOllama:
		default:
			return fmt.Errorf("invalid search.provider: %s (must be 'huggingface' or 'ollama')", searchProvider)
		}

		if searchProvider == ProviderHuggingFace && c.Search.Model == "" && c.HFModel == "" {
			return fmt.Errorf("search.model is required when search.provider is huggingface")
		}

		if searchProvider == ProviderOllama {
			if c.Search.Model == "" && c.OllamaModel == "" {
				return fmt.Errorf("search.model is required when search.provider is ollama")
			}
			if c.Search.BaseURL == "" && c.OllamaBaseURL == "" {
				return fmt.Errorf("search.base_url is required when search.provider is ollama")
			}
		}
	}

	return nil
}

func (c *Config) MigrateFromLegacy() {
	if c.Warm.Provider != "" && c.Search.Provider != "" {
		return
	}

	if c.Provider != "" {
		if c.Warm.Provider == "" {
			c.Warm.Provider = c.Provider
		}
		if c.Search.Provider == "" {
			c.Search.Provider = c.Provider
		}
	}

	c.migrateHuggingFaceSettings()
	c.migrateOllamaSettings()
}

func (c *Config) migrateHuggingFaceSettings() {
	if c.Warm.Model == "" && c.HFModel != "" {
		c.Warm.Model = c.HFModel
	}
	if c.Warm.Token == "" && c.HFToken != "" {
		c.Warm.Token = c.HFToken
	}
	if c.Search.Model == "" && c.HFModel != "" {
		c.Search.Model = c.HFModel
	}
	if c.Search.Token == "" && c.HFToken != "" {
		c.Search.Token = c.HFToken
	}
}

func (c *Config) migrateOllamaSettings() {
	if c.Warm.Model == "" && c.OllamaModel != "" {
		c.Warm.Model = c.OllamaModel
	}
	if c.Warm.BaseURL == "" && c.OllamaBaseURL != "" {
		c.Warm.BaseURL = c.OllamaBaseURL
	}
	if c.Warm.Token == "" && c.OllamaAPIKey != "" {
		c.Warm.Token = c.OllamaAPIKey
	}
	if c.Search.Model == "" && c.OllamaModel != "" {
		c.Search.Model = c.OllamaModel
	}
	if c.Search.BaseURL == "" && c.OllamaBaseURL != "" {
		c.Search.BaseURL = c.OllamaBaseURL
	}
	if c.Search.Token == "" && c.OllamaAPIKey != "" {
		c.Search.Token = c.OllamaAPIKey
	}
}

func (c *Config) EffectiveWarmProvider() ProviderType {
	if c.Warm.Provider != "" {
		return c.Warm.Provider
	}
	return c.Provider
}

func (c *Config) EffectiveSearchProvider() ProviderType {
	if c.Search.Provider != "" {
		return c.Search.Provider
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
