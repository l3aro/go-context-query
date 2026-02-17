// Package commands provides the CLI commands for the go-context-query tool.
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/l3aro/go-context-query/pkg/cfg"
	"github.com/spf13/cobra"
)

// cfgCmd represents the cfg command
var cfgCmd = &cobra.Command{
	Use:   "cfg <file> <function>",
	Short: "Extract control flow graph for a function",
	Long: `Extracts the Control Flow Graph (CFG) for a specific function in a Python file.
Outputs JSON with blocks, edges, and cyclomatic complexity.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		functionName := args[1]

		// Check if file exists
		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("stat file: %w", err)
		}

		if info.IsDir() {
			return fmt.Errorf("path is a directory, expected a file: %s", filePath)
		}

		// Check if file is a Python file
		if !isPythonFile(filePath) {
			return fmt.Errorf("unsupported file type: %s (only .py files supported)", filePath)
		}

		// Extract CFG for the function
		cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
		if err != nil {
			// Check if it's a "function not found" error
			if isFunctionNotFoundError(err) {
				// Try to find similar function names for helpful error message
				suggestions := findSimilarFunctions(filePath, functionName)
				if len(suggestions) > 0 {
					return fmt.Errorf("function %q not found in %s\nDid you mean: %s?", functionName, filePath, suggestions[0])
				}
				return fmt.Errorf("function %q not found in %s", functionName, filePath)
			}
			return fmt.Errorf("extracting CFG: %w", err)
		}

		// Check for ambiguous function names (multiple matches)
		if cmd.Flags().Changed("all") {
			all, _ := cmd.Flags().GetBool("all")
			if all {
				// Could implement finding all functions with same name
				// For now, just warn if there are multiple
			}
		}

		// Get JSON flag
		jsonOutput, _ := cmd.Flags().GetBool("json")

		if jsonOutput {
			data, err := json.MarshalIndent(cfgInfo, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			printCFGInfo(cfgInfo)
		}

		return nil
	},
}

// isPythonFile checks if the file has a .py extension.
func isPythonFile(filePath string) bool {
	return strings.HasSuffix(filePath, ".py")
}

// isFunctionNotFoundError checks if the error indicates function not found.
func isFunctionNotFoundError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "not found")
}

// findSimilarFunctions finds functions with similar names (simple prefix/contains match).
func findSimilarFunctions(filePath, funcName string) []string {
	// For now, return empty - could be enhanced to parse file and find all functions
	return []string{}
}

// printCFGInfo prints CFG information in human-readable format.
func printCFGInfo(info *cfg.CFGInfo) {
	fmt.Printf("=== CFG for function: %s ===\n", info.FunctionName)
	fmt.Printf("Cyclomatic Complexity: %d\n", info.CyclomaticComplexity)
	fmt.Printf("Entry Block: %s\n", info.EntryBlockID)
	fmt.Printf("Exit Blocks: %v\n", info.ExitBlockIDs)
	fmt.Printf("\nBlocks (%d):\n", len(info.Blocks))
	for id, block := range info.Blocks {
		fmt.Printf("  %s (%s, lines %d-%d)\n", id, block.Type, block.StartLine, block.EndLine)
		for _, stmt := range block.Statements {
			fmt.Printf("    %s\n", stmt)
		}
	}

	fmt.Printf("\nEdges (%d):\n", len(info.Edges))
	for _, edge := range info.Edges {
		fmt.Printf("  %s --%s--> %s\n", edge.SourceID, edge.EdgeType, edge.TargetID)
	}
}

func init() {
	cfgCmd.Flags().BoolP("json", "j", false, "Output as JSON")
	cfgCmd.Flags().Bool("all", false, "Show all matches if function name is ambiguous")
	RootCmd.AddCommand(cfgCmd)
}
