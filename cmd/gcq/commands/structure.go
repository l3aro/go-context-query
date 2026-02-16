package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/l3aro/go-context-query/internal/scanner"
	"github.com/l3aro/go-context-query/pkg/extractor"
	"github.com/l3aro/go-context-query/pkg/types"
)

// StructureOutput represents the output structure for JSON
type StructureOutput struct {
	Path      string           `json:"path"`
	Language  string           `json:"language"`
	Functions []types.Function `json:"functions"`
	Classes   []types.Class    `json:"classes"`
	Imports   []types.Import   `json:"imports"`
}

// structureCmd represents the structure command
var structureCmd = &cobra.Command{
	Use:   "structure [path]",
	Short: "Show code structure (functions, classes, imports)",
	Long:  `Analyzes all supported files in the given path and shows their structure including functions, classes, and imports.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
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

		// Get JSON flag
		jsonOutput, _ := cmd.Flags().GetBool("json")

		if info.IsDir() {
			// Scan for files
			sc := scanner.New(scanner.DefaultOptions())
			files, err := sc.Scan(absPath)
			if err != nil {
				return fmt.Errorf("scanning directory: %w", err)
			}
			return outputStructureDir(absPath, files, jsonOutput)
		}
		return outputStructureFile(absPath, jsonOutput)
	},
}

func init() {
	structureCmd.Flags().BoolP("json", "j", false, "Output as JSON")
}

func outputStructureDir(root string, files []scanner.FileInfo, jsonOutput bool) error {
	// Filter supported files
	var supportedFiles []string
	for _, f := range files {
		if isSupported(f.FullPath) {
			supportedFiles = append(supportedFiles, f.FullPath)
		}
	}

	if len(supportedFiles) == 0 {
		if jsonOutput {
			data, _ := json.MarshalIndent([]StructureOutput{}, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Println("No supported files found")
		}
		return nil
	}

	var results []StructureOutput

	for _, filePath := range supportedFiles {
		info, err := extractor.ExtractFile(filePath)
		if err != nil {
			continue
		}

		results = append(results, StructureOutput{
			Path:      info.Path,
			Language:  detectLanguage(filePath),
			Functions: info.Functions,
			Classes:   info.Classes,
			Imports:   info.Imports,
		})
	}

	if jsonOutput {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		for _, r := range results {
			printStructureFile(r)
		}
	}

	return nil
}

func outputStructureFile(filePath string, jsonOutput bool) error {
	if !isSupported(filePath) {
		return fmt.Errorf("unsupported file type: %s", filePath)
	}

	info, err := extractor.ExtractFile(filePath)
	if err != nil {
		return fmt.Errorf("extracting file: %w", err)
	}

	output := StructureOutput{
		Path:      info.Path,
		Language:  detectLanguage(filePath),
		Functions: info.Functions,
		Classes:   info.Classes,
		Imports:   info.Imports,
	}

	if jsonOutput {
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		printStructureFile(output)
	}

	return nil
}

func printStructureFile(info StructureOutput) {
	fmt.Printf("=== %s (%s) ===\n", info.Path, info.Language)

	if len(info.Imports) > 0 {
		fmt.Println("\nImports:")
		for _, imp := range info.Imports {
			if imp.IsFrom {
				fmt.Printf("  from %s import %s\n", imp.Module, joinStrings(imp.Names))
			} else {
				fmt.Printf("  import %s\n", imp.Module)
			}
		}
	}

	if len(info.Classes) > 0 {
		fmt.Println("\nClasses:")
		for _, cls := range info.Classes {
			fmt.Printf("  class %s", cls.Name)
			if len(cls.Bases) > 0 {
				fmt.Printf("(%s)", joinStrings(cls.Bases))
			}
			fmt.Println()
			if cls.Docstring != "" {
				fmt.Printf("    %s\n", cls.Docstring)
			}
			for _, method := range cls.Methods {
				asyncPrefix := ""
				if method.IsAsync {
					asyncPrefix = "async "
				}
				fmt.Printf("    def %s%s(%s)", asyncPrefix, method.Name, method.Params)
				if method.ReturnType != "" {
					fmt.Printf(" -> %s", method.ReturnType)
				}
				fmt.Println()
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
			fmt.Printf("  def %s%s(%s)", asyncPrefix, fn.Name, fn.Params)
			if fn.ReturnType != "" {
				fmt.Printf(" -> %s", fn.ReturnType)
			}
			fmt.Println()
			if fn.Docstring != "" {
				fmt.Printf("    %s\n", fn.Docstring)
			}
		}
	}

	fmt.Println()
}

func isSupported(filePath string) bool {
	registry := extractor.NewLanguageRegistry()
	return registry.IsSupported(filePath)
}

func detectLanguage(filePath string) string {
	ext := ""
	if idx := len(filePath) - 1; idx >= 0 {
		for i := idx; i >= 0; i-- {
			if filePath[i] == '.' {
				ext = filePath[i:]
				break
			}
		}
	}

	langMap := map[string]string{
		".py":  "python",
		".pyw": "python",
		".pyi": "python",
		".go":  "go",
		".ts":  "typescript",
		".tsx": "typescript",
		".js":  "javascript",
		".jsx": "javascript",
	}

	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "unknown"
}

func joinStrings(s []string) string {
	if len(s) == 0 {
		return ""
	}
	result := s[0]
	for i := 1; i < len(s); i++ {
		result += ", " + s[i]
	}
	return result
}
