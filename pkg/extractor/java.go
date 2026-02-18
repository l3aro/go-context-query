// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"sync"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
)

// javaParserPool is a pool of reusable tree-sitter parsers for Java.
var javaParserPool = sync.Pool{
	New: func() interface{} {
		parser := sitter.NewParser()
		parser.SetLanguage(java.GetLanguage())
		return parser
	},
}

// JavaExtractor implements the Extractor interface for Java files.
// It uses tree-sitter to parse Java source code and extract structured information
// about classes, interfaces, methods, imports, and package declarations.
type JavaExtractor struct {
	*BaseExtractor
}

// NewJavaExtractor creates a new Java extractor with initialized parser.
func NewJavaExtractor() Extractor {
	return &JavaExtractor{
		BaseExtractor: NewBaseExtractor(NewJavaParser(), Java),
	}
}

func NewJavaParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(java.GetLanguage())
	return parser
}

// Language returns the language identifier for Java.
func (e *JavaExtractor) Language() Language {
	return Java
}

// FileExtensions returns the file extensions supported by Java.
func (e *JavaExtractor) FileExtensions() []string {
	return []string{".java"}
}

// Extract parses a Java file and returns structured module information.
func (e *JavaExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	return e.ExtractFromBytes(content, filePath)
}

// ExtractFromBytes extracts module information from Java source code bytes.
func (e *JavaExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
	// Parse the full AST
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()

	// Extract all constructs
	imports := e.extractImports(root, content)
	classes := e.extractClasses(root, content)
	interfaces := e.extractInterfaces(root, content)

	// Extract standalone functions (not methods) from the file
	functions := e.extractFunctions(root, content)

	return &types.ModuleInfo{
		Path:       filePath,
		Functions:  functions,
		Classes:    classes,
		Imports:    imports,
		Interfaces: interfaces,
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
	}, nil
}

// extractPackage extracts the package declaration from the AST.
func (e *JavaExtractor) extractPackage(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	if node.Type() == "package_declaration" {
		// The package name is typically in an identifier or scoped_identifier node
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			switch child.Type() {
			case "identifier", "scoped_identifier":
				return e.nodeText(child, content)
			}
		}
	}

	// Recursively search for package_declaration
	for i := 0; i < int(node.ChildCount()); i++ {
		if pkg := e.extractPackage(node.Child(i), content); pkg != "" {
			return pkg
		}
	}

	return ""
}

// extractImports extracts all import statements from the AST.
func (e *JavaExtractor) extractImports(node *sitter.Node, content []byte) []types.Import {
	var imports []types.Import
	e.walkForImports(node, content, &imports)
	return imports
}

// walkForImports recursively walks the AST to find import declarations.
func (e *JavaExtractor) walkForImports(node *sitter.Node, content []byte, imports *[]types.Import) {
	if node == nil {
		return
	}

	if node.Type() == "import_declaration" {
		imp := e.parseImportDeclaration(node, content)
		if imp != nil {
			*imports = append(*imports, *imp)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForImports(node.Child(i), content, imports)
	}
}

// parseImportDeclaration extracts information from an import_declaration node.
func (e *JavaExtractor) parseImportDeclaration(node *sitter.Node, content []byte) *types.Import {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	isStatic := false
	var module string
	var names []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "static":
			isStatic = true
		case "identifier", "scoped_identifier":
			// This is the import path
			module = e.nodeText(child, content)
		case "asterisk":
			// Wildcard import: import java.util.*
			names = append(names, "*")
		}
	}

	if module == "" {
		return nil
	}

	return &types.Import{
		Module:     module,
		Names:      names,
		IsFrom:     isStatic,
		LineNumber: lineNumber,
	}
}

// extractClasses extracts all class definitions from the AST.
func (e *JavaExtractor) extractClasses(node *sitter.Node, content []byte) []types.Class {
	var classes []types.Class
	e.walkForClasses(node, content, &classes)
	return classes
}

// walkForClasses recursively walks the AST to find class definitions.
func (e *JavaExtractor) walkForClasses(node *sitter.Node, content []byte, classes *[]types.Class) {
	if node == nil {
		return
	}

	if node.Type() == "class_declaration" {
		class := e.parseClassDeclaration(node, content)
		if class != nil {
			*classes = append(*classes, *class)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForClasses(node.Child(i), content, classes)
	}
}

// parseClassDeclaration extracts information from a class_declaration node.
func (e *JavaExtractor) parseClassDeclaration(node *sitter.Node, content []byte) *types.Class {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var bases []string
	var docstring string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			name = e.nodeText(child, content)
		case "super_interfaces":
			// implements clause
			bases = append(bases, e.extractTypeList(child, content)...)
		case "superclass":
			// extends clause
			bases = append(bases, e.extractTypeList(child, content)...)
		case "class_body":
			docstring = e.extractClassDocstring(child, content)
		}
	}

	if name == "" {
		return nil
	}

	methods := e.extractClassMethods(node, content)

	return &types.Class{
		Name:       name,
		Bases:      bases,
		Docstring:  docstring,
		Methods:    methods,
		LineNumber: lineNumber,
	}
}

// extractTypeList extracts type names from a type list node (superclass, super_interfaces).
func (e *JavaExtractor) extractTypeList(node *sitter.Node, content []byte) []string {
	var types []string

	if node == nil {
		return types
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "type_identifier":
			types = append(types, e.nodeText(child, content))
		case "scoped_type_identifier":
			types = append(types, e.nodeText(child, content))
		case "type_list":
			types = append(types, e.extractTypeList(child, content)...)
		}
	}

	return types
}

