// Package callgraph provides intra-file call graph building functionality.
// It analyzes function bodies to identify function calls and builds a mapping
// from caller functions to callee functions within a single file.
package callgraph

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/user/go-context-query/pkg/types"
)

// CallType represents the type of function call
type CallType string

const (
	// LocalCall is a call to a function defined within the same file
	LocalCall CallType = "local"
	// ExternalCall is a call to an imported function or builtin
	ExternalCall CallType = "external"
	// MethodCall is a method call on an object (self.method())
	MethodCall CallType = "method"
	// UnknownCall is a call of unknown origin
	UnknownCall CallType = "unknown"
)

// CalledFunction represents a single called function within a caller
type CalledFunction struct {
	// Name is the full name of the called function (e.g., "my_func" or "self.method")
	Name string `json:"name"`
	// Base is the object/identifier being called (e.g., "self" or module name)
	Base string `json:"base"`
	// Method is the method name if this is a method call (e.g., "method" for self.method)
	Method string `json:"method,omitempty"`
	// CallType indicates whether this is a local, external, or method call
	Type CallType `json:"type"`
	// LineNumber is the line number where the call occurs
	LineNumber int `json:"line_number"`
	// IsAttribute indicates if this is an attribute/method access
	IsAttribute bool `json:"is_attribute"`
}

// CallGraphEntry represents all calls from a single caller function
type CallGraphEntry struct {
	// Caller is the name of the calling function
	Caller string `json:"caller"`
	// Calls is the list of functions called by this caller
	Calls []CalledFunction `json:"calls"`
	// LineNumber is the line where the caller function is defined
	LineNumber int `json:"line_number"`
}

// IntraFileCallGraph represents the call graph for a single file
type IntraFileCallGraph struct {
	// FilePath is the path to the source file
	FilePath string `json:"file_path"`
	// Entries maps function names to their call graph entries
	Entries map[string]*CallGraphEntry `json:"entries"`
	// LocalFunctions is a set of function names defined in the file
	LocalFunctions map[string]bool `json:"-"`
	// ImportedNames maps imported aliases to their full module paths
	ImportedNames map[string]string `json:"-"`
}

// NewIntraFileCallGraph creates a new empty call graph
func NewIntraFileCallGraph(filePath string) *IntraFileCallGraph {
	return &IntraFileCallGraph{
		FilePath:       filePath,
		Entries:        make(map[string]*CallGraphEntry),
		LocalFunctions: make(map[string]bool),
		ImportedNames:  make(map[string]string),
	}
}

// Builder builds intra-file call graphs using tree-sitter parsing
type Builder struct {
	parser *sitter.Parser
}

// NewBuilder creates a new call graph builder for Python files
func NewBuilder() *Builder {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())
	return &Builder{parser: parser}
}

// BuildFromFile builds a call graph by analyzing a Python source file
func (b *Builder) BuildFromFile(filePath string, moduleInfo *types.ModuleInfo) (*IntraFileCallGraph, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	return b.BuildFromBytes(content, filePath, moduleInfo)
}

// BuildFromBytes builds a call graph from Python source code bytes
func (b *Builder) BuildFromBytes(content []byte, filePath string, moduleInfo *types.ModuleInfo) (*IntraFileCallGraph, error) {
	graph := NewIntraFileCallGraph(filePath)

	// Index local functions
	for _, fn := range moduleInfo.Functions {
		graph.LocalFunctions[fn.Name] = true
	}
	for _, cls := range moduleInfo.Classes {
		for _, method := range cls.Methods {
			graph.LocalFunctions[method.Name] = true
		}
	}

	// Index imports
	for _, imp := range moduleInfo.Imports {
		if imp.IsFrom {
			// from module import name1, name2
			for _, name := range imp.Names {
				graph.ImportedNames[name] = imp.Module
			}
		} else {
			// import module or import module as alias
			for _, name := range imp.Names {
				// For "import os", name is "os"
				// For "import os as operating_system", name is "operating_system"
				graph.ImportedNames[name] = name
			}
		}
	}

	// Parse the AST
	tree := b.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()

	// Build call graph by walking function bodies
	b.walkForCallGraph(root, content, graph, nil)

	return graph, nil
}

// walkForCallGraph recursively walks the AST to build the call graph
func (b *Builder) walkForCallGraph(node *sitter.Node, content []byte, graph *IntraFileCallGraph, currentFunction *CallGraphEntry) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "function_definition":
		// Parse the function and create a new entry
		fn := b.parseFunctionForCallGraph(node, content)
		if fn != nil {
			graph.Entries[fn.Caller] = fn
			// Walk the function body to find calls
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child != nil && child.Type() == "block" {
					b.walkForCallGraph(child, content, graph, fn)
				}
			}
		}
		return // Don't traverse into function bodies here
	case "class_definition":
		// Walk into class body to find method definitions
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "block" {
				// Walk into class block to find method definitions
				for j := 0; j < int(child.ChildCount()); j++ {
					b.walkForCallGraph(child.Child(j), content, graph, nil)
				}
			}
		}
		return
	case "call":
		// Found a function call - extract it
		if currentFunction != nil {
			calledFn := b.extractCall(node, content, graph)
			if calledFn != nil {
				currentFunction.Calls = append(currentFunction.Calls, *calledFn)
			}
		}
	}

	// Recursively walk children (only if not already handled above)
	for i := 0; i < int(node.ChildCount()); i++ {
		b.walkForCallGraph(node.Child(i), content, graph, currentFunction)
	}
}

