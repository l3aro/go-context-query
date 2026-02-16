// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/l3aro/go-context-query/pkg/types"
)

// PythonExtractor implements the Extractor interface for Python files.
// It uses tree-sitter to parse Python source code and extract structured information
// about functions, classes, methods, imports, and their relationships.
type PythonExtractor struct {
	*BaseExtractor
	importParser *PythonImportParser
}

// NewPythonExtractor creates a new Python extractor with initialized parsers.
func NewPythonExtractor() Extractor {
	return &PythonExtractor{
		BaseExtractor: NewBaseExtractor(NewPythonParser(), Python),
		importParser:  NewPythonImportParser(),
	}
}

// Extract parses a Python file and returns structured module information.
// This is the main entry point for extracting all Python constructs from a file.
func (e *PythonExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	// Parse imports using the import parser
	imports, err := e.importParser.ParseImportsFromBytes(content)
	if err != nil {
		return nil, fmt.Errorf("parsing imports: %w", err)
	}

	// Parse the full AST
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()

	// Extract functions, classes, and other constructs
	functions := e.extractFunctions(root, content)
	classes := e.extractClasses(root, content)

	return &types.ModuleInfo{
		Path:      filePath,
		Functions: functions,
		Classes:   classes,
		Imports:   imports,
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
	}, nil
}

// extractFunctions extracts all function definitions from the AST.
// This includes both top-level functions and can optionally include nested functions.
func (e *PythonExtractor) extractFunctions(node *sitter.Node, content []byte) []types.Function {
	var functions []types.Function
	e.walkForFunctions(node, content, &functions, false, false)
	return functions
}

// walkForFunctions recursively walks the AST to find function definitions.
// Parameters:
//   - node: The current AST node being examined
//   - content: The source code content for extracting text
//   - functions: Accumulator for found functions
//   - isMethod: Whether the function is a class method
//   - includeNested: Whether to include nested functions (currently only top-level)
func (e *PythonExtractor) walkForFunctions(node *sitter.Node, content []byte, functions *[]types.Function, isMethod bool, includeNested bool) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "function_definition":
		// Collect decorators from previous siblings
		decorators := e.collectDecoratorsFromSiblings(node, content)
		fn := e.parseFunction(node, content, isMethod, decorators)
		if fn != nil {
			*functions = append(*functions, *fn)
		}
		// Continue walking to find nested functions if needed
		if includeNested {
			for i := 0; i < int(node.ChildCount()); i++ {
				e.walkForFunctions(node.Child(i), content, functions, isMethod, includeNested)
			}
		}
		return // Don't traverse into function bodies by default
	case "class_definition":
		// Don't traverse into classes for top-level functions
		// Class methods will be handled by extractClasses
		return
	}

	// Recursively walk children
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForFunctions(node.Child(i), content, functions, isMethod, includeNested)
	}
}

// collectDecoratorsFromSiblings collects decorator nodes that are siblings before the given node.
// Tree-sitter places decorators as separate decorator nodes before the decorated definition.
func (e *PythonExtractor) collectDecoratorsFromSiblings(node *sitter.Node, content []byte) []string {
	var decorators []string

	parent := node.Parent()
	if parent == nil {
		return decorators
	}

	// Find the index of this node in the parent's children
	nodeIndex := -1
	for i := 0; i < int(parent.ChildCount()); i++ {
		if parent.Child(i) == node {
			nodeIndex = i
			break
		}
	}

	if nodeIndex < 0 {
		return decorators
	}

	// Look backwards for decorator nodes
	for i := nodeIndex - 1; i >= 0; i-- {
		sibling := parent.Child(i)
		if sibling == nil {
			continue
		}

		if sibling.Type() == "decorator" {
			decorator := e.parseDecorator(sibling, content)
			if decorator != "" {
				// Prepend to maintain order (first decorator should be first in list)
				decorators = append([]string{decorator}, decorators...)
			}
		} else if sibling.Type() != "comment" && sibling.Type() != "\"\"\"" {
			// Stop if we hit a non-decorator, non-comment node
			break
		}
	}

	return decorators
}

// parseFunction extracts function information from a function_definition node.
func (e *PythonExtractor) parseFunction(node *sitter.Node, content []byte, isMethod bool, decorators []string) *types.Function {
	var name string
	var params string
	var returnType string
	var docstring string
	isAsync := false

	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "async":
			isAsync = true
		case "def":
		case "identifier":
			name = e.nodeText(child, content)
		case "parameters":
			params = e.nodeText(child, content)
		case "type":
			returnType = e.extractReturnType(child, content)
		case "block":
			docstring = e.extractDocstring(child, content)
		}
	}

	if returnType == "" {
		returnType = e.extractReturnTypeAnnotation(node, content)
	}

	return &types.Function{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		Docstring:  docstring,
		LineNumber: lineNumber,
		IsMethod:   isMethod,
		IsAsync:    isAsync,
		Decorators: decorators,
	}
}

