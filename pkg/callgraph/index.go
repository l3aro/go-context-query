// Package callgraph provides cross-file project indexing functionality.
// It builds a project-wide function index by scanning project directories
// and extracting function definitions from Python source files.
package callgraph

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/l3aro/go-context-query/pkg/extractor"
)

// FunctionEntry represents a single function in the index with its metadata.
type FunctionEntry struct {
	// Name is the function name
	Name string
	// QualifiedName is the full qualified name (e.g., "module.func" or "module.Class.method")
	QualifiedName string
	// FilePath is the absolute path to the source file
	FilePath string
	// ModuleName is the dotted module path
	ModuleName string
	// IsMethod indicates if this is a class method
	IsMethod bool
	// IsNested indicates if this is a nested function
	IsNested bool
	// ParentName is the parent function or class name for methods/nested functions
	ParentName string
	// LineNumber is the line where the function is defined
	LineNumber int
}

// ProjectIndex is a project-wide function index that maps qualified function
// names to their file paths. It supports:
//   - Top-level functions: module.func
//   - Class methods: module.Class.method
//   - Nested functions: module.outer.nested
//   - Simple function names for quick lookup
//
// The index is safe for concurrent read access but not for concurrent writes.
// Use BuildIndex to populate the index, then use Lookup methods for queries.
type ProjectIndex struct {
	// rootDir is the project root directory
	rootDir string

	// funcToFile maps qualified function names to file paths
	// Keys can be:
	//   - "function_name" (simple name)
	//   - "module.function_name" (module-qualified)
	//   - "module.ClassName.method" (class method)
	//   - "module.outer.nested" (nested function)
	funcToFile map[string]string

	// entries stores detailed metadata for each function
	entries map[string]FunctionEntry

	// fileToFunctions maps file paths to function names defined in them
	fileToFunctions map[string][]string

	// moduleToFiles maps module names to their file paths
	moduleToFiles map[string]string

	// extractor is used to parse Python files
	extractor extractor.Extractor

	// importResolver resolves Python imports
	importResolver *PythonImportResolver

	// mu protects the index maps
	mu sync.RWMutex

	// parsedFiles tracks which files have been parsed to avoid re-parsing
	parsedFiles map[string]bool

	// extensionMap provides O(1) lookup for supported file extensions
	extensionMap map[string]struct{}
}

// NewProjectIndex creates a new empty project index.
// The rootDir should be the project root directory.
func NewProjectIndex(rootDir string) *ProjectIndex {
	ext := extractor.NewPythonExtractor()
	extMap := make(map[string]struct{}, len(ext.FileExtensions()))
	for _, e := range ext.FileExtensions() {
		extMap[e] = struct{}{}
	}

	return &ProjectIndex{
		rootDir:         rootDir,
		funcToFile:      make(map[string]string),
		entries:         make(map[string]FunctionEntry),
		fileToFunctions: make(map[string][]string),
		moduleToFiles:   make(map[string]string),
		extractor:       ext,
		importResolver:  NewPythonImportResolver(rootDir),
		parsedFiles:     make(map[string]bool),
		extensionMap:    extMap,
	}
}

// WithExtractor sets a custom extractor for the index.
// Useful for testing or when using a different extractor implementation.
func (idx *ProjectIndex) WithExtractor(ext extractor.Extractor) *ProjectIndex {
	idx.extractor = ext
	idx.extensionMap = make(map[string]struct{}, len(ext.FileExtensions()))
	for _, e := range ext.FileExtensions() {
		idx.extensionMap[e] = struct{}{}
	}
	return idx
}

// WithImportResolver sets a custom import resolver for the index.
// Useful for testing or custom import resolution logic.
func (idx *ProjectIndex) WithImportResolver(resolver *PythonImportResolver) *ProjectIndex {
	idx.importResolver = resolver
	return idx
}

// BuildIndexFromScan scans the project and builds the function index.
// It uses ScanProject to find all Python files, then extracts functions from each.
//
// Parameters:
//   - language: The language to scan for (currently only "python" is supported)
//
// Returns an error if scanning or extraction fails.
func (idx *ProjectIndex) BuildIndexFromScan(language string) error {
	files, err := ScanProject(idx.rootDir, language)
	if err != nil {
		return fmt.Errorf("scanning project: %w", err)
	}

	absoluteFiles := make([]string, len(files))
	for i, relPath := range files {
		absoluteFiles[i] = filepath.Join(idx.rootDir, relPath)
	}

	return idx.BuildIndex(absoluteFiles)
}

