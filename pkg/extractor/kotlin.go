// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

// KotlinExtractor implements the Extractor interface for Kotlin files.
// It uses tree-sitter to parse Kotlin source code and extract structured information
// about classes, interfaces, functions, imports, data classes, and object declarations.
type KotlinExtractor struct {
	*BaseExtractor
}

// NewKotlinExtractor creates a new Kotlin extractor with initialized parser.
func NewKotlinExtractor() Extractor {
	return &KotlinExtractor{
		BaseExtractor: NewBaseExtractor(NewKotlinParser(), Kotlin),
	}
}

// NewKotlinParser creates a new tree-sitter parser for Kotlin.
func NewKotlinParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(kotlin.GetLanguage())
	return parser
}

// Language returns the language identifier for Kotlin.
func (e *KotlinExtractor) Language() Language {
	return Kotlin
}

// FileExtensions returns the file extensions supported by Kotlin.
func (e *KotlinExtractor) FileExtensions() []string {
	return []string{".kt", ".kts"}
}

// Extract parses a Kotlin file and returns structured module information.
func (e *KotlinExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	return e.ExtractFromBytes(content, filePath)
}

// ExtractFromBytes extracts module information from Kotlin source code bytes.
func (e *KotlinExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
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
	functions := e.extractFunctions(root, content)
	objects := e.extractObjects(root, content)

	// Add object declarations as classes
	for _, obj := range objects {
		classes = append(classes, types.Class{
			Name:       obj.Name,
			Docstring:  obj.Docstring,
			Methods:    obj.Methods,
			LineNumber: obj.LineNumber,
		})
	}

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

// extractImports extracts all import statements from the AST.
func (e *KotlinExtractor) extractImports(node *sitter.Node, content []byte) []types.Import {
	var imports []types.Import
	e.walkForImports(node, content, &imports)
	return imports
}

// walkForImports recursively walks the AST to find import declarations.
func (e *KotlinExtractor) walkForImports(node *sitter.Node, content []byte, imports *[]types.Import) {
	if node == nil {
		return
	}

	if node.Type() == "import_header" {
		imp := e.parseImportHeader(node, content)
		if imp != nil {
			*imports = append(*imports, *imp)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForImports(node.Child(i), content, imports)
	}
}

// parseImportHeader extracts information from an import_header node.
func (e *KotlinExtractor) parseImportHeader(node *sitter.Node, content []byte) *types.Import {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var module string
	var names []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			// Part of the import path
			if module == "" {
				module = e.nodeText(child, content)
			} else {
				module = module + "." + e.nodeText(child, content)
			}
		case "dot":
			// Separator between identifiers
			continue
		case "*":
			// Wildcard import
			names = append(names, "*")
		}
	}

	if module == "" {
		return nil
	}

	return &types.Import{
		Module:     module,
		Names:      names,
		IsFrom:     false,
		LineNumber: lineNumber,
	}
}

// extractClasses extracts all class definitions from the AST.
func (e *KotlinExtractor) extractClasses(node *sitter.Node, content []byte) []types.Class {
	var classes []types.Class
	e.walkForClasses(node, content, &classes)
	return classes
}

// walkForClasses recursively walks the AST to find class definitions.
func (e *KotlinExtractor) walkForClasses(node *sitter.Node, content []byte, classes *[]types.Class) {
	if node == nil {
		return
	}

	// Kotlin class types: class_declaration, data_class_declaration, etc.
	if node.Type() == "class_declaration" || node.Type() == "data_class_declaration" {
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

// parseClassDeclaration extracts information from a class_declaration or data_class_declaration node.
func (e *KotlinExtractor) parseClassDeclaration(node *sitter.Node, content []byte) *types.Class {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var bases []string
	var docstring string
	isDataClass := node.Type() == "data_class_declaration"

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "type_identifier", "simple_identifier":
			// Class name
			if name == "" {
				name = e.nodeText(child, content)
			}
		case "supertype_list":
			// Extends/implements clause
			bases = append(bases, e.extractSupertypeList(child, content)...)
		case "delegation_specifier":
			// Interface/class being extended or implemented
			supertype := e.extractSupertype(child, content)
			if supertype != "" {
				bases = append(bases, supertype)
			}
		case "class_body":
			docstring = e.extractClassDocstring(child, content)
		}
	}

	if name == "" {
		return nil
	}

	methods := e.extractClassMethods(node, content)

	// Add [data] prefix for data classes in docstring
	if isDataClass && docstring == "" {
		docstring = "data class"
	}

	return &types.Class{
		Name:       name,
		Bases:      bases,
		Docstring:  docstring,
		Methods:    methods,
		LineNumber: lineNumber,
	}
}

