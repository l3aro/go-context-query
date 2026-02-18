package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/l3aro/go-context-query/internal/config"
)

// ModelStatus represents the health status of a single model configuration.
type ModelStatus struct {
	Provider string // "huggingface" or "ollama"
	Model    string
	URL      string // ollama endpoint
	Status   string // "ready", "downloading", "error", "inherited"
	Error    string
}

// HealthCheckResult contains the full health check output for display.
type HealthCheckResult struct {
	SavedPath      string
	SavedScope     string // "global" or "project"
	EffectivePath  string
	EffectiveScope string // "global" or "project"
	WarmModel      ModelStatus
	SearchModel    ModelStatus
}

// Check performs a health check against the given config.
// savedPath is where the user saved config (may be empty outside init).
// effectivePath is the config file actually in use (considering priority).
func Check(cfg *config.Config, savedPath string, effectivePath string) (*HealthCheckResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	result := &HealthCheckResult{
		SavedPath:      savedPath,
		SavedScope:     scopeFromPath(savedPath),
		EffectivePath:  effectivePath,
		EffectiveScope: scopeFromPath(effectivePath),
	}

	result.WarmModel = checkWarmModel(cfg)
	result.SearchModel = checkSearchModel(cfg, result.WarmModel)

	return result, nil
}

// scopeFromPath determines "global" or "project" scope from a config file path.
// Returns empty string if path is empty.
func scopeFromPath(path string) string {
	if path == "" {
		return ""
	}

	home, err := os.UserHomeDir()
	if err == nil {
		globalDir := filepath.Join(home, ".gcq")
		if strings.HasPrefix(path, globalDir) {
			return "global"
		}
	}

	return "project"
}

// checkWarmModel checks the status of the warm/indexing model.
func checkWarmModel(cfg *config.Config) ModelStatus {
	provider := cfg.EffectiveWarmProvider()

	switch provider {
	case config.ProviderOllama:
		return checkOllamaModel(cfg.Warm.Model, cfg.Warm.BaseURL, cfg.Warm.Token)
	case config.ProviderHuggingFace:
		return checkHuggingFaceModel(cfg.Warm.Model)
	default:
		return ModelStatus{
			Provider: string(provider),
			Status:   "error",
			Error:    fmt.Sprintf("unknown provider: %s", provider),
		}
	}
}

// checkSearchModel checks the status of the search model.
// If search-specific fields are not configured, marks as "inherited" from warm.
func checkSearchModel(cfg *config.Config, warmStatus ModelStatus) ModelStatus {
	if !isSearchExplicitlyConfigured(cfg) {
		return ModelStatus{
			Provider: warmStatus.Provider,
			Model:    warmStatus.Model,
			URL:      warmStatus.URL,
			Status:   "inherited",
		}
	}

	provider := cfg.EffectiveSearchProvider()

	if isSameAsWarm(cfg) {
		return ModelStatus{
			Provider: warmStatus.Provider,
			Model:    warmStatus.Model,
			URL:      warmStatus.URL,
			Status:   warmStatus.Status,
			Error:    warmStatus.Error,
		}
	}

	switch provider {
	case config.ProviderOllama:
		return checkOllamaModel(cfg.Search.Model, cfg.Search.BaseURL, cfg.Search.Token)
	case config.ProviderHuggingFace:
		return checkHuggingFaceModel(cfg.Search.Model)
	default:
		return ModelStatus{
			Provider: string(provider),
			Status:   "error",
			Error:    fmt.Sprintf("unknown provider: %s", provider),
		}
	}
}

// isSearchExplicitlyConfigured returns true if the user explicitly set search-specific config fields.
func isSearchExplicitlyConfigured(cfg *config.Config) bool {
	if cfg.Search.Provider != "" {
		return true
	}
	if cfg.Search.Model != "" || cfg.Search.BaseURL != "" {
		return true
	}
	return false
}

// isSameAsWarm returns true if the effective search config points to the same endpoint as warm.
func isSameAsWarm(cfg *config.Config) bool {
	if cfg.EffectiveWarmProvider() != cfg.EffectiveSearchProvider() {
		return false
	}
	switch cfg.EffectiveWarmProvider() {
	case config.ProviderOllama:
		return cfg.Warm.Model == cfg.Search.Model &&
			cfg.Warm.BaseURL == cfg.Search.BaseURL
	case config.ProviderHuggingFace:
		return cfg.Warm.Model == cfg.Search.Model
	}
	return false
}

// checkOllamaModel pings the Ollama base URL to verify the endpoint is reachable.
// It does NOT download or pull models. It supports bearer token authentication.
func checkOllamaModel(model, baseURL, apiKey string) ModelStatus {
	status := ModelStatus{
		Provider: "ollama",
		Model:    model,
		URL:      baseURL,
	}

	if baseURL == "" {
		status.Status = "error"
		status.Error = "ollama base URL is not configured"
		return status
	}

	// Ping Ollama root endpoint (GET /) - returns 200 if running
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		status.Status = "error"
		status.Error = fmt.Sprintf("invalid URL: %v", err)
		return status
	}

	// Add bearer token authentication if API key is provided
	if apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		status.Status = "error"
		status.Error = fmt.Sprintf("cannot reach ollama at %s: %v", baseURL, err)
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		status.Status = "ready"
	} else {
		status.Status = "error"
		status.Error = fmt.Sprintf("ollama returned status %d", resp.StatusCode)
	}

	return status
}

// checkHuggingFaceModel checks if a HuggingFace model is cached locally.
// It looks for model files in the HuggingFace cache directory.
// This avoids any network calls or API key requirements.
func checkHuggingFaceModel(model string) ModelStatus {
	status := ModelStatus{
		Provider: "huggingface",
		Model:    model,
	}

	if model == "" {
		status.Status = "error"
		status.Error = "huggingface model is not configured"
		return status
	}

	cacheDir := huggingFaceCacheDir(model)
	if cacheDir == "" {
		status.Status = "ready"
		return status
	}

	if info, err := os.Stat(cacheDir); err == nil && info.IsDir() {
		snapshotsDir := filepath.Join(cacheDir, "snapshots")
		if entries, err := os.ReadDir(snapshotsDir); err == nil && len(entries) > 0 {
			status.Status = "ready"
		} else {
			status.Status = "downloading"
		}
	} else {
		status.Status = "ready"
	}

	return status
}

// huggingFaceCacheDir returns the expected cache directory for a HuggingFace model.
// Returns empty string if home directory cannot be determined.
func huggingFaceCacheDir(model string) string {
	cacheBase := os.Getenv("HF_HUB_CACHE")
	if cacheBase == "" {
		hfHome := os.Getenv("HF_HOME")
		if hfHome != "" {
			cacheBase = filepath.Join(hfHome, "hub")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return ""
			}
			cacheBase = filepath.Join(home, ".cache", "huggingface", "hub")
		}
	}

	// HF cache format: models--<org>--<model> (slashes replaced with --)
	safeName := "models--" + strings.ReplaceAll(model, "/", "--")
	return filepath.Join(cacheBase, safeName)
}
