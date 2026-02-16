// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"os"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// JavaScriptExtractor implements the Extractor interface for JavaScript files.
// It uses tree-sitter to parse JavaScript source code and extract structured information.
// Note: This extractor uses the TypeScript parser since JavaScript is a subset of TypeScript.
type JavaScriptExtractor struct {
	*BaseExtractor
}

// NewJavaScriptExtractor creates a new JavaScript extractor with initialized parser.
func NewJavaScriptExtractor() Extractor {
	return &JavaScriptExtractor{
		BaseExtractor: NewBaseExtractor(NewJavaScriptParser(), JavaScript),
	}
}

// Language returns the language identifier for JavaScript.
func (e *JavaScriptExtractor) Language() Language {
	return JavaScript
}

// FileExtensions returns the file extensions supported by JavaScript.
func (e *JavaScriptExtractor) FileExtensions() []string {
	return []string{".js", ".jsx", ".mjs", ".cjs"}
}

// Extract parses a JavaScript file and returns structured module information.
func (e *JavaScriptExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Parse the full AST using TypeScript parser (JavaScript is a subset)
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, err
	}
	defer tree.Close()

	root := tree.RootNode()

	// Extract constructs similar to TypeScript but simpler
	functions := e.extractFunctions(root, content)
	classes := e.extractClasses(root, content)

	return &types.ModuleInfo{
		Path:      filePath,
		Functions: functions,
		Classes:   classes,
		Imports:   []types.Import{},
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
	}, nil
}

// extractFunctions extracts function definitions from the AST.
func (e *JavaScriptExtractor) extractFunctions(node *sitter.Node, content []byte) []types.Function {
	var functions []types.Function
	e.walkForFunctions(node, content, &functions, false)
	return functions
}

// walkForFunctions recursively walks the AST to find function definitions.
func (e *JavaScriptExtractor) walkForFunctions(node *sitter.Node, content []byte, functions *[]types.Function, isMethod bool) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "function_declaration":
		fn := e.parseFunction(node, content, isMethod)
		if fn != nil {
			*functions = append(*functions, *fn)
		}
		return
	case "class_declaration":
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForFunctions(node.Child(i), content, functions, isMethod)
	}
}

// parseFunction extracts function information from a function_declaration node.
func (e *JavaScriptExtractor) parseFunction(node *sitter.Node, content []byte, isMethod bool) *types.Function {
	var name string
	var params string
	var returnType string
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
		case "identifier":
			name = e.nodeText(child, content)
		case "parameters":
			params = e.nodeText(child, content)
		case "return_type":
			returnType = e.nodeText(child, content)
		}
	}

	return &types.Function{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		LineNumber: lineNumber,
		IsMethod:   isMethod,
		IsAsync:    isAsync,
	}
}

// extractClasses extracts class definitions from the AST.
func (e *JavaScriptExtractor) extractClasses(node *sitter.Node, content []byte) []types.Class {
	var classes []types.Class
	e.walkForClasses(node, content, &classes)
	return classes
}

// walkForClasses recursively walks the AST to find class definitions.
func (e *JavaScriptExtractor) walkForClasses(node *sitter.Node, content []byte, classes *[]types.Class) {
	if node == nil {
		return
	}

	if node.Type() == "class_declaration" {
		class := e.parseClass(node, content)
		if class != nil {
			*classes = append(*classes, *class)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForClasses(node.Child(i), content, classes)
	}
}

// parseClass extracts class information from a class_declaration node.
func (e *JavaScriptExtractor) parseClass(node *sitter.Node, content []byte) *types.Class {
	var name string
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
		case "class_body":
			// Look for comments before the class
			docstring = e.extractDocstring(node, content)
		}
	}

	methods := e.extractMethods(node, content)

	return &types.Class{
		Name:       name,
		Docstring:  docstring,
		Methods:    methods,
		LineNumber: lineNumber,
	}
}

// extractMethods extracts method definitions from a class body.
func (e *JavaScriptExtractor) extractMethods(classNode *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	var bodyNode *sitter.Node
	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child != nil && child.Type() == "class_body" {
			bodyNode = child
			break
		}
	}

	if bodyNode == nil {
		return methods
	}

	for i := 0; i < int(bodyNode.ChildCount()); i++ {
		child := bodyNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "method_definition" {
			method := e.parseMethod(child, content)
			if method.Name != "" {
				methods = append(methods, method)
			}
		}
	}

	return methods
}

// parseMethod extracts method information from a method_definition node.
func (e *JavaScriptExtractor) parseMethod(node *sitter.Node, content []byte) types.Method {
	var name string
	var params string
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
		case "property_name":
			name = e.nodeText(child, content)
		case "parameters":
			params = e.nodeText(child, content)
		}
	}

	return types.Method{
		Name:       name,
		Params:     params,
		LineNumber: lineNumber,
		IsMethod:   true,
		IsAsync:    isAsync,
	}
}

// extractDocstring extracts JSDoc comments before a class.
func (e *JavaScriptExtractor) extractDocstring(node *sitter.Node, content []byte) string {
	parent := node.Parent()
	if parent == nil {
		return ""
	}

	for i := 0; i < int(parent.ChildCount()); i++ {
		child := parent.Child(i)
		if child == nil {
			continue
		}

		if child == node {
			break
		}

		if child.Type() == "comment" {
			text := e.nodeText(child, content)
			if len(text) > 0 && text[0:2] == "//" {
				return text
			}
		}
	}

	return ""
}

// nodeText extracts the text content of a node from the source.
func (e *JavaScriptExtractor) nodeText(node *sitter.Node, content []byte) string {
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

// NewJavaScriptParser creates a new tree-sitter parser for JavaScript.
func NewJavaScriptParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())
	return parser
}