// parseFunctionForCallGraph extracts function name and creates an entry
func (b *Builder) parseFunctionForCallGraph(node *sitter.Node, content []byte) *CallGraphEntry {
	if node == nil || node.Type() != "function_definition" {
		return nil
	}

	var name string
	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "identifier" {
			name = b.nodeText(child, content)
			break
		}
	}

	if name == "" {
		return nil
	}

	return &CallGraphEntry{
		Caller:     name,
		Calls:      []CalledFunction{},
		LineNumber: lineNumber,
	}
}

// extractCall extracts call information from a call node
func (b *Builder) extractCall(node *sitter.Node, content []byte, graph *IntraFileCallGraph) *CalledFunction {
	if node == nil || node.Type() != "call" {
		return nil
	}

	// The first child of a call node is the function being called
	if node.ChildCount() == 0 {
		return nil
	}

	fnNode := node.Child(0)
	if fnNode == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1

	switch fnNode.Type() {
	case "identifier":
		// Simple function call: my_function()
		name := b.nodeText(fnNode, content)
		callType := b.determineCallType(name, graph)

		return &CalledFunction{
			Name:        name,
			Base:        name,
			Type:        callType,
			LineNumber:  lineNumber,
			IsAttribute: false,
		}

	case "attribute":
		// Method call: obj.method() or module.function()
		base, method := b.extractAttributeParts(fnNode, content)
		name := b.nodeText(fnNode, content)
		callType := b.determineAttributeCallType(base, method, graph)

		return &CalledFunction{
			Name:        name,
			Base:        base,
			Method:      method,
			Type:        callType,
			LineNumber:  lineNumber,
			IsAttribute: true,
		}

	case "call":
		// Chained call: foo()() - treat as call result
		// Extract the inner call
		return b.extractCall(fnNode, content, graph)

	case "subscript":
		// Call on indexed result: obj[0]()
		name := b.nodeText(fnNode, content)
		return &CalledFunction{
			Name:        name,
			Base:        name,
			Type:        UnknownCall,
			LineNumber:  lineNumber,
			IsAttribute: false,
		}

	default:
		// Other cases: lambda calls, etc.
		name := b.nodeText(fnNode, content)
		return &CalledFunction{
			Name:        name,
			Base:        name,
			Type:        UnknownCall,
			LineNumber:  lineNumber,
			IsAttribute: false,
		}
	}
}

// extractAttributeParts extracts base and method from an attribute node
// For "self.method" returns ("self", "method")
// For "os.path.join" returns ("os.path", "join")
func (b *Builder) extractAttributeParts(node *sitter.Node, content []byte) (base, method string) {
	if node == nil || node.Type() != "attribute" {
		return "", ""
	}

	// Find the last identifier which is the method name
	var parts []string
	b.collectAttributeParts(node, content, &parts)

	if len(parts) == 0 {
		return "", ""
	}

	if len(parts) == 1 {
		return parts[0], ""
	}

	// Last part is the method, everything else is the base
	method = parts[len(parts)-1]
	base = strings.Join(parts[:len(parts)-1], ".")

	return base, method
}

// collectAttributeParts recursively collects all parts of an attribute chain
func (b *Builder) collectAttributeParts(node *sitter.Node, content []byte, parts *[]string) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "identifier":
		*parts = append(*parts, b.nodeText(node, content))
	case "attribute":
		// Walk children to extract parts
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			b.collectAttributeParts(child, content, parts)
		}
	}
}

// determineCallType determines if a simple function call is local or external
func (b *Builder) determineCallType(name string, graph *IntraFileCallGraph) CallType {
	// Check if it's a local function
	if graph.LocalFunctions[name] {
		return LocalCall
	}

	// Check if it's an imported name
	if _, ok := graph.ImportedNames[name]; ok {
		return ExternalCall
	}

	// Check for common Python builtins
	if isPythonBuiltin(name) {
		return ExternalCall
	}

	// Unknown - could be a builtin or undefined
	return UnknownCall
}

