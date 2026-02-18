// Package callgraph provides cross-file call graph resolution functionality.
// It builds a project-wide call graph by analyzing imports and matching call
// sites to function definitions across multiple files.
package callgraph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/l3aro/go-context-query/pkg/extractor"
	"github.com/l3aro/go-context-query/pkg/types"
)

// FunctionIndex maps qualified function names to their file paths.
// The key format is "module.function" or just "function" for simple lookups.
type FunctionIndex struct {
	mu sync.RWMutex

	// funcToFile maps function identifiers to file paths
	// Keys can be:
	//   - "function_name" (simple name)
	//   - "module.function_name" (qualified name)
	//   - "module_path:function_name" (file-based lookup)
	funcToFile map[string]string

	// fileToFunctions maps file paths to the functions defined in them
	fileToFunctions map[string][]string
}

// NewFunctionIndex creates a new empty function index.
func NewFunctionIndex() *FunctionIndex {
	return &FunctionIndex{
		funcToFile:      make(map[string]string),
		fileToFunctions: make(map[string][]string),
	}
}

// AddFunction adds a function to the index.
// moduleName is the dotted module path (e.g., "pkg.utils")
// funcName is the function name
// filePath is the absolute path to the file
func (idx *FunctionIndex) AddFunction(moduleName, funcName, filePath string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Add simple name mapping
	simpleKey := funcName
	if _, exists := idx.funcToFile[simpleKey]; !exists {
		idx.funcToFile[simpleKey] = filePath
	}

	// Add qualified name mapping
	if moduleName != "" {
		qualifiedKey := moduleName + "." + funcName
		idx.funcToFile[qualifiedKey] = filePath

		// Also add the simple module name (last component)
		parts := strings.Split(moduleName, ".")
		if len(parts) > 0 {
			simpleModuleKey := parts[len(parts)-1] + "." + funcName
			idx.funcToFile[simpleModuleKey] = filePath
		}
	}

	// Track functions by file
	idx.fileToFunctions[filePath] = append(idx.fileToFunctions[filePath], funcName)
}

