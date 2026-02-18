package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/l3aro/go-context-query/internal/config"
	"github.com/l3aro/go-context-query/internal/daemon"
	"github.com/l3aro/go-context-query/pkg/embed"
	"github.com/l3aro/go-context-query/pkg/search"
	"github.com/l3aro/go-context-query/pkg/semantic"
	"github.com/spf13/cobra"
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

	rootDir, err := findProjectRoot(cwd)
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Load the semantic index
	vecIndex, metadata, err := semantic.LoadIndex(rootDir)
	if err != nil {
		return fmt.Errorf("loading semantic index: %w\nRun 'gcq warm' first to build the index", err)
	}

	// Get CLI flags
	searchProviderFlag, _ := cmd.Flags().GetString("search-provider")
	providerFlag, _ := cmd.Flags().GetString("provider")
	searchModelFlag, _ := cmd.Flags().GetString("search-model")
	modelFlag, _ := cmd.Flags().GetString("model")
	k, _ := cmd.Flags().GetInt("k")

	if k <= 0 {
		k = 10
	}

	// Apply CLI flags to config for search provider
	if searchProviderFlag != "" {
		cfg.Search.Provider = config.ProviderType(searchProviderFlag)
	} else if providerFlag != "" {
		cfg.Search.Provider = config.ProviderType(providerFlag)
	}

	if searchModelFlag != "" {
		cfg.Search.Model = searchModelFlag
	} else if modelFlag != "" {
		cfg.Search.Model = modelFlag
	}

	// Create embedding service with config
	service, err := embed.NewEmbeddingService(cfg)
	if err != nil {
		return fmt.Errorf("creating embedding service: %w", err)
	}

	// Get search provider from service
	provider := service.SearchProvider()
	if provider == nil {
		return fmt.Errorf("search provider not initialized")
	}

	// Check dimension compatibility between index and search provider
	// This warns if dimensions differ but allows search to continue
	if metadata.Dimension > 0 {
		if err := embed.ValidateSearchCompatibility(metadata.Dimension, provider); err != nil {
			if errors.Is(err, embed.ErrDimensionMismatch) {
				// Dimensions differ but both can report - warn and continue
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				fmt.Fprintf(os.Stderr, "Search results may be degraded due to dimension mismatch.\n")
			} else {
				// Can't determine provider dimension - this is severe
				return fmt.Errorf("dimension compatibility check failed: %w", err)
			}
		}
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
		relPath := r.FilePath
		if filepath.IsAbs(r.FilePath) {
			var err error
			relPath, err = filepath.Rel(output.RootDir, r.FilePath)
			if err != nil {
				relPath = r.FilePath
			}
		}
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
	semanticCmd.Flags().StringP("provider", "p", "", "Embedding provider for backward compatibility (ollama or huggingface)")
	semanticCmd.Flags().StringP("model", "m", "", "Embedding model name for backward compatibility")
	semanticCmd.Flags().String("search-provider", "", "Search-specific embedding provider (ollama or huggingface)")
	semanticCmd.Flags().String("search-model", "", "Search-specific embedding model name")
	semanticCmd.Flags().IntP("k", "k", 10, "Number of results to return")
}
