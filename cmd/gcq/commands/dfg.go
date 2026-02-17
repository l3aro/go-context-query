// Package commands provides the CLI commands for the go-context-query tool.
package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/l3aro/go-context-query/pkg/dfg"
	"github.com/spf13/cobra"
)

var dfgCmd = &cobra.Command{
	Use:   "dfg <file> <function>",
	Short: "Extract data flow graph for a function",
	Long: `Extracts the Data Flow Graph (DFG) for a specific function.
Supports Python, Go, TypeScript, Rust, Java, C, C++, Ruby, and PHP.
Outputs JSON with varRefs, dataflowEdges, and variables.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		functionName := args[1]

		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("stat file: %w", err)
		}

		if info.IsDir() {
			return fmt.Errorf("path is a directory, expected a file: %s", filePath)
		}

		dfgInfo, err := dfg.ExtractDFG(filePath, functionName)
		if err != nil {
			if isFunctionNotFoundError(err) {
				return fmt.Errorf("function %q not found in %s", functionName, filePath)
			}
			return fmt.Errorf("extracting DFG: %w", err)
		}

		jsonOutput, _ := cmd.Flags().GetBool("json")

		if jsonOutput {
			data, err := json.MarshalIndent(dfgInfo, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			printDFGInfo(dfgInfo)
		}

		return nil
	},
}

func printDFGInfo(info *dfg.DFGInfo) {
	fmt.Printf("=== DFG for function: %s ===\n", info.FunctionName)
	fmt.Printf("\nVariables (%d):\n", len(info.Variables))
	for name, refs := range info.Variables {
		fmt.Printf("  %s:\n", name)
		for _, ref := range refs {
			fmt.Printf("    - %s (line %d, col %d)\n", ref.RefType, ref.Line, ref.Column)
		}
	}

	fmt.Printf("\nVariable References (%d):\n", len(info.VarRefs))
	for _, ref := range info.VarRefs {
		fmt.Printf("  %s: %s (line %d, col %d)\n", ref.Name, ref.RefType, ref.Line, ref.Column)
	}

	fmt.Printf("\nData Flow Edges (%d):\n", len(info.DataflowEdges))
	for _, edge := range info.DataflowEdges {
		fmt.Printf("  %s: def(line %d) -> use(line %d)\n",
			edge.VarName, edge.DefRef.Line, edge.UseRef.Line)
	}
}

func init() {
	dfgCmd.Flags().BoolP("json", "j", false, "Output as JSON")
	RootCmd.AddCommand(dfgCmd)
}
