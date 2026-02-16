package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/l3aro/go-context-query/internal/scanner"
	"github.com/l3aro/go-context-query/pkg/callgraph"
	"github.com/l3aro/go-context-query/pkg/extractor"
	"github.com/l3aro/go-context-query/pkg/types"
	"github.com/spf13/cobra"
)

// ContextOutput represents the LLM context output
type ContextOutput struct {
	EntryPoint string             `json:"entry_point"`
	RootDir    string             `json:"root_dir"`
	Modules    []types.ModuleInfo `json:"modules"`
	CallGraph  types.CallGraph    `json:"call_graph,omitempty"`
	Summary    ContextSummary     `json:"summary"`
}

// ContextSummary provides a summary of the gathered context
type ContextSummary struct {
	TotalFiles     int `json:"total_files"`
	TotalFunctions int `json:"total_functions"`
	TotalClasses   int `json:"total_classes"`
	TotalImports   int `json:"total_imports"`
}

// contextCmd represents the context command
var contextCmd = &cobra.Command{
	Use:   "context <entry>",
	Short: "Get LLM-ready context from entry point",
	Long:  `Analyzes an entry point file and gathers its dependencies, imports, and call graph to provide a comprehensive context for LLM processing.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entryPath := args[0]

		// Get absolute path
		absPath, err := filepath.Abs(entryPath)
		if err != nil {
			return fmt.Errorf("getting absolute path: %w", err)
		}

		// Check if file exists
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("stat file: %w", err)
		}

		if info.IsDir() {
			return fmt.Errorf("path is a directory, expected a file: %s", entryPath)
		}

		// Find project root (go up until we find a reasonable project root)
		rootDir := findProjectRoot(absPath)

		// Scan project files
		sc := scanner.New(scanner.DefaultOptions())
		files, err := sc.Scan(rootDir)
		if err != nil {
			return fmt.Errorf("scanning directory: %w", err)
		}

		// Get all supported file paths
		var supportedFiles []string
		registry := extractor.NewLanguageRegistry()
		for _, f := range files {
			if registry.IsSupported(f.FullPath) {
				supportedFiles = append(supportedFiles, f.FullPath)
			}
		}

		// Build call graph resolver
		resolver := callgraph.NewResolver(rootDir, extractor.NewPythonExtractor())

		// Build index and call graph
		if err := resolver.BuildIndex(supportedFiles); err != nil {
			return fmt.Errorf("building function index: %w", err)
		}

		callGraph, err := resolver.ResolveCalls(supportedFiles)
		if err != nil {
			return fmt.Errorf("resolving call graph: %w", err)
		}

		// Get modules that are relevant to the entry point
		modules, err := getRelevantModules(absPath, supportedFiles, callGraph)
		if err != nil {
			return fmt.Errorf("getting relevant modules: %w", err)
		}

		// Build summary
		summary := ContextSummary{
			TotalFiles:     len(modules),
			TotalFunctions: 0,
			TotalClasses:   0,
			TotalImports:   0,
		}

		for _, m := range modules {
			summary.TotalFunctions += len(m.Functions)
			summary.TotalClasses += len(m.Classes)
			summary.TotalImports += len(m.Imports)
		}

		output := ContextOutput{
			EntryPoint: absPath,
			RootDir:    rootDir,
			Modules:    modules,
			CallGraph:  callGraph.ToTypesCallGraph(),
			Summary:    summary,
		}

		// Get JSON flag
		jsonOutput, _ := cmd.Flags().GetBool("json")

		if jsonOutput {
			data, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			printContext(output)
		}

		return nil
	},
}

func init() {
	contextCmd.Flags().BoolP("json", "j", false, "Output as JSON")
}

// findProjectRoot finds the project root directory
func findProjectRoot(filePath string) string {
	dir := filepath.Dir(filePath)

	// Look for common project markers
	markers := []string{"go.mod", "pyproject.toml", "package.json", "requirements.txt", ".git"}

	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}

		// Go up one level
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	// Default to the directory containing the file
	return filepath.Dir(filePath)
}

// getRelevantModules returns modules relevant to the entry point
func getRelevantModules(entryPath string, allFiles []string, callGraph *callgraph.CrossFileCallGraph) ([]types.ModuleInfo, error) {
	// Get entry point module
	entryModule, err := extractor.ExtractFile(entryPath)
	if err != nil {
		return nil, fmt.Errorf("extracting entry point: %w", err)
	}

	var modules []types.ModuleInfo
	modules = append(modules, *entryModule)

	// Track already added files
	addedFiles := map[string]bool{
		entryPath: true,
	}

	// Find files that are called from or call the entry point
	// Also follow imports recursively
	if err := collectRelatedModules(entryPath, allFiles, callGraph, &modules, addedFiles); err != nil {
		return nil, err
	}

	return modules, nil
}

// collectRelatedModules recursively collects modules related to the given file
func collectRelatedModules(filePath string, allFiles []string, callGraph *callgraph.CrossFileCallGraph, modules *[]types.ModuleInfo, addedFiles map[string]bool) error {
	// Get module info for this file
	moduleInfo, err := extractor.ExtractFile(filePath)
	if err != nil {
		return nil
	}

	// Find related files through call graph
	for _, edge := range callGraph.CrossFileEdges {
		if edge.SourceFile == filePath || edge.DestFile == filePath {
			// Add destination file if not already added
			if !addedFiles[edge.DestFile] {
				addedFiles[edge.DestFile] = true
				if module, err := extractor.ExtractFile(edge.DestFile); err == nil {
					*modules = append(*modules, *module)
					// Recursively collect
					if err := collectRelatedModules(edge.DestFile, allFiles, callGraph, modules, addedFiles); err != nil {
						return err
					}
				}
			}
		}
	}

	// Also follow imports
	for _, imp := range moduleInfo.Imports {
		// Try to find a file that matches this import
		for _, f := range allFiles {
			if addedFiles[f] {
				continue
			}

			// Check if file matches import
			fileName := filepath.Base(f)
			moduleName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

			if imp.IsFrom {
				for _, name := range imp.Names {
					if name == moduleName || strings.Contains(imp.Module, moduleName) {
						addedFiles[f] = true
						if module, err := extractor.ExtractFile(f); err == nil {
							*modules = append(*modules, *module)
							if err := collectRelatedModules(f, allFiles, callGraph, modules, addedFiles); err != nil {
								return err
							}
						}
						break
					}
				}
			} else {
				if strings.Contains(imp.Module, moduleName) {
					addedFiles[f] = true
					if module, err := extractor.ExtractFile(f); err == nil {
						*modules = append(*modules, *module)
						if err := collectRelatedModules(f, allFiles, callGraph, modules, addedFiles); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func printContext(output ContextOutput) {
	fmt.Printf("=== LLM Context from: %s ===\n\n", output.EntryPoint)
	fmt.Printf("Project Root: %s\n\n", output.RootDir)

	fmt.Printf("Summary:\n")
	fmt.Printf("  Files: %d\n", output.Summary.TotalFiles)
	fmt.Printf("  Functions: %d\n", output.Summary.TotalFunctions)
	fmt.Printf("  Classes: %d\n", output.Summary.TotalClasses)
	fmt.Printf("  Imports: %d\n\n", output.Summary.TotalImports)

	fmt.Println("Modules:")
	for _, module := range output.Modules {
		relPath, _ := filepath.Rel(output.RootDir, module.Path)
		fmt.Printf("\n--- %s ---\n", relPath)

		if module.Docstring != "" {
			fmt.Printf("\"\"\"\n%s\n\"\"\"\n\n", module.Docstring)
		}

		// Show imports
		if len(module.Imports) > 0 {
			fmt.Println("Imports:")
			for _, imp := range module.Imports {
				if imp.IsFrom {
					fmt.Printf("  from %s import %s\n", imp.Module, joinStrings(imp.Names))
				} else {
					fmt.Printf("  import %s\n", imp.Module)
				}
			}
			fmt.Println()
		}

		// Show classes
		for _, cls := range module.Classes {
			fmt.Printf("class %s", cls.Name)
			if len(cls.Bases) > 0 {
				fmt.Printf("(%s)", joinStrings(cls.Bases))
			}
			fmt.Println(":")

			for _, method := range cls.Methods {
				asyncPrefix := ""
				if method.IsAsync {
					asyncPrefix = "async "
				}
				retType := ""
				if method.ReturnType != "" {
					retType = " -> " + method.ReturnType
				}
				fmt.Printf("    %sdef %s(%s)%s\n", asyncPrefix, method.Name, method.Params, retType)
			}
			fmt.Println()
		}

		// Show functions
		for _, fn := range module.Functions {
			asyncPrefix := ""
			if fn.IsAsync {
				asyncPrefix = "async "
			}
			retType := ""
			if fn.ReturnType != "" {
				retType = " -> " + fn.ReturnType
			}
			fmt.Printf("def %s%s(%s)%s\n", asyncPrefix, fn.Name, fn.Params, retType)
		}
	}
}