// BuildIndex builds the function index from the given file paths.
// It parses each file and extracts all function definitions.
//
// Parameters:
//   - filePaths: List of file paths to index
//
// Returns an error if extraction fails for any file.
// Already parsed files are skipped to avoid re-parsing.
func (idx *ProjectIndex) BuildIndex(filePaths []string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for _, filePath := range filePaths {
		// Skip already parsed files
		if idx.parsedFiles[filePath] {
			continue
		}

		// Skip non-Python files
		if !idx.isSupportedFile(filePath) {
			continue
		}

		if err := idx.indexFile(filePath); err != nil {
			// Log error but continue with other files
			continue
		}

		idx.parsedFiles[filePath] = true
	}

	return nil
}

// isSupportedFile checks if a file has a supported extension.
func (idx *ProjectIndex) isSupportedFile(filePath string) bool {
	if idx.extensionMap == nil {
		return false
	}
	ext := filepath.Ext(filePath)
	_, ok := idx.extensionMap[ext]
	return ok
}

// indexFile extracts and indexes all functions from a single file.
func (idx *ProjectIndex) indexFile(filePath string) error {
	moduleInfo, err := idx.extractor.Extract(filePath)
	if err != nil {
		return fmt.Errorf("extracting from %s: %w", filePath, err)
	}

	// Derive module name from file path
	relPath, err := filepath.Rel(idx.rootDir, filePath)
	if err != nil {
		relPath = filePath
	}
	moduleName := idx.filePathToModuleName(relPath)

	// Track the module to file mapping
	idx.moduleToFiles[moduleName] = filePath

	// Index top-level functions
	for _, fn := range moduleInfo.Functions {
		idx.addFunction(filePath, moduleName, fn.Name, fn.LineNumber, false, "", false)
	}

	// Index classes and their methods
	for _, cls := range moduleInfo.Classes {
		// Index the class as a type
		idx.addFunction(filePath, moduleName, cls.Name, cls.LineNumber, false, "", false)

		// Index methods with qualified names
		for _, method := range cls.Methods {
			// Add as simple method name
			idx.addFunction(filePath, moduleName, method.Name, method.LineNumber, true, cls.Name, false)
			// Add as qualified method name: Class.method
			qualifiedMethodName := cls.Name + "." + method.Name
			idx.addFunction(filePath, moduleName, qualifiedMethodName, method.LineNumber, true, cls.Name, false)
		}
	}

	return nil
}

// filePathToModuleName converts a file path to a dotted module name.
// Example: "pkg/utils.py" -> "pkg.utils"
func (idx *ProjectIndex) filePathToModuleName(filePath string) string {
	// Remove supported extensions
	for _, ext := range idx.extractor.FileExtensions() {
		filePath = strings.TrimSuffix(filePath, ext)
	}

	// Convert path separators to dots
	filePath = filepath.ToSlash(filePath)
	return strings.ReplaceAll(filePath, "/", ".")
}

// addFunction adds a function to the index with all its name variants.
func (idx *ProjectIndex) addFunction(filePath, moduleName, funcName string, lineNum int, isMethod bool, parentName string, isNested bool) {
	entry := FunctionEntry{
		Name:       funcName,
		FilePath:   filePath,
		ModuleName: moduleName,
		IsMethod:   isMethod,
		IsNested:   isNested,
		ParentName: parentName,
		LineNumber: lineNum,
	}

	// Simple function name
	simpleKey := funcName
	idx.funcToFile[simpleKey] = filePath
	entry.QualifiedName = funcName
	idx.entries[simpleKey] = entry

	// Module-qualified name
	if moduleName != "" {
		qualifiedKey := moduleName + "." + funcName
		idx.funcToFile[qualifiedKey] = filePath
		entry.QualifiedName = qualifiedKey
		idx.entries[qualifiedKey] = entry
	}

	// Track functions by file
	idx.fileToFunctions[filePath] = append(idx.fileToFunctions[filePath], funcName)
}

// Lookup finds the file path for a given function.
// It accepts various formats:
//   - "function_name" - simple function name
//   - "module.function_name" - module-qualified function
//   - "ClassName.method" - class method
//   - "module.ClassName.method" - fully qualified method
//
// Returns the file path and true if found, empty string and false otherwise.
func (idx *ProjectIndex) Lookup(funcName string) (string, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if file, ok := idx.funcToFile[funcName]; ok {
		return file, true
	}

	return "", false
}

