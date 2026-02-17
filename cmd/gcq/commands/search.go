package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/l3aro/go-context-query/pkg/search"
	"github.com/spf13/cobra"
)

// SearchOutput represents the output structure for JSON
type SearchOutput struct {
	Path        string   `json:"path"`
	Line        int      `json:"line"`
	Column      int      `json:"column"`
	Match       string   `json:"match"`
	LineContent string   `json:"line_content"`
	Context     []string `json:"context,omitempty"`
}

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search [pattern] [path]",
	Short: "Search for regex pattern in files",
	Long: `Searches for a regex pattern across all files in the given path.
The pattern is treated as a regular expression.

Examples:
  gcq search "func.*test" .
  gcq search --ext .go --context 2 "TODO" .
  gcq search --json "error" /path/to/project`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]
		path := "."
		if len(args) > 1 {
			path = args[1]
		}

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

		if !info.IsDir() {
			return fmt.Errorf("path must be a directory: %s", absPath)
		}

		// Get flags
		jsonOutput, _ := cmd.Flags().GetBool("json")
		contextLines, _ := cmd.Flags().GetInt("context")
		maxResults, _ := cmd.Flags().GetInt("max")
		extensions, _ := cmd.Flags().GetStringSlice("ext")

		// Create searcher
		searcher := search.NewTextSearcher(search.TextSearchOptions{
			Extensions:   extensions,
			ContextLines: contextLines,
			MaxResults:   maxResults,
		})

		// Perform search
		ctx := context.Background()
		matches, err := searcher.Search(ctx, pattern, absPath)
		if err != nil {
			return fmt.Errorf("searching: %w", err)
		}

		// Output results
		if jsonOutput {
			return outputSearchJSON(matches)
		}
		return outputSearchText(matches)
	},
}

func init() {
	searchCmd.Flags().StringP("pattern", "p", "", "Regex pattern to search for")
	searchCmd.Flags().IntP("context", "c", 0, "Number of context lines before and after match")
	searchCmd.Flags().StringSliceP("ext", "e", []string{}, "File extensions to search (can repeat)")
	searchCmd.Flags().IntP("max", "m", 0, "Maximum number of results (0 = unlimited)")
	searchCmd.Flags().BoolP("json", "j", false, "Output as JSON")
}

func outputSearchJSON(matches []search.TextMatch) error {
	var results []SearchOutput
	for _, m := range matches {
		output := SearchOutput{
			Path:        m.FilePath,
			Line:        m.LineNumber,
			Column:      m.Column,
			Match:       m.Match,
			LineContent: m.LineContent,
		}
		if len(m.ContextBefore) > 0 || len(m.ContextAfter) > 0 {
			output.Context = append(output.Context, m.ContextBefore...)
			output.Context = append(output.Context, m.LineContent)
			output.Context = append(output.Context, m.ContextAfter...)
		}
		results = append(results, output)
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func outputSearchText(matches []search.TextMatch) error {
	if len(matches) == 0 {
		fmt.Println("No matches found")
		return nil
	}

	// Group matches by file
	files := make(map[string][]search.TextMatch)
	for _, m := range matches {
		files[m.FilePath] = append(files[m.FilePath], m)
	}

	for filePath, fileMatches := range files {
		fmt.Printf("=== %s ===\n", filePath)
		for _, m := range fileMatches {
			fmt.Printf("  %d:%d: %s\n", m.LineNumber, m.Column, m.LineContent)
			if len(m.ContextBefore) > 0 {
				fmt.Println("    --- Context Before ---")
				for _, ctx := range m.ContextBefore {
					fmt.Printf("    %s\n", ctx)
				}
			}
			if len(m.ContextAfter) > 0 {
				fmt.Println("    --- Context After ---")
				for _, ctx := range m.ContextAfter {
					fmt.Printf("    %s\n", ctx)
				}
			}
		}
		fmt.Println()
	}

	return nil
}
