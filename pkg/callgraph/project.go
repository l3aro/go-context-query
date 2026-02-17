// Package callgraph provides cross-file project scanning functionality.
// It scans project directories to discover source files while respecting
// ignore patterns and language filters.
package callgraph

import (
	"fmt"
	"strings"

	"github.com/l3aro/go-context-query/internal/scanner"
)

// PythonExtensions contains the file extensions recognized as Python source files.
var PythonExtensions = map[string]bool{
	".py":  true,
	".pyi": true,
	".pyw": true,
}

// IsPythonFile checks if a file path has a Python extension.
func IsPythonFile(path string) bool {
	// Extract extension (handles .pyc, .pyo by checking against our map)
	ext := strings.ToLower(getExtension(path))
	return PythonExtensions[ext]
}

// getExtension extracts the file extension from a path.
func getExtension(path string) string {
	// Find the last dot
	lastDot := strings.LastIndex(path, ".")
	if lastDot == -1 || lastDot == len(path)-1 {
		return ""
	}
	return path[lastDot:]
}

// IsCompiledPython checks if a file is a compiled Python file that should be excluded.
func IsCompiledPython(path string) bool {
	ext := strings.ToLower(getExtension(path))
	return ext == ".pyc" || ext == ".pyo" || ext == ".pyd"
}

// IsInPycache checks if a path is inside a __pycache__ directory.
func IsInPycache(path string) bool {
	// Normalize path separators
	normalized := strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if part == "__pycache__" {
			return true
		}
	}
	return false
}

// IsTestFile checks if a file appears to be a test file.
// Test files typically start with "test_" or end with "_test.py".
func IsTestFile(path string) bool {
	// Get the filename - handle both Unix and Windows path separators
	filename := path
	if idx := strings.LastIndex(path, "/"); idx != -1 {
		filename = path[idx+1:]
	}
	if idx := strings.LastIndex(filename, "\\"); idx != -1 {
		filename = filename[idx+1:]
	}

	// Check for test patterns
	return strings.HasPrefix(filename, "test_") ||
		strings.HasSuffix(filename, "_test.py") ||
		strings.HasPrefix(filename, "conftest.py")
}

// ScanOptions configures the project scanning behavior.
type ScanOptions struct {
	// IncludeTestFiles if true, includes test files in the results
	IncludeTestFiles bool
	// ExcludePatterns additional patterns to exclude beyond scanner defaults
	ExcludePatterns []string
}

// DefaultScanOptions returns scan options with sensible defaults.
func DefaultScanOptions() ScanOptions {
	return ScanOptions{
		IncludeTestFiles: false,
		ExcludePatterns:  nil,
	}
}

// ScanProject scans a project directory and returns all Python source files.
// It respects ignore patterns from .gcqignore files and default exclusions.
//
// Parameters:
//   - root: The root directory to scan
//   - language: The language to filter for (currently only "python" is supported)
//
// Returns:
//   - []string: List of relative paths to Python files
//   - error: Any error encountered during scanning
func ScanProject(root string, language string) ([]string, error) {
	return ScanProjectWithOptions(root, language, DefaultScanOptions())
}

// ScanProjectWithOptions scans a project with custom options.
//
// Parameters:
//   - root: The root directory to scan
//   - language: The language to filter for (currently only "python" is supported)
//   - opts: Scan options for fine-tuned control
//
// Returns:
//   - []string: List of relative paths to Python files
//   - error: Any error encountered during scanning
func ScanProjectWithOptions(root string, language string, opts ScanOptions) ([]string, error) {
	// Currently only Python is supported
	if language != "python" && language != "" {
		return nil, fmt.Errorf("unsupported language: %s (only 'python' is supported)", language)
	}

	// Create scanner with default options
	scanOpts := scanner.DefaultOptions()

	// Add compiled Python files to exclusions
	// (already handled by extension filtering, but good to be explicit)
	scanOpts.DefaultExcludes = append(scanOpts.DefaultExcludes, "__pycache__")

	// Scan the project
	s := scanner.New(scanOpts)
	files, err := s.Scan(root)
	if err != nil {
		return nil, fmt.Errorf("scanning project: %w", err)
	}

	// Filter for Python files
	var pythonFiles []string
	for _, file := range files {
		// Skip compiled Python files
		if IsCompiledPython(file.Path) {
			continue
		}

		// Skip files in __pycache__
		if IsInPycache(file.Path) {
			continue
		}

		// Skip test files unless explicitly included
		if !opts.IncludeTestFiles && IsTestFile(file.Path) {
			continue
		}

		// Check if it's a Python source file
		if IsPythonFile(file.Path) {
			pythonFiles = append(pythonFiles, file.Path)
		}
	}

	return pythonFiles, nil
}

// ScanProjectAllLanguages scans a project and returns all source files grouped by language.
// This is useful for multi-language projects.
//
// Parameters:
//   - root: The root directory to scan
//
// Returns:
//   - map[string][]string: Map of language to list of file paths
//   - error: Any error encountered during scanning
func ScanProjectAllLanguages(root string) (map[string][]string, error) {
	// Create scanner with default options
	scanOpts := scanner.DefaultOptions()
	s := scanner.New(scanOpts)

	files, err := s.Scan(root)
	if err != nil {
		return nil, fmt.Errorf("scanning project: %w", err)
	}

	// Group files by language
	filesByLanguage := make(map[string][]string)
	for _, file := range files {
		if file.Language != "" {
			filesByLanguage[file.Language] = append(filesByLanguage[file.Language], file.Path)
		} else {
			filesByLanguage["unknown"] = append(filesByLanguage["unknown"], file.Path)
		}
	}

	return filesByLanguage, nil
}