// determineAttributeCallType determines the type of an attribute call
func (b *Builder) determineAttributeCallType(base, method string, graph *IntraFileCallGraph) CallType {
	// Check if the first part of base is an imported module (e.g., "os" in "os.path")
	baseParts := strings.Split(base, ".")
	if len(baseParts) > 0 {
		if _, ok := graph.ImportedNames[baseParts[0]]; ok {
			return ExternalCall
		}
	}

	// Check for self/cls method calls
	if base == "self" || base == "cls" {
		return MethodCall
	}

	// Check for other common instance names that might be method calls
	// This is a heuristic - in Task 9 we'll do proper type resolution
	if isLikelyInstanceName(base) {
		return MethodCall
	}

	// Check if method is a local function (less likely for attribute calls)
	if graph.LocalFunctions[method] {
		return LocalCall
	}

	return UnknownCall
}

// isPythonBuiltin checks if a name is a common Python builtin
func isPythonBuiltin(name string) bool {
	builtins := []string{
		"abs", "all", "any", "ascii", "bin", "bool", "breakpoint", "bytearray",
		"bytes", "callable", "chr", "classmethod", "compile", "complex",
		"delattr", "dict", "dir", "divmod", "enumerate", "eval", "exec",
		"filter", "float", "format", "frozenset", "getattr", "globals",
		"hasattr", "hash", "help", "hex", "id", "input", "int", "isinstance",
		"issubclass", "iter", "len", "list", "locals", "map", "max", "min",
		"next", "object", "oct", "open", "ord", "pow", "print", "property",
		"range", "repr", "reversed", "round", "set", "setattr", "slice",
		"sorted", "staticmethod", "str", "sum", "super", "tuple", "type",
		"vars", "zip", "__import__",
	}

	for _, b := range builtins {
		if b == name {
			return true
		}
	}
	return false
}

// isLikelyInstanceName checks if a variable name likely refers to an instance
func isLikelyInstanceName(name string) bool {
	// Common instance variable patterns
	instancePatterns := []string{"self", "this", "that", "it", "obj", "instance"}
	for _, pattern := range instancePatterns {
		if strings.ToLower(name) == pattern {
			return true
		}
	}

	// Single letter names are often instance variables
	if len(name) == 1 && name >= "a" && name <= "z" {
		return true
	}

	return false
}

// nodeText extracts the text content of a node from the source
func (b *Builder) nodeText(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	start := node.StartByte()
	end := node.EndByte()
	if start >= uint32(len(content)) || end > uint32(len(content)) {
		return ""
	}
	return string(content[start:end])
}

// GetCalls returns all calls made by a specific function
func (g *IntraFileCallGraph) GetCalls(functionName string) []CalledFunction {
	entry, ok := g.Entries[functionName]
	if !ok {
		return nil
	}
	return entry.Calls
}

// GetLocalCalls returns only local calls made by a specific function
func (g *IntraFileCallGraph) GetLocalCalls(functionName string) []CalledFunction {
	var localCalls []CalledFunction
	calls := g.GetCalls(functionName)
	for _, call := range calls {
		if call.Type == LocalCall {
			localCalls = append(localCalls, call)
		}
	}
	return localCalls
}

// GetExternalCalls returns only external calls made by a specific function
func (g *IntraFileCallGraph) GetExternalCalls(functionName string) []CalledFunction {
	var externalCalls []CalledFunction
	calls := g.GetCalls(functionName)
	for _, call := range calls {
		if call.Type == ExternalCall {
			externalCalls = append(externalCalls, call)
		}
	}
	return externalCalls
}

// GetMethodCalls returns only method calls made by a specific function
func (g *IntraFileCallGraph) GetMethodCalls(functionName string) []CalledFunction {
	var methodCalls []CalledFunction
	calls := g.GetCalls(functionName)
	for _, call := range calls {
		if call.Type == MethodCall {
			methodCalls = append(methodCalls, call)
		}
	}
	return methodCalls
}

// ToCallGraph converts IntraFileCallGraph to types.CallGraph format
func (g *IntraFileCallGraph) ToCallGraph() types.CallGraph {
	var edges []types.CallGraphEdge

	for callerName, entry := range g.Entries {
		for _, call := range entry.Calls {
			edge := types.CallGraphEdge{
				SourceFile: g.FilePath,
				SourceFunc: callerName,
				DestFile:   g.FilePath,
				DestFunc:   call.Name,
			}
			edges = append(edges, edge)
		}
	}

	return types.CallGraph{Edges: edges}
}

// GetAllFunctions returns a list of all functions in the call graph
func (g *IntraFileCallGraph) GetAllFunctions() []string {
	functions := make([]string, 0, len(g.Entries))
	for name := range g.Entries {
		functions = append(functions, name)
	}
	return functions
}

// HasCalls returns true if the given function makes any calls
func (g *IntraFileCallGraph) HasCalls(functionName string) bool {
	entry, ok := g.Entries[functionName]
	if !ok {
		return false
	}
	return len(entry.Calls) > 0
}

// GetCallCount returns the total number of calls in the graph
func (g *IntraFileCallGraph) GetCallCount() int {
	count := 0
	for _, entry := range g.Entries {
		count += len(entry.Calls)
	}
	return count
}
