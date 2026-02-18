package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/l3aro/go-context-query/internal/config"
	"github.com/l3aro/go-context-query/internal/daemon"
	"github.com/l3aro/go-context-query/pkg/dirty"
	"github.com/l3aro/go-context-query/pkg/embed"
	"github.com/l3aro/go-context-query/pkg/semantic"
	"github.com/spf13/cobra"
)

// WarmOutput represents the output of the warm command
type WarmOutput struct {
	RootDir       string   `json:"root_dir"`
	Success       bool     `json:"success"`
	UnitsCount    int      `json:"units_count"`
	Dimension     int      `json:"dimension"`
	Model         string   `json:"model"`
	CacheDir      string   `json:"cache_dir"`
	Message       string   `json:"message"`
	Languages     []string `json:"languages,omitempty"`
	ProcessedLang string   `json:"processed_lang,omitempty"`
}

// supportedLanguages returns the list of supported languages for indexing
func supportedLanguages() []string {
	return []string{
		"python",
		"go",
		"typescript",
		"javascript",
		"java",
		"rust",
		"c",
		"cpp",
		"ruby",
		"php",
		"swift",
		"kotlin",
		"csharp",
	}
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

		// Get language flag - empty means auto-detect (all languages)
		langFlag, _ := cmd.Flags().GetString("language")

		// Validate language if specified
		if langFlag != "" && langFlag != "auto" {
			validLang := false
			for _, lang := range supportedLanguages() {
				if langFlag == lang {
					validLang = true
					break
				}
			}
			if !validLang {
				return fmt.Errorf("unsupported language: %s (supported: %s, or 'auto' for all)", langFlag, supportedLanguages())
			}
		}

		// Get force flag
		forceFlag, _ := cmd.Flags().GetBool("force")

		// Load dirty tracker
		tracker := dirty.New(dirty.WithCacheDir(filepath.Join(".gcq", "cache")))
		if err := tracker.Load(); err != nil {
			return fmt.Errorf("loading dirty tracker: %w", err)
		}

		dirtyCount := tracker.Count()

		// Display dirty count if not forcing full rebuild
		if !forceFlag && dirtyCount > 0 {
			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput {
				fmt.Printf("{\"dirty_files\": %d, \"force\": false}\n", dirtyCount)
			} else {
				fmt.Printf("Dirty files detected: %d\n", dirtyCount)
				fmt.Println("Use --force to rebuild all files")
			}
		}

		// Check if daemon is available and use it
		if daemon.IsRunning() {
			return runWarmViaDaemon(path, cmd, langFlag, forceFlag, tracker)
		}

		return runWarmLocally(path, cmd, langFlag, forceFlag, tracker)
	},
}

func runWarmViaDaemon(path string, cmd *cobra.Command, langFlag string, forceFlag bool, tracker *dirty.Tracker) error {
	// TODO: Implement daemon-based semantic indexing
	// For now, fall back to local
	return runWarmLocally(path, cmd, langFlag, forceFlag, tracker)
}