// extractSupertypeList extracts type names from a supertype list.
func (e *KotlinExtractor) extractSupertypeList(node *sitter.Node, content []byte) []string {
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
		case "delegation_specifier":
			supertype := e.extractSupertype(child, content)
			if supertype != "" {
				types = append(types, supertype)
			}
		}
	}

	return types
}

// extractSupertype extracts a supertype name from a delegation_specifier node.
func (e *KotlinExtractor) extractSupertype(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "type_identifier", "simple_identifier", "user_type":
			return e.nodeText(child, content)
		}
	}

	return ""
}

// extractClassDocstring extracts documentation from a class body.
func (e *KotlinExtractor) extractClassDocstring(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for line comments at the start of the class body
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "line_comment" || child.Type() == "multiline_comment" {
			return e.nodeText(child, content)
		}
	}

	return ""
}

// extractClassMethods extracts all method definitions from a class body.
func (e *KotlinExtractor) extractClassMethods(classNode *sitter.Node, content []byte) []types.Method {
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

		// Function declarations within class body
		if child.Type() == "function_declaration" {
			method := e.parseFunctionDeclaration(child, content)
			if method != nil {
				methods = append(methods, *method)
			}
		}
		// Property declarations (Kotlin properties with getters/setters)
		if child.Type() == "property_declaration" {
			method := e.parsePropertyDeclaration(child, content)
			if method != nil {
				methods = append(methods, *method)
			}
		}
	}

	return methods
}

// extractInterfaces extracts all interface definitions from the AST.
func (e *KotlinExtractor) extractInterfaces(node *sitter.Node, content []byte) []types.Interface {
	var interfaces []types.Interface
	e.walkForInterfaces(node, content, &interfaces)
	return interfaces
}

