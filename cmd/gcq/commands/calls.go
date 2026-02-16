package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/l3aro/go-context-query/internal/daemon"
	"github.com/l3aro/go-context-query/internal/scanner"
	"github.com/l3aro/go-context-query/pkg/callgraph"
	"github.com/l3aro/go-context-query/pkg/extractor"
	"github.com/l3aro/go-context-query/pkg/types"
)

// CallGraphOutput represents the output of the calls command
type CallGraphOutput struct {
	RootDir    string                `json:"root_dir"`
	Stats      CallGraphStats        `json:"stats"`
	Edges      []types.CallGraphEdge `json:"edges,omitempty"`
	Unresolved []UnresolvedCall      `json:"unresolved,omitempty"`
}

// CallGraphStats represents statistics about the call graph
type CallGraphStats struct {
	TotalEdges      int `json:"total_edges"`
	IntraFileEdges  int `json:"intra_file_edges"`
	CrossFileEdges  int `json:"cross_file_edges"`
	UnresolvedCalls int `json:"unresolved_calls"`
}

// UnresolvedCall represents an unresolved call
type UnresolvedCall struct {
	CallerFile string `json:"caller_file"`
	CallerFunc string `json:"caller_func"`
	CallName   string `json:"call_name"`
	Reason     string `json:"reason"`
}

// callsCmd represents the calls command
var callsCmd = &cobra.Command{
	Use:   "calls [path]",
	Short: "Build call graph for a project",
	Long: `Analyzes a project and builds a call graph showing function calls.
The call graph includes both intra-file and cross-file edges.`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		// Check if daemon is available and use it
		if daemon.IsRunning() {
			return runCallsViaDaemon(path, cmd)
		}

		return runCallsLocally(path, cmd)
	},
}

func runCallsViaDaemon(path string, cmd *cobra.Command) error {
	// TODO: Implement daemon-based call graph building
	// For now, fall back to local
	return runCallsLocally(path, cmd)
}

func runCallsLocally(path string, cmd *cobra.Command) error {
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

	// Find project root
	rootDir := findProjectRoot(absPath)
	if info.IsDir() {
		rootDir = absPath
	}

	// Scan project files
	sc := scanner.New(scanner.DefaultOptions())
	files, err := sc.Scan(rootDir)
	if err != nil {
		return fmt.Errorf("scanning directory: %w", err)
	}

	// Get supported file paths
	var supportedFiles []string
	registry := extractor.NewLanguageRegistry()
	for _, f := range files {
		if registry.IsSupported(f.FullPath) {
			supportedFiles = append(supportedFiles, f.FullPath)
		}
	}

	// Build call graph
	resolver := callgraph.NewResolver(rootDir)
	callGraph, err := resolver.ResolveCalls(supportedFiles)
	if err != nil {
		return fmt.Errorf("building call graph: %w", err)
	}

	// Build output
	stats := CallGraphStats{
		TotalEdges:      len(callGraph.Edges),
		IntraFileEdges:  len(callGraph.IntraFileEdges),
		CrossFileEdges:  len(callGraph.CrossFileEdges),
		UnresolvedCalls: len(callGraph.UnresolvedCalls),
	}

	var unresolved []UnresolvedCall
	for _, u := range callGraph.UnresolvedCalls {
		unresolved = append(unresolved, UnresolvedCall{
			CallerFile: u.CallerFile,
			CallerFunc: u.CallerFunc,
			CallName:   u.CallName,
			Reason:     u.Reason,
		})
	}

	output := CallGraphOutput{
		RootDir:    rootDir,
		Stats:      stats,
		Edges:      callGraph.Edges,
		Unresolved: unresolved,
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
		printCallGraph(output)
	}

	return nil
}

func printCallGraph(output CallGraphOutput) {
	fmt.Printf("=== Call Graph: %s ===\n\n", output.RootDir)

	fmt.Printf("Statistics:\n")
	fmt.Printf("  Total edges: %d\n", output.Stats.TotalEdges)
	fmt.Printf("  Intra-file edges: %d\n", output.Stats.IntraFileEdges)
	fmt.Printf("  Cross-file edges: %d\n", output.Stats.CrossFileEdges)
	fmt.Printf("  Unresolved calls: %d\n\n", output.Stats.UnresolvedCalls)

	if len(output.Edges) > 0 {
		fmt.Println("Edges:")
		for _, edge := range output.Edges {
			fmt.Printf("  %s:%s -> %s:%s\n",
				edge.SourceFile, edge.SourceFunc,
				edge.DestFile, edge.DestFunc)
		}
	}

	if len(output.Unresolved) > 0 {
		fmt.Println("\nUnresolved calls:")
		for _, u := range output.Unresolved {
			fmt.Printf("  %s:%s calls %s (%s)\n",
				u.CallerFile, u.CallerFunc, u.CallName, u.Reason)
		}
	}
}

func init() {
	callsCmd.Flags().BoolP("json", "j", false, "Output as JSON")
}
