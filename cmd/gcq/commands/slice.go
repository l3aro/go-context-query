// Package commands provides the CLI commands for the go-context-query tool.
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/l3aro/go-context-query/pkg/pdg"
	"github.com/spf13/cobra"
)

var sliceCmd = &cobra.Command{
	Use:   "slice <file> <function> --line N [--backward|--forward] [--var NAME] [--json]",
	Short: "Perform backward or forward slice analysis on a function",
	Long: `Perform slice analysis on a specific function to find data and control dependencies.

Backward slice: Find all lines that may affect the value at the target line.
Forward slice: Find all lines that may be affected by the value at the source line.

Supports Python, Go, TypeScript, Rust, Java, C, C++, Ruby, and PHP.`,
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

		lineNum, err := cmd.Flags().GetInt("line")
		if err != nil {
			return fmt.Errorf("getting line flag: %w", err)
		}
		if lineNum <= 0 {
			return fmt.Errorf("line number must be positive: %d", lineNum)
		}

		backward, _ := cmd.Flags().GetBool("backward")
		forward, _ := cmd.Flags().GetBool("forward")

		// Default to backward if neither specified
		if !backward && !forward {
			backward = true
		}

		var varFilter *string
		if cmd.Flags().Changed("var") {
			varName, _ := cmd.Flags().GetString("var")
			varFilter = &varName
		}

		pdgInfo, err := pdg.ExtractPDG(filePath, functionName)
		if err != nil {
			if isFunctionNotFoundError(err) {
				return fmt.Errorf("function %q not found in %s", functionName, filePath)
			}
			return fmt.Errorf("extracting PDG: %w", err)
		}

		var sliceLines []int
		if backward {
			sliceLines = pdg.BackwardSlice(pdgInfo, lineNum, varFilter)
		} else {
			sliceLines = pdg.ForwardSlice(pdgInfo, lineNum, varFilter)
		}

		if sliceLines == nil {
			sliceLines = []int{}
		}

		// Sort slice lines for consistent output
		slices.Sort(sliceLines)

		jsonOutput, _ := cmd.Flags().GetBool("json")

		if jsonOutput {
			output := struct {
				FunctionName string `json:"function_name"`
				Line         int    `json:"line"`
				Direction    string `json:"direction"`
				Variable     string `json:"variable,omitempty"`
				SliceLines   []int  `json:"slice_lines"`
			}{
				FunctionName: functionName,
				Line:         lineNum,
				Direction:    map[bool]string{true: "backward", false: "forward"}[backward],
				SliceLines:   sliceLines,
			}
			if varFilter != nil {
				output.Variable = *varFilter
			}

			data, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			printSliceInfo(functionName, lineNum, backward, varFilter, sliceLines, pdgInfo)
		}

		return nil
	},
}

func printSliceInfo(functionName string, lineNum int, backward bool, varFilter *string, sliceLines []int, pdgInfo *pdg.PDGInfo) {
	direction := "backward"
	if !backward {
		direction = "forward"
	}

	fmt.Printf("=== Slice for function: %s (line %d, %s) ===\n", functionName, lineNum, direction)

	if varFilter != nil {
		fmt.Printf("Variable filter: %s\n", *varFilter)
	}

	fmt.Printf("\nSlice lines (%d): ", len(sliceLines))
	if len(sliceLines) == 0 {
		fmt.Println("none")
		return
	}

	// Print as range representation
	fmt.Println(formatLineRanges(sliceLines))

	// If we have the PDG, show the source code with highlighted slice lines
	if pdgInfo != nil && pdgInfo.CFG != nil {
		fmt.Println("\n--- Source code with slice lines highlighted ---")
		printSourceWithHighlights(pdgInfo, sliceLines)
	}
}

func formatLineRanges(lines []int) string {
	if len(lines) == 0 {
		return "none"
	}

	var ranges []string
	start := lines[0]
	end := lines[0]

	for i := 1; i < len(lines); i++ {
		if lines[i] == end+1 {
			end = lines[i]
		} else {
			if start == end {
				ranges = append(ranges, fmt.Sprintf("%d", start))
			} else {
				ranges = append(ranges, fmt.Sprintf("%d-%d", start, end))
			}
			start = lines[i]
			end = lines[i]
		}
	}

	// Add the last range
	if start == end {
		ranges = append(ranges, fmt.Sprintf("%d", start))
	} else {
		ranges = append(ranges, fmt.Sprintf("%d-%d", start, end))
	}

	return strings.Join(ranges, ", ")
}

func printSourceWithHighlights(pdgInfo *pdg.PDGInfo, sliceLines []int) {
	// Create a set for O(1) lookup
	lineSet := make(map[int]bool)
	for _, line := range sliceLines {
		lineSet[line] = true
	}

	// Collect all statements from CFG blocks with their line numbers
	type lineContent struct {
		lineNum int
		content string
	}

	var allLines []lineContent

	// Get line ranges from PDG nodes
	for _, node := range pdgInfo.Nodes {
		for line := node.StartLine; line <= node.EndLine; line++ {
			// Try to get the statement from CFG blocks
			if block, ok := pdgInfo.CFG.Blocks[node.CFGBlockID]; ok {
				stmtIndex := line - block.StartLine
				if stmtIndex >= 0 && stmtIndex < len(block.Statements) {
					allLines = append(allLines, lineContent{
						lineNum: line,
						content: block.Statements[stmtIndex],
					})
				}
			}
		}
	}

	// Sort by line number and print
	slices.SortFunc(allLines, func(a, b lineContent) int {
		return a.lineNum - b.lineNum
	})

	// Deduplicate while preserving order
	seen := make(map[int]bool)
	var uniqueLines []lineContent
	for _, lc := range allLines {
		if !seen[lc.lineNum] {
			seen[lc.lineNum] = true
			uniqueLines = append(uniqueLines, lc)
		}
	}

	for _, lc := range uniqueLines {
		highlight := ""
		if lineSet[lc.lineNum] {
			highlight = " >>>"
		}
		fmt.Printf("%5d:%s %s\n", lc.lineNum, highlight, lc.content)
	}
}

func init() {
	sliceCmd.Flags().IntP("line", "l", 0, "Line number to slice from (required)")
	sliceCmd.Flags().BoolP("backward", "b", false, "Backward slice (default)")
	sliceCmd.Flags().BoolP("forward", "f", false, "Forward slice")
	sliceCmd.Flags().StringP("var", "v", "", "Variable name to filter (optional)")
	sliceCmd.Flags().BoolP("json", "j", false, "Output as JSON")

	_ = sliceCmd.MarkFlagRequired("line")

	RootCmd.AddCommand(sliceCmd)
}
