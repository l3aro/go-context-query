package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/go-context-query/internal/config"
	"github.com/user/go-context-query/internal/daemon"
	"github.com/user/go-context-query/pkg/embed"
	"github.com/user/go-context-query/pkg/semantic"
)

// WarmOutput represents the output of the warm command
type WarmOutput struct {
	RootDir    string `json:"root_dir"`
	Success    bool   `json:"success"`
	UnitsCount int    `json:"units_count"`
	Dimension  int    `json:"dimension"`
	Model      string `json:"model"`
	CacheDir   string `json:"cache_dir"`
	Message    string `json:"message"`
}

// warmCmd represents the warm command
var warmCmd = &cobra.Command{
	Use:   "warm [path]",
	Short: "Build semantic index for a project",
	Long: `Scans the project, extracts code units, generates embeddings,
and builds a searchable semantic index.`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		// Check if daemon is available and use it
		if daemon.IsRunning() {
			return runWarmViaDaemon(path, cmd)
		}

		return runWarmLocally(path, cmd)
	},
}

func runWarmViaDaemon(path string, cmd *cobra.Command) error {
	// TODO: Implement daemon-based semantic indexing
	// For now, fall back to local
	return runWarmLocally(path, cmd)
}

func runWarmLocally(path string, cmd *cobra.Command) error {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat path: %w", err)
	}

	// Determine root dir
	rootDir := absPath
	if info.IsDir() {
		rootDir = absPath
	} else {
		rootDir = filepath.Dir(absPath)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Get provider type
	providerType, _ := cmd.Flags().GetString("provider")
	modelName, _ := cmd.Flags().GetString("model")

	if providerType == "" {
		providerType = string(cfg.Provider)
		if providerType == "" {
			providerType = "ollama"
		}
	}

	var provider embed.Provider

	switch providerType {
	case "ollama":
		model := modelName
		if model == "" {
			model = cfg.OllamaModel
		}
		if model == "" {
			model = "nomic-embed-text"
		}
		provider, err = embed.NewOllamaProvider(&embed.Config{
			Model:    model,
			Endpoint: cfg.OllamaBaseURL,
			APIKey:   cfg.OllamaAPIKey,
		})
		if err != nil {
			return fmt.Errorf("creating Ollama provider: %w", err)
		}
	case "huggingface":
		model := modelName
		if model == "" {
			model = cfg.HFModel
		}
		provider, err = embed.NewHuggingFaceProvider(&embed.Config{
			Model:  model,
			APIKey: cfg.HFToken,
		})
		if err != nil {
			return fmt.Errorf("creating HuggingFace provider: %w", err)
		}
	default:
		return fmt.Errorf("unknown provider: %s (use 'ollama' or 'huggingface')", providerType)
	}

	// Build the index
	err = semantic.BuildIndex(rootDir, provider)
	if err != nil {
		return fmt.Errorf("building index: %w", err)
	}

	// Try to load the index
	vecIndex, metadata, err := semantic.LoadIndex(rootDir)
	if err != nil {
		// Index was built but can't be loaded
		output := WarmOutput{
			RootDir: rootDir,
			Success: true,
			Message: "Index built but could not be loaded",
		}
		printWarmOutput(output, cmd)
		return nil
	}

	var output WarmOutput
	if vecIndex != nil && vecIndex.Count() > 0 {
		output = WarmOutput{
			RootDir:    rootDir,
			Success:    true,
			UnitsCount: vecIndex.Count(),
			Dimension:  vecIndex.Dimension(),
			Model:      metadata.Model,
			CacheDir:   filepath.Join(rootDir, ".gcq", "cache", "semantic"),
			Message:    fmt.Sprintf("Indexed %d code units", vecIndex.Count()),
		}
	} else {
		output = WarmOutput{
			RootDir: rootDir,
			Success: true,
			Message: "No code units found to index",
		}
	}

	printWarmOutput(output, cmd)
	return nil
}

func printWarmOutput(output WarmOutput, cmd *cobra.Command) {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling JSON: %v\n", err)
			return
		}
		fmt.Println(string(data))
		return
	}

	fmt.Printf("=== Semantic Index: %s ===\n\n", output.RootDir)

	if output.Success {
		fmt.Println("Status: Success")
		if output.UnitsCount > 0 {
			fmt.Printf("Code units indexed: %d\n", output.UnitsCount)
			fmt.Printf("Embedding dimension: %d\n", output.Dimension)
			fmt.Printf("Model: %s\n", output.Model)
			fmt.Printf("Cache directory: %s\n", output.CacheDir)
		}
	} else {
		fmt.Println("Status: Failed")
	}

	if output.Message != "" {
		fmt.Printf("\n%s\n", output.Message)
	}
}

func init() {
	warmCmd.Flags().BoolP("json", "j", false, "Output as JSON")
	warmCmd.Flags().StringP("provider", "p", "", "Embedding provider (ollama or huggingface)")
	warmCmd.Flags().StringP("model", "m", "", "Embedding model name")
}