// parseDecorator extracts the decorator expression from a decorator node.
// Handles @decorator, @decorator(args), @module.decorator, etc.
func (e *PythonExtractor) parseDecorator(node *sitter.Node, content []byte) string {
	if node == nil || node.Type() != "decorator" {
		return ""
	}

	// Skip the "@" symbol and get the decorator expression
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		// Skip the "@" token
		if child.Type() == "@" {
			continue
		}

		// Get the decorator expression (identifier, attribute, call, etc.)
		text := e.nodeText(child, content)
		if text != "" {
			return text
		}
	}

	return ""
}

// extractReturnType extracts the return type from a type annotation node.
func (e *PythonExtractor) extractReturnType(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// The type node might contain the return type directly
	text := e.nodeText(node, content)
	if text != "" {
		return strings.TrimSpace(text)
	}

	return ""
}

// extractReturnTypeAnnotation extracts return type from function definition.
// Looks for "-> Type" syntax in the function signature.
func (e *PythonExtractor) extractReturnTypeAnnotation(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for "type" node or check after parameters for -> annotation
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "type" {
			return strings.TrimSpace(e.nodeText(child, content))
		}
	}

	return ""
}

// extractDocstring extracts the docstring from a function or class body.
// Looks for an expression_statement containing a string as the first statement.
func (e *PythonExtractor) extractDocstring(blockNode *sitter.Node, content []byte) string {
	if blockNode == nil {
		return ""
	}

	// Look for expression_statement containing a string as the first statement
	for i := 0; i < int(blockNode.ChildCount()); i++ {
		child := blockNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "expression_statement" {
			// Check if this is a string literal
			for j := 0; j < int(child.ChildCount()); j++ {
				grandchild := child.Child(j)
				if grandchild == nil {
					continue
				}

				// Handle various string types
				switch grandchild.Type() {
				case "string":
					return e.nodeText(grandchild, content)
				case "concatenated_string":
					return e.nodeText(grandchild, content)
				}
			}
		}
	}
	return ""
}

// extractClasses extracts all class definitions from the AST.
func (e *PythonExtractor) extractClasses(node *sitter.Node, content []byte) []types.Class {
	var classes []types.Class
	e.walkForClasses(node, content, &classes)
	return classes
}

// walkForClasses recursively walks the AST to find class definitions.
func (e *PythonExtractor) walkForClasses(node *sitter.Node, content []byte, classes *[]types.Class) {
	if node == nil {
		return
	}

	if node.Type() == "class_definition" {
		decorators := e.collectDecoratorsFromSiblings(node, content)
		class := e.parseClass(node, content, decorators)
		if class != nil {
			*classes = append(*classes, *class)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForClasses(node.Child(i), content, classes)
	}
}

// parseClass extracts class information from a class_definition node.
func (e *PythonExtractor) parseClass(node *sitter.Node, content []byte, decorators []string) *types.Class {
	var name string
	var bases []string
	var docstring string

	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "class":
		case "identifier":
			name = e.nodeText(child, content)
		case "argument_list":
			bases = e.parseBaseClasses(child, content)
		case "block":
			docstring = e.extractDocstring(child, content)
		}
	}

	methods := e.extractMethods(node, content)

	return &types.Class{
		Name:       name,
		Bases:      bases,
		Docstring:  docstring,
		Methods:    methods,
		LineNumber: lineNumber,
	}
}

// parseBaseClasses extracts base class names from an argument_list node.
// Handles simple inheritance (class A(B)) and multiple inheritance (class A(B, C)).
// Also handles module-qualified base classes (class A(module.B)).
func (e *PythonExtractor) parseBaseClasses(node *sitter.Node, content []byte) []string {
	var bases []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		// Skip parentheses
		if child.Type() == "(" || child.Type() == ")" || child.Type() == "," {
			continue
		}

		switch child.Type() {
		case "identifier":
			// Simple class name: class MyClass(BaseClass)
			base := e.nodeText(child, content)
			if base != "" {
				bases = append(bases, base)
			}
		case "attribute":
			// Module-qualified class: class MyClass(module.BaseClass)
			base := e.nodeText(child, content)
			if base != "" {
				bases = append(bases, base)
			}
		case "call":
			// Generic class with arguments: class MyClass(Generic[T])
			// Extract the function name from the call
			base := e.extractCallBase(child, content)
			if base != "" {
				bases = append(bases, base)
			}
		case "subscript":
			// Subscripted type: class MyClass(List[int])
			base := e.nodeText(child, content)
			if base != "" {
				bases = append(bases, base)
			}
		case "list":
			// List of bases: class MyClass([Base1, Base2])
			// This shouldn't normally occur but handle it gracefully
			bases = append(bases, e.nodeText(child, content))
		case "tuple":
			// Tuple of bases: class MyClass((Base1, Base2))
			// Extract individual elements
			bases = append(bases, e.extractTupleBases(child, content)...)
		default:
			// For any other node types, try to extract text
			text := e.nodeText(child, content)
			if text != "" && text != "(" && text != ")" && text != "," {
				bases = append(bases, text)
			}
		}
	}

	return bases
}

