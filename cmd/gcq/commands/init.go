package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/l3aro/go-context-query/internal/config"
	"github.com/l3aro/go-context-query/internal/healthcheck"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gcq configuration interactively",
	Long: `Guides you through setting up gcq configuration step by step.
Creates a config file with warm model (for indexing) and search model settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func runInit() error {
	// === SECTION 1: Warm Model ===
	warmProvider := ""
	warmModel := ""
	warmBaseURL := ""
	warmAPIKey := ""

	// First, ask for provider
	var warmProviderChoice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Warm Model - Used for indexing/warming your codebase").
				Description("Select the embedding provider for indexing").
				Options(
					huh.NewOption("HuggingFace", "huggingface"),
					huh.NewOption("Ollama", "ollama"),
				).
				Value(&warmProviderChoice),
		),
	)
	err := form.Run()
	if err != nil {
		return fmt.Errorf("interactive prompt failed: %w", err)
	}

	warmProvider = warmProviderChoice

	// Provider-specific questions for warm model
	if warmProvider == "huggingface" {
		warmModel = "google/embeddinggemma-300m"
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("HuggingFace model for indexing").
					Placeholder("google/embeddinggemma-300m").
					Value(&warmModel),
			),
		)
		err = form.Run()
		if err != nil {
			return fmt.Errorf("interactive prompt failed: %w", err)
		}
	} else if warmProvider == "ollama" {
		warmBaseURL = "http://localhost:11434"
		warmModel = "embeddinggemma"

		form = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Ollama base URL").
					Placeholder("http://localhost:11434").
					Value(&warmBaseURL),
			),
		)
		err = form.Run()
		if err != nil {
			return fmt.Errorf("interactive prompt failed: %w", err)
		}

		form = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Ollama model for indexing").
					Placeholder("embeddinggemma").
					Value(&warmModel),
			),
		)
		err = form.Run()
		if err != nil {
			return fmt.Errorf("interactive prompt failed: %w", err)
		}

		form = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Ollama bearer API key (optional, press Enter to skip)").
					Placeholder("optional").
					Value(&warmAPIKey),
			),
		)
		err = form.Run()
		if err != nil {
			return fmt.Errorf("interactive prompt failed: %w", err)
		}
	}

	// === SECTION 2: Search Model ===
	var useSameModel bool
	form = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Search Model - Used for semantic search queries").
				Description("Use same model as warm model?").
				Affirmative("Yes, use warm model").
				Negative("No, configure separately").
				Value(&useSameModel),
		),
	)
	err = form.Run()
	if err != nil {
		return fmt.Errorf("interactive prompt failed: %w", err)
	}

	// Search-specific configuration (only if not using same model)
	searchProvider := ""
	searchModel := ""
	searchBaseURL := ""
	searchAPIKey := ""

	if !useSameModel {
		var searchProviderChoice string
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Search Model Provider").
					Description("Select the embedding provider for search").
					Options(
						huh.NewOption("HuggingFace", "huggingface"),
						huh.NewOption("Ollama", "ollama"),
					).
					Value(&searchProviderChoice),
			),
		)
		err = form.Run()
		if err != nil {
			return fmt.Errorf("interactive prompt failed: %w", err)
		}

		searchProvider = searchProviderChoice

		if searchProvider == "huggingface" {
			// Default to warm model if set
			defaultModel := warmModel
			if defaultModel == "" {
				defaultModel = "google/embeddinggemma-300m"
			}
			searchModel = defaultModel
			form = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("HuggingFace model for search").
						Placeholder(defaultModel).
						Value(&searchModel),
				),
			)
			err = form.Run()
			if err != nil {
				return fmt.Errorf("interactive prompt failed: %w", err)
			}

			var searchToken string
			form = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("HuggingFace API token (optional, press Enter to skip)").
						Placeholder("optional").
						Value(&searchToken),
				),
			)
			err = form.Run()
			if err != nil {
				return fmt.Errorf("interactive prompt failed: %w", err)
			}
			// Token will be used in config if provided
			_ = searchToken
		} else if searchProvider == "ollama" {
			// Default to warm settings if set
			defaultURL := warmBaseURL
			if defaultURL == "" {
				defaultURL = "http://localhost:11434"
			}
			defaultModel := warmModel
			if defaultModel == "" {
				defaultModel = "embeddinggemma"
			}

			searchBaseURL = defaultURL
			form = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Ollama base URL for search").
						Placeholder(defaultURL).
						Value(&searchBaseURL),
				),
			)
			err = form.Run()
			if err != nil {
				return fmt.Errorf("interactive prompt failed: %w", err)
			}

			searchModel = defaultModel
			form = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Ollama model for search").
						Placeholder(defaultModel).
						Value(&searchModel),
				),
			)
			err = form.Run()
			if err != nil {
				return fmt.Errorf("interactive prompt failed: %w", err)
			}

			form = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Ollama bearer API key (optional, press Enter to skip)").
						Placeholder("optional").
						Value(&searchAPIKey),
				),
			)
			err = form.Run()
			if err != nil {
				return fmt.Errorf("interactive prompt failed: %w", err)
			}
		}
	}

	// === SECTION 3: Config Location ===
	var saveLocationChoice string
	form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Save Configuration").
				Description("Where to save the configuration file?").
				Options(
					huh.NewOption("Global (~/.gcq/config.yaml)", "global"),
					huh.NewOption("Project (./.gcq/config.yaml)", "project"),
				).
				Value(&saveLocationChoice),
		),
	)
	err = form.Run()
	if err != nil {
		return fmt.Errorf("interactive prompt failed: %w", err)
	}

	// Determine save path
	var configPath string
	if saveLocationChoice == "global" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		configPath = filepath.Join(home, ".gcq", "config.yaml")
	} else {
		configPath = ".gcq/config.yaml"
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		var overwrite bool
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Config file exists").
					Description(fmt.Sprintf("Overwrite existing config at %s?", configPath)).
					Affirmative("Overwrite").
					Negative("Cancel").
					Value(&overwrite),
			),
		)
		err = form.Run()
		if err != nil {
			return fmt.Errorf("interactive prompt failed: %w", err)
		}
		if !overwrite {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// === Build config struct ===
	cfg := config.DefaultConfig()

	// Set warm provider and settings
	cfg.WarmProvider = config.ProviderType(warmProvider)
	if warmProvider == "huggingface" {
		cfg.WarmHFModel = warmModel
	} else if warmProvider == "ollama" {
		cfg.WarmOllamaModel = warmModel
		cfg.WarmOllamaBaseURL = warmBaseURL
		if warmAPIKey != "" {
			cfg.WarmOllamaAPIKey = warmAPIKey
		}
	}

	// Set search settings only if user selected "no" for "same as warm"
	if !useSameModel {
		cfg.SearchProvider = config.ProviderType(searchProvider)
		if searchProvider == "huggingface" {
			cfg.SearchHFModel = searchModel
		} else if searchProvider == "ollama" {
			cfg.SearchOllamaModel = searchModel
			cfg.SearchOllamaBaseURL = searchBaseURL
			if searchAPIKey != "" {
				cfg.SearchOllamaAPIKey = searchAPIKey
			}
		}
	}

	// Validate config before saving
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Show config preview
	fmt.Println("\n=== Configuration Preview ===")
	fmt.Printf("Config path: %s\n", configPath)
	fmt.Printf("Warm Provider: %s\n", cfg.WarmProvider)
	if cfg.WarmProvider == config.ProviderHuggingFace {
		fmt.Printf("Warm Model: %s\n", cfg.WarmHFModel)
	} else {
		fmt.Printf("Warm Model: %s\n", cfg.WarmOllamaModel)
		fmt.Printf("Warm URL: %s\n", cfg.WarmOllamaBaseURL)
	}

	if useSameModel {
		fmt.Println("Search Model: inherited from warm")
	} else {
		fmt.Printf("Search Provider: %s\n", cfg.SearchProvider)
		if cfg.SearchProvider == config.ProviderHuggingFace {
			fmt.Printf("Search Model: %s\n", cfg.SearchHFModel)
		} else {
			fmt.Printf("Search Model: %s\n", cfg.SearchOllamaModel)
			fmt.Printf("Search URL: %s\n", cfg.SearchOllamaBaseURL)
		}
	}
	fmt.Println("================================")

	// Save config
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("Configuration saved to: %s\n", configPath)

	// === SECTION 4: Health Check ===
	fmt.Println("\n=== Running Health Check ===")

	// Load the saved config for health check
	loadedCfg, err := config.LoadFromFile(configPath)
	if err != nil {
		return fmt.Errorf("loading saved config: %w", err)
	}

	// Run health check
	result, err := healthcheck.Check(loadedCfg, configPath, configPath)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Display results
	fmt.Printf("\nConfig Scope: %s\n", result.SavedScope)
	if result.SavedScope == "global" {
		fmt.Printf("Config Path: %s\n", configPath)
	} else {
		absPath, _ := filepath.Abs(configPath)
		fmt.Printf("Config Path: %s\n", absPath)
	}

	// Warm model status
	fmt.Printf("\nWarm Model Status: %s\n", result.WarmModel.Status)
	fmt.Printf("  Provider: %s\n", result.WarmModel.Provider)
	fmt.Printf("  Model: %s\n", result.WarmModel.Model)
	if result.WarmModel.URL != "" {
		fmt.Printf("  URL: %s\n", result.WarmModel.URL)
	}
	if result.WarmModel.Error != "" {
		fmt.Printf("  Error: %s\n", result.WarmModel.Error)
	}

	// Search model status
	fmt.Printf("\nSearch Model Status: %s\n", result.SearchModel.Status)
	if result.SearchModel.Status == "inherited" {
		fmt.Println("  (inherited from warm model)")
	} else {
		fmt.Printf("  Provider: %s\n", result.SearchModel.Provider)
		fmt.Printf("  Model: %s\n", result.SearchModel.Model)
		if result.SearchModel.URL != "" {
			fmt.Printf("  URL: %s\n", result.SearchModel.URL)
		}
		if result.SearchModel.Error != "" {
			fmt.Printf("  Error: %s\n", result.SearchModel.Error)
		}
	}

	fmt.Println("\n=== Initialization Complete ===")
	return nil
}

func init() {
	RootCmd.AddCommand(initCmd)
}
