package healthcheck

import (
	"os"
	"testing"

	"github.com/l3aro/go-context-query/internal/config"
)

func TestCheckWithNilConfig(t *testing.T) {
	_, err := Check(nil, "", "")
	if err == nil {
		t.Error("Expected error for nil config, got nil")
	}
}

func TestCheckMarksSearchAsInherited(t *testing.T) {
	cfg := &config.Config{
		Warm: config.WarmConfig{
			Provider: config.ProviderHuggingFace,
			Model:    "google/embeddinggemma-300m",
		},
		ChunkSize:        512,
		ChunkOverlap:     100,
		MaxContextChunks: 10,
	}
	// Search fields are empty - should be marked as inherited

	result, err := Check(cfg, "", "")
	if err != nil {
		t.Fatalf("Check() failed: %v", err)
	}

	if result.SearchModel.Status != "inherited" {
		t.Errorf("SearchModel.Status = %q, want %q", result.SearchModel.Status, "inherited")
	}
}

func TestCheckWithSeparateSearchModel(t *testing.T) {
	cfg := &config.Config{
		Warm: config.WarmConfig{
			Provider: config.ProviderHuggingFace,
			Model:    "google/embeddinggemma-300m",
		},
		Search: config.SearchConfig{
			Provider: config.ProviderOllama,
			Model:    "bge-m3",
			BaseURL:  "http://localhost:11434",
		},
		ChunkSize:        512,
		ChunkOverlap:     100,
		MaxContextChunks: 10,
	}

	result, err := Check(cfg, "", "")
	if err != nil {
		t.Fatalf("Check() failed: %v", err)
	}

	if result.SearchModel.Status == "inherited" {
		t.Error("SearchModel.Status should not be inherited when search provider is explicitly set")
	}
}

func TestScopeFromPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	globalPath := ""
	if home != "" {
		globalPath = home + "/.gcq/config.yaml"
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"empty path", "", ""},
		// Global config is no longer supported - all paths return "project"
		{"global path", globalPath, "project"},
		{"project path", "/project/.gcq/config.yaml", "project"},
		{"relative project path", ".gcq/config.yaml", "project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scopeFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("scopeFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsSearchExplicitlyConfigured(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected bool
	}{
		{
			name:     "empty config",
			cfg:      &config.Config{},
			expected: false,
		},
		{
			name: "only warm settings",
			cfg: &config.Config{
				Warm: config.WarmConfig{
					Provider: config.ProviderHuggingFace,
					Model:    "test-model",
				},
			},
			expected: false,
		},
		{
			name: "search provider set",
			cfg: &config.Config{
				Warm: config.WarmConfig{
					Provider: config.ProviderHuggingFace,
				},
				Search: config.SearchConfig{
					Provider: config.ProviderOllama,
				},
			},
			expected: true,
		},
		{
			name: "search model set",
			cfg: &config.Config{
				Warm: config.WarmConfig{
					Provider: config.ProviderHuggingFace,
				},
				Search: config.SearchConfig{
					Model: "test-model",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSearchExplicitlyConfigured(tt.cfg)
			if result != tt.expected {
				t.Errorf("isSearchExplicitlyConfigured() = %v, want %v", result, tt.expected)
			}
		})
	}
}