// extractCallBase extracts the base name from a call expression.
// For example, "Generic[T]" or "BaseClass(arg)" -> "Generic" or "BaseClass"
func (e *PythonExtractor) extractCallBase(node *sitter.Node, content []byte) string {
	if node == nil || node.Type() != "call" {
		return ""
	}

	// The first child is usually the function being called
	if node.ChildCount() > 0 {
		fnNode := node.Child(0)
		if fnNode != nil {
			return e.nodeText(fnNode, content)
		}
	}

	return ""
}

// extractTupleBases extracts base classes from a tuple node.
func (e *PythonExtractor) extractTupleBases(node *sitter.Node, content []byte) []string {
	var bases []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		// Skip parentheses and commas
		if child.Type() == "(" || child.Type() == ")" || child.Type() == "," {
			continue
		}

		text := e.nodeText(child, content)
		if text != "" {
			bases = append(bases, text)
		}
	}

	return bases
}

// extractMethods extracts all method definitions from a class body.
// Handles instance methods, class methods, static methods, and async methods.
func (e *PythonExtractor) extractMethods(classNode *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	// Find the block node within the class definition
	var blockNode *sitter.Node
	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child != nil && child.Type() == "block" {
			blockNode = child
			break
		}
	}

	if blockNode == nil {
		return methods
	}

	for i := 0; i < int(blockNode.ChildCount()); i++ {
		child := blockNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "function_definition" {
			decorators := e.collectDecoratorsFromSiblings(child, content)
			fn := e.parseFunction(child, content, true, decorators)
			if fn != nil {
				methods = append(methods, *fn)
			}
		}
	}

	return methods
}

// nodeText extracts the text content of a node from the source.
func (e *PythonExtractor) nodeText(node *sitter.Node, content []byte) string {
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

// ExtractFunctions extracts only function definitions from a Python file.
// This is a convenience method for when only function information is needed.
func (e *PythonExtractor) ExtractFunctions(filePath string) ([]types.Function, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()
	functions := e.extractFunctions(root, content)

	return functions, nil
}

// ExtractClasses extracts only class definitions from a Python file.
// This is a convenience method for when only class information is needed.
func (e *PythonExtractor) ExtractClasses(filePath string) ([]types.Class, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()
	classes := e.extractClasses(root, content)

	return classes, nil
}

// ExtractFromBytes extracts module information from Python source code bytes.
// This is useful for testing and when the source is already in memory.
func (e *PythonExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
	// Parse imports using the import parser
	imports, err := e.importParser.ParseImportsFromBytes(content)
	if err != nil {
		return nil, fmt.Errorf("parsing imports: %w", err)
	}

	// Parse the full AST
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing content failed")
	}
	defer tree.Close()

	root := tree.RootNode()

	// Extract functions, classes, and other constructs
	functions := e.extractFunctions(root, content)
	classes := e.extractClasses(root, content)

	return &types.ModuleInfo{
		Path:      filePath,
		Functions: functions,
		Classes:   classes,
		Imports:   imports,
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
	}, nil
}

// IsAsyncFunction checks if a function node represents an async function.
func (e *PythonExtractor) IsAsyncFunction(node *sitter.Node) bool {
	if node == nil || node.Type() != "function_definition" {
		return false
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "async" {
			return true
		}
	}

	return false
}

// HasDecorator checks if a function or class has a specific decorator.
func (e *PythonExtractor) HasDecorator(node *sitter.Node, decoratorName string) bool {
	if node == nil {
		return false
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "decorator" {
			decorator := e.parseDecorator(child, nil)
			if decorator == decoratorName {
				return true
			}
			// Also check for decorator with arguments (e.g., @decorator(args))
			if strings.HasPrefix(decorator, decoratorName+"(") {
				return true
			}
		}
	}

	return false
}

// GetDecorators returns all decorators applied to a function or class.
func (e *PythonExtractor) GetDecorators(node *sitter.Node, content []byte) []string {
	if node == nil {
		return nil
	}

	var decorators []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "decorator" {
			decorator := e.parseDecorator(child, content)
			if decorator != "" {
				decorators = append(decorators, decorator)
			}
		}
	}

	return decorators
}

// NewPythonParser creates a new tree-sitter parser for Python.
// This is a package-level helper function for creating Python parsers.
func NewPythonParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())
	return parser
}