func runWarmLocally(path string, cmd *cobra.Command, langFlag string, forceFlag bool, tracker *dirty.Tracker) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("stat path: %w", err)
	}

	rootDir, err := findProjectRoot(absPath)
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Get provider type - check warm-provider first, then fall back to provider for backward compat
	warmProviderFlag, _ := cmd.Flags().GetString("warm-provider")
	providerFlag, _ := cmd.Flags().GetString("provider")
	warmModelFlag, _ := cmd.Flags().GetString("warm-model")
	modelFlag, _ := cmd.Flags().GetString("model")

	// Determine provider: warm-provider > provider > config.WarmProvider > config.Provider > default
	providerType := warmProviderFlag
	if providerType == "" {
		providerType = providerFlag
	}
	if providerType == "" {
		providerType = string(cfg.WarmProvider)
	}
	if providerType == "" {
		providerType = string(cfg.Provider)
	}
	if providerType == "" {
		providerType = "ollama"
	}

	var provider embed.Provider

	switch providerType {
	case "ollama":
		model := warmModelFlag
		if model == "" {
			model = modelFlag
		}
		if model == "" {
			model = cfg.WarmOllamaModel
		}
		if model == "" {
			model = cfg.OllamaModel
		}
		if model == "" {
			model = "nomic-embed-text"
		}
		endpoint := cfg.WarmOllamaBaseURL
		if endpoint == "" {
			endpoint = cfg.OllamaBaseURL
		}
		apiKey := cfg.WarmOllamaAPIKey
		if apiKey == "" {
			apiKey = cfg.OllamaAPIKey
		}
		provider, err = embed.NewOllamaProvider(&embed.Config{
			Model:    model,
			Endpoint: endpoint,
			APIKey:   apiKey,
		})
		if err != nil {
			return fmt.Errorf("creating Ollama provider: %w", err)
		}
	case "huggingface":
		model := warmModelFlag
		if model == "" {
			model = modelFlag
		}
		if model == "" {
			model = cfg.WarmHFModel
		}
		if model == "" {
			model = cfg.HFModel
		}
		if model == "" {
			model = "sentence-transformers/all-MiniLM-L6-v2"
		}
		token := cfg.WarmHFToken
		if token == "" {
			token = cfg.HFToken
		}
		provider, err = embed.NewHuggingFaceProvider(&embed.Config{
			Model:  model,
			APIKey: token,
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
		processedLang := langFlag
		if processedLang == "" {
			processedLang = "auto"
		}
		output := WarmOutput{
			RootDir:       rootDir,
			Success:       true,
			Message:       "Index built but could not be loaded",
			ProcessedLang: processedLang,
			Languages:     supportedLanguages(),
		}
		printWarmOutput(output, cmd)
		return nil
	}

	var output WarmOutput
	if vecIndex != nil && vecIndex.Count() > 0 {
		processedLang := langFlag
		if processedLang == "" {
			processedLang = "auto"
		}
		output = WarmOutput{
			RootDir:       rootDir,
			Success:       true,
			UnitsCount:    vecIndex.Count(),
			Dimension:     vecIndex.Dimension(),
			Model:         metadata.WarmModel,
			CacheDir:      filepath.Join(rootDir, ".gcq", "cache", "semantic"),
			Message:       fmt.Sprintf("Indexed %d code units", vecIndex.Count()),
			ProcessedLang: processedLang,
			Languages:     supportedLanguages(),
		}
	} else {
		processedLang := langFlag
		if processedLang == "" {
			processedLang = "auto"
		}
		output = WarmOutput{
			RootDir:       rootDir,
			Success:       true,
			Message:       "No code units found to index",
			ProcessedLang: processedLang,
			Languages:     supportedLanguages(),
		}
	}

	printWarmOutput(output, cmd)

	// Clear dirty flags after successful warm
	tracker.ClearDirty(nil)
	if err := tracker.Save(); err != nil {
		return fmt.Errorf("saving dirty tracker: %w", err)
	}

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
		if output.ProcessedLang != "" {
			fmt.Printf("Processed language: %s\n", output.ProcessedLang)
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
	warmCmd.Flags().StringP("provider", "p", "", "Embedding provider for backward compatibility (use --warm-provider for separate warm provider)")
	warmCmd.Flags().StringP("model", "m", "", "Embedding model name for backward compatibility (use --warm-model for separate warm model)")
	warmCmd.Flags().String("warm-provider", "", "Embedding provider for indexing (ollama or huggingface). Overrides --provider")
	warmCmd.Flags().String("warm-model", "", "Embedding model name for indexing. Overrides --model")
	warmCmd.Flags().StringP("language", "l", "", "Language to index (auto-detects all by default). Supported: python, go, typescript, javascript, java, rust, c, cpp, ruby, php, swift, kotlin, csharp")
	warmCmd.Flags().BoolP("force", "f", false, "Force full rebuild, ignoring dirty tracking")
}
