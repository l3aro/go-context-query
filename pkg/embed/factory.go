package embed

import (
	"fmt"

	"github.com/l3aro/go-context-query/internal/config"
)

// NewProvider creates a new embedding provider based on the provider type.
// It returns the appropriate provider (Ollama or HuggingFace) based on the
// provider type string. Returns an error for unknown provider types.
func NewProvider(providerType config.ProviderType, cfg *Config) (Provider, error) {
	switch providerType {
	case config.ProviderOllama:
		return NewOllamaProvider(cfg)
	case config.ProviderHuggingFace:
		return NewHuggingFaceProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}