// walkForInterfaces recursively walks the AST to find interface definitions.
func (e *KotlinExtractor) walkForInterfaces(node *sitter.Node, content []byte, interfaces *[]types.Interface) {
	if node == nil {
		return
	}

	// In Kotlin tree-sitter grammar, interfaces are class_declaration with an "interface" child
	if node.Type() == "class_declaration" && e.isInterfaceDeclaration(node) {
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

// isInterfaceDeclaration checks if a class_declaration node is actually an interface.
func (e *KotlinExtractor) isInterfaceDeclaration(node *sitter.Node) bool {
	if node == nil {
		return false
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "interface" {
			return true
		}
	}
	return false
}

// parseInterfaceDeclaration extracts information from an interface_declaration node.
func (e *KotlinExtractor) parseInterfaceDeclaration(node *sitter.Node, content []byte) *types.Interface {
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
		case "type_identifier", "simple_identifier":
			if name == "" {
				name = e.nodeText(child, content)
			}
		case "supertype_list":
			bases = append(bases, e.extractSupertypeList(child, content)...)
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
func (e *KotlinExtractor) extractInterfaceMethods(interfaceNode *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	// Find the class_body node (interfaces use class_body in tree-sitter-kotlin)
	var interfaceBody *sitter.Node
	for i := 0; i < int(interfaceNode.ChildCount()); i++ {
		child := interfaceNode.Child(i)
		if child != nil && child.Type() == "class_body" {
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

		if child.Type() == "function_declaration" {
			method := e.parseFunctionDeclaration(child, content)
			if method != nil {
				methods = append(methods, *method)
			}
		}
	}

	return methods
}

// ObjectInfo represents a Kotlin object declaration.
type ObjectInfo struct {
	Name       string
	Docstring  string
	Methods    []types.Method
	LineNumber int
}

// extractObjects extracts all object declarations from the AST.
func (e *KotlinExtractor) extractObjects(node *sitter.Node, content []byte) []ObjectInfo {
	var objects []ObjectInfo
	e.walkForObjects(node, content, &objects)
	return objects
}

// walkForObjects recursively walks the AST to find object declarations.
func (e *KotlinExtractor) walkForObjects(node *sitter.Node, content []byte, objects *[]ObjectInfo) {
	if node == nil {
		return
	}

	if node.Type() == "object_declaration" {
		obj := e.parseObjectDeclaration(node, content)
		if obj != nil {
			*objects = append(*objects, *obj)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForObjects(node.Child(i), content, objects)
	}
}

// parseObjectDeclaration extracts information from an object_declaration node.
func (e *KotlinExtractor) parseObjectDeclaration(node *sitter.Node, content []byte) *ObjectInfo {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "type_identifier", "simple_identifier":
			if name == "" {
				name = e.nodeText(child, content)
			}
		}
	}

	if name == "" {
		return nil
	}

	methods := e.extractObjectMethods(node, content)

	return &ObjectInfo{
		Name:       name,
		Methods:    methods,
		LineNumber: lineNumber,
	}
}

// extractObjectMethods extracts methods from an object declaration.
func (e *KotlinExtractor) extractObjectMethods(objectNode *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	var objectBody *sitter.Node
	for i := 0; i < int(objectNode.ChildCount()); i++ {
		child := objectNode.Child(i)
		if child != nil && child.Type() == "class_body" {
			objectBody = child
			break
		}
	}

	if objectBody == nil {
		return methods
	}

	for i := 0; i < int(objectBody.ChildCount()); i++ {
		child := objectBody.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "function_declaration" {
			method := e.parseFunctionDeclaration(child, content)
			if method != nil {
				methods = append(methods, *method)
			}
		}
	}

	return methods
}

// extractFunctions extracts top-level function definitions from the AST.
func (e *KotlinExtractor) extractFunctions(node *sitter.Node, content []byte) []types.Function {
	var functions []types.Function
	e.walkForFunctions(node, content, &functions)
	return functions
}

// walkForFunctions recursively walks the AST to find top-level function definitions.
func (e *KotlinExtractor) walkForFunctions(node *sitter.Node, content []byte, functions *[]types.Function) {
	if node == nil {
		return
	}

	// Only process top-level functions, not those inside classes/objects
	if node.Type() == "function_declaration" {
		// Check if this is a top-level function by checking parent
		parent := node.Parent()
		if parent != nil && (parent.Type() == "source_file" || parent.Type() == "file") {
			fn := e.parseTopLevelFunctionDeclaration(node, content)
			if fn != nil {
				*functions = append(*functions, *fn)
			}
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "function_declaration" {
			// Check if this is top-level
			parent := child.Parent()
			if parent != nil && (parent.Type() == "source_file" || parent.Type() == "file") {
				fn := e.parseTopLevelFunctionDeclaration(child, content)
				if fn != nil {
					*functions = append(*functions, *fn)
				}
			}
		} else if child != nil {
			e.walkForFunctions(child, content, functions)
		}
	}
}

// parseTopLevelFunctionDeclaration extracts information from a top-level function_declaration node.
func (e *KotlinExtractor) parseTopLevelFunctionDeclaration(node *sitter.Node, content []byte) *types.Function {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var params string
	var returnType string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "simple_identifier":
			if name == "" {
				name = e.nodeText(child, content)
			}
		case "function_value_parameters":
			params = e.nodeText(child, content)
		case "type_reference", "user_type", "type_identifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		}
	}

	if name == "" {
		return nil
	}

	return &types.Function{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		LineNumber: lineNumber,
		IsMethod:   false,
	}
}

// parseFunctionDeclaration extracts information from a function_declaration node.
func (e *KotlinExtractor) parseFunctionDeclaration(node *sitter.Node, content []byte) *types.Method {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var params string
	var returnType string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "simple_identifier":
			if name == "" {
				name = e.nodeText(child, content)
			}
		case "function_value_parameters":
			params = e.nodeText(child, content)
		case "type_reference", "user_type", "type_identifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		}
	}

	if name == "" {
		return nil
	}

	return &types.Method{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		LineNumber: lineNumber,
		IsMethod:   true,
	}
}

// parsePropertyDeclaration extracts property information as a method.
func (e *KotlinExtractor) parsePropertyDeclaration(node *sitter.Node, content []byte) *types.Method {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var returnType string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "simple_identifier":
			if name == "" {
				name = e.nodeText(child, content)
			}
		case "type_reference", "user_type", "type_identifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		}
	}

	if name == "" {
		return nil
	}

	return &types.Method{
		Name:       name,
		ReturnType: returnType,
		LineNumber: lineNumber,
		IsMethod:   true,
	}
}

// nodeText extracts the text content of a node from the source.
func (e *KotlinExtractor) nodeText(node *sitter.Node, content []byte) string {
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

// ExtractFunctions extracts only function definitions from a Kotlin file.
func (e *KotlinExtractor) ExtractFunctions(filePath string) ([]types.Function, error) {
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

// ExtractClasses extracts only class definitions from a Kotlin file.
func (e *KotlinExtractor) ExtractClasses(filePath string) ([]types.Class, error) {
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
