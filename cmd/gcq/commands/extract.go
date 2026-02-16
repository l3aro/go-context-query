package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/l3aro/go-context-query/pkg/extractor"
	"github.com/l3aro/go-context-query/pkg/types"
)

// extractCmd represents the extract command
var extractCmd = &cobra.Command{
	Use:   "extract <file>",
	Short: "Full file analysis",
	Long:  `Extracts complete module information including functions, classes, imports, and call graph from a single file.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// Check if file exists
		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("stat file: %w", err)
		}

		if info.IsDir() {
			return fmt.Errorf("path is a directory, expected a file: %s", filePath)
		}

		// Check if file type is supported
		registry := extractor.NewLanguageRegistry()
		if !registry.IsSupported(filePath) {
			return fmt.Errorf("unsupported file type: %s", filePath)
		}

		// Extract module info
		moduleInfo, err := extractor.ExtractFile(filePath)
		if err != nil {
			return fmt.Errorf("extracting file: %w", err)
		}

		// Get JSON flag
		jsonOutput, _ := cmd.Flags().GetBool("json")

		if jsonOutput {
			data, err := json.MarshalIndent(moduleInfo, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			printModuleInfo(moduleInfo)
		}

		return nil
	},
}

func init() {
	extractCmd.Flags().BoolP("json", "j", false, "Output as JSON")
}

func printModuleInfo(info *types.ModuleInfo) {
	fmt.Printf("=== Module: %s ===\n", info.Path)

	if info.Docstring != "" {
		fmt.Printf("\nDocstring:\n%s\n", info.Docstring)
	}

	if len(info.Imports) > 0 {
		fmt.Println("\nImports:")
		for _, imp := range info.Imports {
			if imp.IsFrom {
				fmt.Printf("  from %s import %s (line %d)\n", imp.Module, joinStrings(imp.Names), imp.LineNumber)
			} else {
				fmt.Printf("  import %s (line %d)\n", imp.Module, imp.LineNumber)
			}
		}
	}

	if len(info.Classes) > 0 {
		fmt.Println("\nClasses:")
		for _, cls := range info.Classes {
			fmt.Printf("  class %s (line %d)", cls.Name, cls.LineNumber)
			if len(cls.Bases) > 0 {
				fmt.Printf(" extends %s", joinStrings(cls.Bases))
			}
			fmt.Println()
			if cls.Docstring != "" {
				fmt.Printf("    %s\n", cls.Docstring)
			}
			if len(cls.Methods) > 0 {
				fmt.Println("    Methods:")
				for _, method := range cls.Methods {
					asyncPrefix := ""
					if method.IsAsync {
						asyncPrefix = "async "
					}
					fmt.Printf("      %sdef %s%s(%s)", asyncPrefix, method.Name, method.Params, method.ReturnType)
					fmt.Println()
					if method.Docstring != "" {
						fmt.Printf("        %s\n", method.Docstring)
					}
				}
			}
		}
	}

	if len(info.Functions) > 0 {
		fmt.Println("\nFunctions:")
		for _, fn := range info.Functions {
			asyncPrefix := ""
			if fn.IsAsync {
				asyncPrefix = "async "
			}
			fmt.Printf("  %sdef %s%s(%s)", asyncPrefix, fn.Name, fn.Params, fn.ReturnType)
			fmt.Printf(" (line %d)\n", fn.LineNumber)
			if fn.Docstring != "" {
				fmt.Printf("    %s\n", fn.Docstring)
			}
		}
	}

	if len(info.CallGraph.Edges) > 0 {
		fmt.Println("\nCall Graph Edges:")
		for _, edge := range info.CallGraph.Edges {
			fmt.Printf("  %s:%s -> %s:%s\n", edge.SourceFile, edge.SourceFunc, edge.DestFile, edge.DestFunc)
		}
	}

	fmt.Println()
}