// LookupWithModule finds the file path for a function using module and function name.
// This is useful when you have the module and function separately.
//
// Parameters:
//   - moduleName: The dotted module path (e.g., "pkg.utils")
//   - funcName: The function name (e.g., "my_func" or "MyClass.method")
//
// Returns the file path and true if found, empty string and false otherwise.
func (idx *ProjectIndex) LookupWithModule(moduleName, funcName string) (string, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Try fully qualified name first
	if moduleName != "" {
		key := moduleName + "." + funcName
		if file, ok := idx.funcToFile[key]; ok {
			return file, true
		}

		// Try with simple module name (last component)
		parts := strings.Split(moduleName, ".")
		if len(parts) > 0 {
			simpleKey := parts[len(parts)-1] + "." + funcName
			if file, ok := idx.funcToFile[simpleKey]; ok {
				return file, true
			}
		}
	}

	// Try simple function name
	if file, ok := idx.funcToFile[funcName]; ok {
		return file, true
	}

	return "", false
}

// LookupEntry retrieves the full function entry metadata.
// Returns the entry and true if found, empty entry and false otherwise.
func (idx *ProjectIndex) LookupEntry(qualifiedName string) (FunctionEntry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if entry, ok := idx.entries[qualifiedName]; ok {
		return entry, true
	}

	return FunctionEntry{}, false
}

// GetFunctionsInFile returns all function names defined in a given file.
func (idx *ProjectIndex) GetFunctionsInFile(filePath string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if funcs, ok := idx.fileToFunctions[filePath]; ok {
		result := make([]string, len(funcs))
		copy(result, funcs)
		return result
	}

	return nil
}

// GetModuleFile returns the file path for a given module name.
func (idx *ProjectIndex) GetModuleFile(moduleName string) (string, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if file, ok := idx.moduleToFiles[moduleName]; ok {
		return file, true
	}

	return "", false
}

// GetAllFunctions returns all function names in the index.
func (idx *ProjectIndex) GetAllFunctions() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	funcs := make([]string, 0, len(idx.entries))
	seen := make(map[string]bool)
	for key, entry := range idx.entries {
		// Only include qualified names to avoid duplicates
		if strings.Contains(key, ".") && !seen[entry.Name] {
			funcs = append(funcs, entry.Name)
			seen[entry.Name] = true
		}
	}

	return funcs
}

// GetStats returns statistics about the index.
func (idx *ProjectIndex) GetStats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return IndexStats{
		TotalFunctions: len(idx.entries),
		TotalFiles:     len(idx.fileToFunctions),
		TotalModules:   len(idx.moduleToFiles),
		ParsedFiles:    len(idx.parsedFiles),
	}
}

// IndexStats holds statistics about the project index.
type IndexStats struct {
	TotalFunctions int
	TotalFiles     int
	TotalModules   int
	ParsedFiles    int
}

// AddNestedFunction adds a nested function to the index.
// This handles functions defined inside other functions.
//
// Parameters:
//   - filePath: Path to the source file
//   - moduleName: The module name
//   - outerFunc: The outer function name
//   - nestedFunc: The nested function name
//   - lineNum: Line number where the nested function is defined
func (idx *ProjectIndex) AddNestedFunction(filePath, moduleName, outerFunc, nestedFunc string, lineNum int) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Qualified nested name: outer.nested
	qualifiedNestedName := outerFunc + "." + nestedFunc

	// Add with module prefix
	fullQualifiedName := moduleName + "." + qualifiedNestedName

	entry := FunctionEntry{
		Name:          nestedFunc,
		QualifiedName: fullQualifiedName,
		FilePath:      filePath,
		ModuleName:    moduleName,
		IsMethod:      false,
		IsNested:      true,
		ParentName:    outerFunc,
		LineNumber:    lineNum,
	}

	// Add simple nested name
	idx.funcToFile[qualifiedNestedName] = filePath
	idx.entries[qualifiedNestedName] = entry

	// Add full qualified name
	idx.funcToFile[fullQualifiedName] = filePath
	idx.entries[fullQualifiedName] = entry

	// Also add simple name for lookup
	idx.funcToFile[nestedFunc] = filePath

	// Track in file functions
	idx.fileToFunctions[filePath] = append(idx.fileToFunctions[filePath], qualifiedNestedName)
}

// RefreshFile re-indexes a single file, updating existing entries.
// This is useful when a file has been modified.
func (idx *ProjectIndex) RefreshFile(filePath string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove existing entries for this file
	if funcs, ok := idx.fileToFunctions[filePath]; ok {
		for _, funcName := range funcs {
			delete(idx.funcToFile, funcName)
			delete(idx.entries, funcName)
		}
		delete(idx.fileToFunctions, filePath)
	}

	// Remove from parsed files to force re-parsing
	delete(idx.parsedFiles, filePath)

	// Re-index
	return idx.indexFile(filePath)
}

// IsIndexed checks if a file has already been indexed.
func (idx *ProjectIndex) IsIndexed(filePath string) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.parsedFiles[filePath]
}