// extractClassDocstring extracts documentation from a class body.
func (e *JavaExtractor) extractClassDocstring(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for block comment at the start of the class body
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "block_comment" {
			return e.nodeText(child, content)
		}
	}

	return ""
}

// extractClassMethods extracts all method definitions from a class body.
func (e *JavaExtractor) extractClassMethods(classNode *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	// Find the class_body node
	var classBody *sitter.Node
	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child != nil && child.Type() == "class_body" {
			classBody = child
			break
		}
	}

	if classBody == nil {
		return methods
	}

	for i := 0; i < int(classBody.ChildCount()); i++ {
		child := classBody.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "method_declaration" || child.Type() == "constructor_declaration" {
			method := e.parseMethodDeclaration(child, content)
			if method != nil {
				methods = append(methods, *method)
			}
		}
	}

	return methods
}

// parseMethodDeclaration extracts information from a method_declaration or constructor_declaration node.
func (e *JavaExtractor) parseMethodDeclaration(node *sitter.Node, content []byte) *types.Method {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var params string
	var returnType string
	var docstring string
	isConstructor := node.Type() == "constructor_declaration"

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			name = e.nodeText(child, content)
		case "formal_parameters":
			params = e.nodeText(child, content)
		case "void_type", "type_identifier", "scoped_type_identifier":
			returnType = e.nodeText(child, content)
		case "array_type":
			returnType = e.nodeText(child, content)
		case "generic_type":
			returnType = e.nodeText(child, content)
		case "block":
			docstring = e.extractMethodDocstring(child, content)
		}
	}

	if name == "" {
		return nil
	}

	// Constructors don't have a return type in the traditional sense
	if isConstructor {
		returnType = "constructor"
	}

	return &types.Method{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		Docstring:  docstring,
		LineNumber: lineNumber,
	}
}

// extractMethodDocstring extracts documentation from a method body.
func (e *JavaExtractor) extractMethodDocstring(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for block comment at the start of the method body
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "block_comment" {
			return e.nodeText(child, content)
		}
	}

	return ""
}

// extractInterfaces extracts all interface definitions from the AST.
func (e *JavaExtractor) extractInterfaces(node *sitter.Node, content []byte) []types.Interface {
	var interfaces []types.Interface
	e.walkForInterfaces(node, content, &interfaces)
	return interfaces
}

// walkForInterfaces recursively walks the AST to find interface definitions.
func (e *JavaExtractor) walkForInterfaces(node *sitter.Node, content []byte, interfaces *[]types.Interface) {
	if node == nil {
		return
	}

	if node.Type() == "interface_declaration" {
		iface := e.parseInterfaceDeclaration(node, content)
		if iface != nil {
			*interfaces = append(*interfaces, *iface)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForInterfaces(node.Child(i), content, interfaces)
	}
}

// parseInterfaceDeclaration extracts information from an interface_declaration node.
func (e *JavaExtractor) parseInterfaceDeclaration(node *sitter.Node, content []byte) *types.Interface {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var bases []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			name = e.nodeText(child, content)
		case "extends_interfaces":
			bases = append(bases, e.extractTypeList(child, content)...)
		}
	}

	if name == "" {
		return nil
	}

	methods := e.extractInterfaceMethods(node, content)

	return &types.Interface{
		Name:       name,
		Bases:      bases,
		Methods:    methods,
		LineNumber: lineNumber,
	}
}

// extractInterfaceMethods extracts method signatures from an interface body.
func (e *JavaExtractor) extractInterfaceMethods(interfaceNode *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	// Find the interface_body node
	var interfaceBody *sitter.Node
	for i := 0; i < int(interfaceNode.ChildCount()); i++ {
		child := interfaceNode.Child(i)
		if child != nil && child.Type() == "interface_body" {
			interfaceBody = child
			break
		}
	}

	if interfaceBody == nil {
		return methods
	}

	for i := 0; i < int(interfaceBody.ChildCount()); i++ {
		child := interfaceBody.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "method_declaration" || child.Type() == "abstract_method_declaration" {
			method := e.parseMethodDeclaration(child, content)
			if method != nil {
				methods = append(methods, *method)
			}
		}
	}

	return methods
}

// extractFunctions extracts standalone functions (not methods) from the AST.
// In Java, standalone functions are rare but can exist in some contexts.
func (e *JavaExtractor) extractFunctions(node *sitter.Node, content []byte) []types.Function {
	// Java doesn't have standalone functions in the traditional sense
	// All functions are methods within classes
	return []types.Function{}
}

// nodeText extracts the text content of a node from the source.
func (e *JavaExtractor) nodeText(node *sitter.Node, content []byte) string {
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

// ExtractFunctions extracts only function/method definitions from a Java file.
func (e *JavaExtractor) ExtractFunctions(filePath string) ([]types.Function, error) {
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

	// Collect all methods from classes
	classes := e.extractClasses(root, content)
	var allMethods []types.Function
	for _, class := range classes {
		for _, method := range class.Methods {
			allMethods = append(allMethods, method)
		}
	}

	return allMethods, nil
}

// ExtractClasses extracts only class definitions from a Java file.
func (e *JavaExtractor) ExtractClasses(filePath string) ([]types.Class, error) {
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
	return e.extractClasses(root, content), nil
}

// GetPackageName extracts the package name from a Java file.
func (e *JavaExtractor) GetPackageName(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("reading file %s: %w", filePath, err)
	}

	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return "", fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()
	return e.extractPackage(root, content), nil
}
