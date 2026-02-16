package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/go-context-query/internal/daemon"
	"github.com/user/go-context-query/internal/scanner"
	"github.com/user/go-context-query/pkg/callgraph"
	"github.com/user/go-context-query/pkg/extractor"
)

// CallerInfo represents information about a caller
type CallerInfo struct {
	File   string `json:"file"`
	Func   string `json:"func"`
	Line   int    `json:"line,omitempty"`
	IsRoot bool   `json:"is_root"`
}

// ImpactOutput represents the output of the impact command
type ImpactOutput struct {
	TargetFunc string       `json:"target_func"`
	RootDir    string       `json:"root_dir"`
	Callers    []CallerInfo `json:"callers"`
	Count      int          `json:"count"`
}

// impactCmd represents the impact command
var impactCmd = &cobra.Command{
	Use:   "impact <function>",
	Short: "Find all callers of a function",
	Long: `Finds all functions that call the specified function.
This helps understand the impact of changing a function.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		funcName := args[0]

		// Check if daemon is available and use it
		if daemon.IsRunning() {
			return runImpactViaDaemon(funcName, cmd)
		}

		return runImpactLocally(funcName, cmd)
	},
}

func runImpactViaDaemon(funcName string, cmd *cobra.Command) error {
	// TODO: Implement daemon-based impact analysis
	// For now, fall back to local
	return runImpactLocally(funcName, cmd)
}

func runImpactLocally(funcName string, cmd *cobra.Command) error {
	// Find project root from current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	rootDir := findProjectRoot(cwd)

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

	// Find all callers of the function
	var callers []CallerInfo
	funcKey := funcName

	// Also check for qualified names like "ClassName.method"
	if strings.Contains(funcName, ".") {
		parts := strings.Split(funcName, ".")
		funcKey = parts[len(parts)-1]
	}

	// Search through call graph edges
	for _, edge := range callGraph.Edges {
		// Check if this edge calls our target function
		if edge.DestFunc == funcName || edge.DestFunc == funcKey ||
			strings.HasSuffix(edge.DestFunc, "."+funcKey) {
			callers = append(callers, CallerInfo{
				File:   edge.SourceFile,
				Func:   edge.SourceFunc,
				IsRoot: false,
			})
		}
	}

	// Check if the function itself is defined (it calls itself)
	for _, edge := range callGraph.Edges {
		if (edge.SourceFile == edge.DestFile) &&
			(edge.SourceFunc == funcName || edge.SourceFunc == funcKey) &&
			(edge.DestFunc == funcName || edge.DestFunc == funcKey) {
			// Found self-call
			exists := false
			for _, c := range callers {
				if c.Func == edge.SourceFunc && c.File == edge.SourceFile {
					exists = true
					break
				}
			}
			if !exists {
				callers = append(callers, CallerInfo{
					File:   edge.SourceFile,
					Func:   edge.SourceFunc,
					IsRoot: true,
				})
			}
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var uniqueCallers []CallerInfo
	for _, c := range callers {
		key := c.File + ":" + c.Func
		if !seen[key] {
			seen[key] = true
			uniqueCallers = append(uniqueCallers, c)
		}
	}

	output := ImpactOutput{
		TargetFunc: funcName,
		RootDir:    rootDir,
		Callers:    uniqueCallers,
		Count:      len(uniqueCallers),
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
		printImpact(output)
	}

	return nil
}

func printImpact(output ImpactOutput) {
	fmt.Printf("=== Impact Analysis: %s ===\n\n", output.TargetFunc)
	fmt.Printf("Root directory: %s\n", output.RootDir)
	fmt.Printf("Found %d caller(s)\n\n", output.Count)

	if len(output.Callers) > 0 {
		fmt.Println("Callers:")
		for _, c := range output.Callers {
			relPath, _ := filepath.Rel(output.RootDir, c.File)
			fmt.Printf("  %s:%s\n", relPath, c.Func)
		}
	} else {
		fmt.Println("No callers found.")
	}
}

func init() {
	impactCmd.Flags().BoolP("json", "j", false, "Output as JSON")
}
