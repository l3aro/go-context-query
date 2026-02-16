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
	"github.com/user/go-context-query/pkg/search"
	"github.com/user/go-context-query/pkg/semantic"
)

// SemanticOutput represents the output of the semantic command
type SemanticOutput struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	Stats   SemanticStats  `json:"stats"`
	RootDir string         `json:"root_dir,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	FilePath   string  `json:"file_path"`
	LineNumber int     `json:"line_number"`
	Name       string  `json:"name"`
	Signature  string  `json:"signature,omitempty"`
	Docstring  string  `json:"docstring,omitempty"`
	Type       string  `json:"type"`
	Score      float32 `json:"score"`
}

// SemanticStats represents statistics about the search
type SemanticStats struct {
	TotalResults int `json:"total_results"`
}

// semanticCmd represents the semantic command
var semanticCmd = &cobra.Command{
	Use:   "semantic <query>",
	Short: "Search the code index using semantic similarity",
	Long: `Performs semantic search over the indexed code to find
functions, methods, and classes that match the query.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		// Check if daemon is available and use it
		if daemon.IsRunning() {
			return runSemanticViaDaemon(query, cmd)
		}

		return runSemanticLocally(query, cmd)
	},
}

func runSemanticViaDaemon(query string, cmd *cobra.Command) error {
	// TODO: Implement daemon-based semantic search
	// For now, fall back to local
	return runSemanticLocally(query, cmd)
}

func runSemanticLocally(query string, cmd *cobra.Command) error {
	// Find project root from current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	rootDir := findProjectRoot(cwd)

	// Load the semantic index
	vecIndex, metadata, err := semantic.LoadIndex(rootDir)
	if err != nil {
		return fmt.Errorf("loading semantic index: %w\nRun 'gcq warm' first to build the index", err)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Get provider type and model
	providerType, _ := cmd.Flags().GetString("provider")
	modelName, _ := cmd.Flags().GetString("model")
	k, _ := cmd.Flags().GetInt("k")

	if k <= 0 {
		k = 10
	}

	if providerType == "" {
		providerType = string(cfg.Provider)
		if providerType == "" {
			providerType = "ollama"
		}
	}

	// Create the embedding provider
	var provider embed.Provider
	switch providerType {
	case "ollama":
		model := modelName
		if model == "" {
			model = metadata.Model
			if model == "" {
				model = cfg.OllamaModel
			}
			if model == "" {
				model = "nomic-embed-text"
			}
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
			model = metadata.Model
			if model == "" {
				model = cfg.HFModel
			}
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

	// Create searcher and perform search
	searcher := search.NewSearcher(provider, vecIndex)
	results, err := searcher.Search(query, k)
	if err != nil {
		return fmt.Errorf("performing search: %w", err)
	}

	// Convert results to our format
	var searchResults []SearchResult
	for _, r := range results {
		searchResults = append(searchResults, SearchResult{
			FilePath:   r.FilePath,
			LineNumber: r.LineNumber,
			Name:       r.Name,
			Signature:  r.Signature,
			Docstring:  r.Docstring,
			Type:       r.Type,
			Score:      r.Score,
		})
	}

	output := SemanticOutput{
		Query:   query,
		Results: searchResults,
		Stats:   SemanticStats{TotalResults: len(searchResults)},
		RootDir: rootDir,
	}

	// Output
	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		printSemantic(output)
	}

	return nil
}

func printSemantic(output SemanticOutput) {
	fmt.Printf("=== Semantic Search: %s ===\n\n", output.Query)

	if len(output.Results) == 0 {
		fmt.Println("No results found.")
		return
	}

	fmt.Printf("Found %d result(s):\n\n", len(output.Results))

	for i, r := range output.Results {
		relPath, _ := filepath.Rel(output.RootDir, r.FilePath)
		fmt.Printf("%d. %s:%d\n", i+1, relPath, r.LineNumber)
		fmt.Printf("   Name: %s (type: %s)\n", r.Name, r.Type)
		fmt.Printf("   Score: %.3f\n", r.Score)
		if r.Signature != "" {
			fmt.Printf("   Signature: %s\n", r.Signature)
		}
		if r.Docstring != "" {
			// Truncate docstring if too long
			doc := r.Docstring
			if len(doc) > 100 {
				doc = doc[:100] + "..."
			}
			fmt.Printf("   Doc: %s\n", doc)
		}
		fmt.Println()
	}
}

func init() {
	semanticCmd.Flags().BoolP("json", "j", false, "Output as JSON")
	semanticCmd.Flags().StringP("provider", "p", "", "Embedding provider (ollama or huggingface)")
	semanticCmd.Flags().StringP("model", "m", "", "Embedding model name")
	semanticCmd.Flags().IntP("k", "k", 10, "Number of results to return")
}