// Lookup finds the file path for a given function.
// It tries qualified names first, then simple names.
func (idx *FunctionIndex) Lookup(moduleName, funcName string) (string, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Try fully qualified name first
	if moduleName != "" {
		key := moduleName + "." + funcName
		if file, ok := idx.funcToFile[key]; ok {
			return file, true
		}

		// Try with simple module name
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

// LookupByQualifiedName looks up a function by its qualified name (e.g., "module.func").
func (idx *FunctionIndex) LookupByQualifiedName(qualifiedName string) (string, bool) {
	if file, ok := idx.funcToFile[qualifiedName]; ok {
		return file, true
	}
	return "", false
}

// GetFunctionsInFile returns all functions defined in a given file.
func (idx *FunctionIndex) GetFunctionsInFile(filePath string) []string {
	return idx.fileToFunctions[filePath]
}

// ImportResolver resolves imported module names to file paths.
type ImportResolver struct {
	rootDir string
	index   *FunctionIndex
}

// NewImportResolver creates a new import resolver.
func NewImportResolver(rootDir string, index *FunctionIndex) *ImportResolver {
	return &ImportResolver{
		rootDir: rootDir,
		index:   index,
	}
}

// ImportMapping represents a resolved import with its local alias.
type ImportMapping struct {
	// LocalName is the name used in the source file (may be an alias)
	LocalName string
	// ModulePath is the dotted module path
	ModulePath string
	// OriginalName is the original imported name (for from imports)
	OriginalName string
	// IsFrom indicates if this was a "from X import Y" style import
	IsFrom bool
	// IsRelative indicates if this is a relative import
	IsRelative bool
	// RelativeLevel is the number of dots for relative imports (1 for ., 2 for ..)
	RelativeLevel int
}

// ResolveImport resolves an import statement to a file path.
// Returns the file path and true if resolved, or empty string and false if not.
func (r *ImportResolver) ResolveImport(imp types.Import, fromFile string) (*ImportMapping, error) {
	mapping := &ImportMapping{
		ModulePath:    imp.Module,
		IsFrom:        imp.IsFrom,
		IsRelative:    extractor.IsRelativeImport(imp.Module),
		RelativeLevel: extractor.GetRelativeLevel(imp.Module),
	}

	// Handle relative imports
	if mapping.IsRelative {
		resolvedPath, err := r.resolveRelativeImport(imp.Module, fromFile)
		if err != nil {
			return nil, err
		}
		mapping.ModulePath = resolvedPath
	}

	return mapping, nil
}

// resolveRelativeImport resolves a relative import path to an absolute module path.
// Examples:
//   - "." from "pkg/utils.py" -> "pkg"
//   - ".helpers" from "pkg/utils.py" -> "pkg.helpers"
//   - ".." from "pkg/sub/mod.py" -> "pkg"
//   - "..utils" from "pkg/sub/mod.py" -> "pkg.utils"
func (r *ImportResolver) resolveRelativeImport(module string, fromFile string) (string, error) {
	// Get the directory of the source file
	fromDir := filepath.Dir(fromFile)
	relDir, err := filepath.Rel(r.rootDir, fromDir)
	if err != nil {
		return "", fmt.Errorf("getting relative directory: %w", err)
	}

	// Normalize to use dots for module path
	relDir = filepath.ToSlash(relDir)
	if relDir == "." {
		relDir = ""
	}

	// Count leading dots and get the module part
	level := 0
	for i, ch := range module {
		if ch == '.' {
			level++
		} else {
			module = module[i:]
			break
		}
		if i == len(module)-1 {
			module = ""
		}
	}

	// Navigate up the directory tree
	parts := strings.Split(relDir, "/")
	if len(parts) < level {
		return "", fmt.Errorf("relative import goes beyond package root: %s from %s", module, fromFile)
	}

	// Remove 'level' number of directories from the end
	if len(parts) > 0 && parts[0] != "" {
		parts = parts[:len(parts)-level]
	}

	// Append the module part if present
	if module != "" {
		parts = append(parts, strings.ReplaceAll(module, "/", "."))
	}

	// Join to form the full module path
	result := strings.Join(parts, ".")
	return result, nil
}

// Resolver builds and resolves cross-file call graphs.
type Resolver struct {
	mu          sync.RWMutex
	parseMu     sync.Mutex // Protects tree-sitter parser (not thread-safe)
	rootDir     string
	index       *FunctionIndex
	importCache map[string][]types.Import // filePath -> imports
	callGraph   *CrossFileCallGraph
	extractor   extractor.Extractor
	builder     *Builder
}

// CrossFileCallGraph represents a complete cross-file call graph.
type CrossFileCallGraph struct {
	// Edges contains all call edges as (caller_file, caller_func, callee_file, callee_func)
	Edges []types.CallGraphEdge
	// IntraFileEdges contains edges where caller and callee are in the same file
	IntraFileEdges []types.CallGraphEdge
	// CrossFileEdges contains edges where caller and callee are in different files
	CrossFileEdges []types.CallGraphEdge
	// UnresolvedCalls contains calls that couldn't be resolved
	UnresolvedCalls []UnresolvedCall
}

// UnresolvedCall represents a call that couldn't be resolved to a definition.
type UnresolvedCall struct {
	CallerFile string
	CallerFunc string
	CallName   string
	Reason     string
}

// NewResolver creates a new cross-file call graph resolver.
// It accepts an Extractor interface to support any language.
func NewResolver(rootDir string, ext extractor.Extractor) *Resolver {
	return &Resolver{
		rootDir:     rootDir,
		index:       NewFunctionIndex(),
		importCache: make(map[string][]types.Import),
		callGraph: &CrossFileCallGraph{
			Edges:           []types.CallGraphEdge{},
			IntraFileEdges:  []types.CallGraphEdge{},
			CrossFileEdges:  []types.CallGraphEdge{},
			UnresolvedCalls: []UnresolvedCall{},
		},
		extractor: ext,
		builder:   NewBuilder(),
	}
}

// isSupportedFile checks if a file has a supported extension for the extractor.
func (r *Resolver) isSupportedFile(filePath string) bool {
	for _, ext := range r.extractor.FileExtensions() {
		if strings.HasSuffix(filePath, ext) {
			return true
		}
	}
	return false
}

// BuildIndex builds the function index from all files supported by the extractor.
func (r *Resolver) BuildIndex(filePaths []string) error {
	var wg sync.WaitGroup

	for _, filePath := range filePaths {
		wg.Add(1)

		go func(fp string) {
			defer wg.Done()

			if !r.isSupportedFile(fp) {
				return
			}

			// Parse mutex needed - tree-sitter parser isn't thread-safe
			r.parseMu.Lock()
			moduleInfo, err := r.extractor.Extract(fp)
			r.parseMu.Unlock()

			if err != nil {
				return
			}

			// Derive module name from file path
			relPath, err := filepath.Rel(r.rootDir, fp)
			if err != nil {
				return
			}

			moduleName := r.filePathToModuleName(relPath)

			// Index all functions
			for _, fn := range moduleInfo.Functions {
				r.index.AddFunction(moduleName, fn.Name, fp)
			}

			// Index all class methods
			for _, cls := range moduleInfo.Classes {
				// Index the class itself
				r.index.AddFunction(moduleName, cls.Name, fp)
				// Index methods
				for _, method := range cls.Methods {
					r.index.AddFunction(moduleName, method.Name, fp)
					// Also add qualified method name
					r.index.AddFunction(moduleName, cls.Name+"."+method.Name, fp)
				}
			}

			// Cache imports for later use (thread-safe)
			r.mu.Lock()
			r.importCache[fp] = moduleInfo.Imports
			r.mu.Unlock()
		}(filePath)
	}

	wg.Wait()

	return nil
}

// filePathToModuleName converts a file path to a dotted module name.
// Example: "pkg/utils.py" -> "pkg.utils"
func (r *Resolver) filePathToModuleName(filePath string) string {
	for _, ext := range r.extractor.FileExtensions() {
		filePath = strings.TrimSuffix(filePath, ext)
	}

	// Convert path separators to dots
	filePath = filepath.ToSlash(filePath)
	return strings.ReplaceAll(filePath, "/", ".")
}

// ResolveCalls resolves all calls in the given files and builds the cross-file call graph.
func (r *Resolver) ResolveCalls(filePaths []string) (*CrossFileCallGraph, error) {
	// First build the index if not already done
	if len(r.index.funcToFile) == 0 {
		if err := r.BuildIndex(filePaths); err != nil {
			return nil, fmt.Errorf("building function index: %w", err)
		}
	}

	resolver := NewImportResolver(r.rootDir, r.index)

	var wg sync.WaitGroup
	errCh := make(chan error, len(filePaths))

	for _, filePath := range filePaths {
		if !r.isSupportedFile(filePath) {
			continue
		}

		wg.Add(1)
		go func(fp string) {
			defer wg.Done()

			if err := r.resolveFileCalls(fp, resolver); err != nil {
				errCh <- err
			}
		}(filePath)
	}

	wg.Wait()
	close(errCh)

	// Collect errors but don't fail - we processed what we could
	for err := range errCh {
		// Log error but continue - we still have partial results
		_ = err
	}

	return r.callGraph, nil
}

// resolveFileCalls resolves calls within a single file.
func (r *Resolver) resolveFileCalls(filePath string, resolver *ImportResolver) error {
	// Get module info - must hold parseMu since tree-sitter parser is not thread-safe
	r.parseMu.Lock()
	moduleInfo, err := r.extractor.Extract(filePath)
	r.parseMu.Unlock()

	if err != nil {
		return fmt.Errorf("extracting module info: %w", err)
	}

	// Get imports
	imports, ok := r.importCache[filePath]
	if !ok {
		imports = moduleInfo.Imports
	}

	// Build import mapping for this file
	importMap := r.buildImportMap(imports, filePath, resolver)

	// Build intra-file call graph (builder.parser is not thread-safe)
	r.parseMu.Lock()
	intraGraph, err := r.builder.BuildFromFile(filePath, moduleInfo)
	r.parseMu.Unlock()
	if err != nil {
		return fmt.Errorf("building intra-file call graph: %w", err)
	}

	relPath, err := filepath.Rel(r.rootDir, filePath)
	if err != nil {
		relPath = filePath
	}

	// Process each call
	for callerName, entry := range intraGraph.Entries {
		for _, call := range entry.Calls {
			r.resolveSingleCall(relPath, callerName, call, intraGraph, importMap)
		}
	}

	return nil
}

// ImportMap holds resolved imports for a file.
type ImportMap struct {
	// nameToModule maps imported names to their module info
	nameToModule map[string]ImportInfo
	// moduleAliases maps local aliases to their full module paths
	moduleAliases map[string]string
}

// ImportInfo holds information about a resolved import.
type ImportInfo struct {
	ModulePath   string
	OriginalName string
	IsFrom       bool
	FilePath     string
}

// buildImportMap builds a mapping of imported names to their resolved info.
func (r *Resolver) buildImportMap(imports []types.Import, fromFile string, resolver *ImportResolver) *ImportMap {
	result := &ImportMap{
		nameToModule:  make(map[string]ImportInfo),
		moduleAliases: make(map[string]string),
	}

	for _, imp := range imports {
		// Resolve the import
		mapping, err := resolver.ResolveImport(imp, fromFile)
		if err != nil {
			// Skip imports that can't be resolved
			continue
		}

		if imp.IsFrom {
			// from module import name1, name2
			for _, name := range imp.Names {
				if name == "*" {
					// Wildcard import - can't resolve specific names
					continue
				}

				info := ImportInfo{
					ModulePath:   mapping.ModulePath,
					OriginalName: name,
					IsFrom:       true,
				}

				// Try to find the file for this import
				if file, ok := r.index.Lookup(mapping.ModulePath, name); ok {
					info.FilePath = file
				}

				result.nameToModule[name] = info
			}
		} else {
			// import module or import module as alias
			for _, name := range imp.Names {
				// The name is either the module itself or an alias
				result.moduleAliases[name] = mapping.ModulePath

				info := ImportInfo{
					ModulePath:   mapping.ModulePath,
					OriginalName: name,
					IsFrom:       false,
				}

				result.nameToModule[name] = info
			}
		}
	}

	return result
}

// resolveSingleCall resolves a single call to its target file and function.
func (r *Resolver) resolveSingleCall(
	callerFile string,
	callerFunc string,
	call CalledFunction,
	intraGraph *IntraFileCallGraph,
	importMap *ImportMap,
) {
	edge := types.CallGraphEdge{
		SourceFile: callerFile,
		SourceFunc: callerFunc,
	}

	switch call.Type {
	case LocalCall:
		// Intra-file call
		edge.DestFile = callerFile
		edge.DestFunc = call.Name
		r.addEdge(edge, true)

	case ExternalCall:
		// Try to resolve via imports
		if resolved := r.resolveExternalCall(call, importMap); resolved != nil {
			edge.DestFile = resolved.DestFile
			edge.DestFunc = resolved.DestFunc
			r.addEdge(edge, false)
		} else {
			// Unresolved external call
			r.mu.Lock()
			r.callGraph.UnresolvedCalls = append(r.callGraph.UnresolvedCalls, UnresolvedCall{
				CallerFile: callerFile,
				CallerFunc: callerFunc,
				CallName:   call.Name,
				Reason:     "external module not resolved",
			})
			r.mu.Unlock()
		}

	case MethodCall:
		// Method calls (self.method()) are intra-file
		edge.DestFile = callerFile
		edge.DestFunc = call.Name
		r.addEdge(edge, true)

	case UnknownCall:
		// Try to resolve as external first, then intra-file
		if resolved := r.resolveExternalCall(call, importMap); resolved != nil {
			edge.DestFile = resolved.DestFile
			edge.DestFunc = resolved.DestFunc
			r.addEdge(edge, false)
		} else if intraGraph.LocalFunctions[call.Name] {
			// It's a local function
			edge.DestFile = callerFile
			edge.DestFunc = call.Name
			r.addEdge(edge, true)
		} else {
			// Truly unresolved
			r.mu.Lock()
			r.callGraph.UnresolvedCalls = append(r.callGraph.UnresolvedCalls, UnresolvedCall{
				CallerFile: callerFile,
				CallerFunc: callerFunc,
				CallName:   call.Name,
				Reason:     "unknown call target",
			})
			r.mu.Unlock()
		}
	}
}

// resolveExternalCall tries to resolve an external call via imports.
func (r *Resolver) resolveExternalCall(call CalledFunction, importMap *ImportMap) *types.CallGraphEdge {
	// Handle attribute calls (module.function())
	if call.IsAttribute && call.Base != "" {
		// Check if base is a module alias
		if modulePath, ok := importMap.moduleAliases[call.Base]; ok {
			// module.func() call
			if file, ok := r.index.Lookup(modulePath, call.Method); ok {
				return &types.CallGraphEdge{
					DestFile: file,
					DestFunc: call.Method,
				}
			}
		}

		// Check if base is an imported name (from X import Y)
		if info, ok := importMap.nameToModule[call.Base]; ok {
			// If the base was imported, and we're calling a method on it
			// This might be calling a function from the imported module
			if info.IsFrom {
				if file, ok := r.index.Lookup(info.ModulePath, call.Method); ok {
					return &types.CallGraphEdge{
						DestFile: file,
						DestFunc: call.Method,
					}
				}
			}
		}
	}

	// Handle simple function calls
	// Check if this name was imported
	if info, ok := importMap.nameToModule[call.Name]; ok && info.FilePath != "" {
		return &types.CallGraphEdge{
			DestFile: info.FilePath,
			DestFunc: info.OriginalName,
		}
	}

	// Try to find in the index by simple name
	if file, ok := r.index.Lookup("", call.Name); ok {
		return &types.CallGraphEdge{
			DestFile: file,
			DestFunc: call.Name,
		}
	}

	return nil
}

// addEdge adds an edge to the call graph, tracking whether it's intra-file or cross-file.
func (r *Resolver) addEdge(edge types.CallGraphEdge, isIntraFile bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callGraph.Edges = append(r.callGraph.Edges, edge)

	if isIntraFile {
		r.callGraph.IntraFileEdges = append(r.callGraph.IntraFileEdges, edge)
	} else {
		r.callGraph.CrossFileEdges = append(r.callGraph.CrossFileEdges, edge)
	}
}

// GetIndex returns the function index.
func (r *Resolver) GetIndex() *FunctionIndex {
	return r.index
}

// GetCallGraph returns the resolved call graph.
func (r *Resolver) GetCallGraph() *CrossFileCallGraph {
	return r.callGraph
}

// BuildProjectCallGraph is a convenience function to build a complete project call graph.
// It scans the project, builds the index, and resolves all calls.
// It accepts an Extractor to support any language.
func BuildProjectCallGraph(rootDir string, ext extractor.Extractor) (*CrossFileCallGraph, error) {
	// Find all files matching the extractor's supported extensions
	filePaths, err := findFilesByExtension(rootDir, ext.FileExtensions())
	if err != nil {
		return nil, fmt.Errorf("finding files: %w", err)
	}

	// Create resolver and build call graph
	resolver := NewResolver(rootDir, ext)

	callGraph, err := resolver.ResolveCalls(filePaths)
	if err != nil {
		return nil, fmt.Errorf("resolving calls: %w", err)
	}

	return callGraph, nil
}

// findFilesByExtension finds all files in the project directory matching the given extensions.
func findFilesByExtension(rootDir string, extensions []string) ([]string, error) {
	var files []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking despite errors
		}

		if info.IsDir() {
			// Skip hidden directories and common non-source directories
			name := info.Name()
			if strings.HasPrefix(name, ".") ||
				name == "__pycache__" ||
				name == "node_modules" ||
				name == "venv" ||
				name == ".venv" ||
				name == "build" ||
				name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check for matching extensions
		for _, ext := range extensions {
			if strings.HasSuffix(path, ext) {
				files = append(files, path)
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// ToTypesCallGraph converts the cross-file call graph to the standard types.CallGraph format.
func (cg *CrossFileCallGraph) ToTypesCallGraph() types.CallGraph {
	return types.CallGraph{
		Edges: cg.Edges,
	}
}

// GetStats returns statistics about the call graph.
func (cg *CrossFileCallGraph) GetStats() CallGraphStats {
	return CallGraphStats{
		TotalEdges:      len(cg.Edges),
		IntraFileEdges:  len(cg.IntraFileEdges),
		CrossFileEdges:  len(cg.CrossFileEdges),
		UnresolvedCalls: len(cg.UnresolvedCalls),
	}
}

// CallGraphStats holds statistics about a call graph.
type CallGraphStats struct {
	TotalEdges      int
	IntraFileEdges  int
	CrossFileEdges  int
	UnresolvedCalls int
}

// PythonImportResolver resolves Python import statements to their canonical forms.
// It handles:
//   - from X import Y → resolves to X.Y
//   - import X → resolves to X
//   - import X as Y → Y maps to X
//   - Relative imports (.module, ..module, etc.)
type PythonImportResolver struct {
	rootDir string
}

// ResolvedImport represents a fully resolved Python import.
type ResolvedImport struct {
	// CanonicalName is the fully qualified module name (e.g., "os.path", "pkg.utils")
	CanonicalName string
	// LocalName is the name used in the source file (may be an alias)
	LocalName string
	// OriginalName is the original imported name (for from imports)
	OriginalName string
	// IsFrom indicates if this was a "from X import Y" style import
	IsFrom bool
	// IsRelative indicates if this is a relative import
	IsRelative bool
	// RelativeLevel is the number of dots for relative imports (1 for ., 2 for ..)
	RelativeLevel int
}

// NewPythonImportResolver creates a new Python import resolver.
func NewPythonImportResolver(rootDir string) *PythonImportResolver {
	return &PythonImportResolver{
		rootDir: rootDir,
	}
}

// Resolve resolves a single import statement to its canonical form.
// It handles regular imports, aliased imports, from imports, and relative imports.
func (r *PythonImportResolver) Resolve(imp types.Import, fromFile string) (*ResolvedImport, error) {
	resolved := &ResolvedImport{
		IsFrom:        imp.IsFrom,
		IsRelative:    extractor.IsRelativeImport(imp.Module),
		RelativeLevel: extractor.GetRelativeLevel(imp.Module),
	}

	module := imp.Module
	if resolved.IsRelative {
		absoluteModule, err := r.resolveRelativeModule(imp.Module, fromFile)
		if err != nil {
			return nil, fmt.Errorf("resolving relative import: %w", err)
		}
		module = absoluteModule
	}

	if imp.IsFrom {
		if len(imp.Names) > 0 {
			name := imp.Names[0]
			resolved.LocalName = name
			resolved.OriginalName = name
			resolved.CanonicalName = module + "." + name
		}
	} else {
		if len(imp.Names) > 0 {
			resolved.LocalName = imp.Names[0]
			resolved.OriginalName = imp.Names[0]
			resolved.CanonicalName = module
		}
	}

	return resolved, nil
}

// ResolveAll resolves all imports from a file and returns a mapping.
// Returns a map of local name → canonical name.
func (r *PythonImportResolver) ResolveAll(imports []types.Import, fromFile string) (map[string]string, error) {
	result := make(map[string]string)

	for _, imp := range imports {
		resolved, err := r.Resolve(imp, fromFile)
		if err != nil {
			continue
		}

		if imp.IsFrom {
			for _, name := range imp.Names {
				if name == "*" {
					continue
				}
				canonical := imp.Module + "." + name
				if resolved.IsRelative {
					absModule, _ := r.resolveRelativeModule(imp.Module, fromFile)
					canonical = absModule + "." + name
				}
				result[name] = canonical
			}
		} else {
			for _, name := range imp.Names {
				result[name] = imp.Module
			}
		}
	}

	return result, nil
}

// resolveRelativeModule converts a relative import to an absolute module path.
// Examples:
//   - "." from "pkg/utils.py" → "pkg"
//   - ".helpers" from "pkg/utils.py" → "pkg.helpers"
//   - ".." from "pkg/sub/mod.py" → "pkg"
//   - "..utils" from "pkg/sub/mod.py" → "pkg.utils"
func (r *PythonImportResolver) resolveRelativeModule(module string, fromFile string) (string, error) {
	fromDir := filepath.Dir(fromFile)
	relDir, err := filepath.Rel(r.rootDir, fromDir)
	if err != nil {
		return "", fmt.Errorf("getting relative directory: %w", err)
	}

	relDir = filepath.ToSlash(relDir)
	if relDir == "." {
		relDir = ""
	}

	level := 0
	for i, ch := range module {
		if ch == '.' {
			level++
		} else {
			module = module[i:]
			break
		}
		if i == len(module)-1 {
			module = ""
		}
	}

	parts := strings.Split(relDir, "/")
	if len(parts) < level {
		return "", fmt.Errorf("relative import goes beyond package root: %s from %s", module, fromFile)
	}

	if level > 0 && len(parts) >= level-1 {
		parts = parts[:len(parts)-(level-1)]
	}

	if module != "" {
		parts = append(parts, module)
	}

	result := strings.Join(parts, ".")
	return result, nil
}

// LookupCanonicalName looks up the canonical name for a local name in the import map.
// Returns the canonical name and true if found, or empty string and false.
func (r *PythonImportResolver) LookupCanonicalName(importMap map[string]string, localName string) (string, bool) {
	if canonical, ok := importMap[localName]; ok {
		return canonical, true
	}
	return "", false
}

// ResolveQualifiedName resolves a potentially qualified name (e.g., "os.path.join")
// using the import map. Returns the canonical qualified name.
func (r *PythonImportResolver) ResolveQualifiedName(importMap map[string]string, qualifiedName string) string {
	parts := strings.Split(qualifiedName, ".")
	if len(parts) == 0 {
		return qualifiedName
	}

	if canonical, ok := importMap[parts[0]]; ok {
		parts[0] = canonical
		return strings.Join(parts, ".")
	}

	return qualifiedName
}
