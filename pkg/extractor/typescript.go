// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"strings"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TypeScriptExtractor implements the Extractor interface for TypeScript files.
// It uses tree-sitter to parse TypeScript source code and extract structured information
// about functions, classes, interfaces, types, enums, imports, and their relationships.
type TypeScriptExtractor struct {
	*BaseExtractor
	importParser *TypeScriptImportParser
}

// NewTypeScriptExtractor creates a new TypeScript extractor with initialized parsers.
func NewTypeScriptExtractor() Extractor {
	return &TypeScriptExtractor{
		BaseExtractor: NewBaseExtractor(NewTypeScriptParser(), TypeScript),
		importParser:  NewTypeScriptImportParser(),
	}
}

// Language returns the language identifier for TypeScript.
func (e *TypeScriptExtractor) Language() Language {
	return TypeScript
}

// FileExtensions returns the file extensions supported by TypeScript.
func (e *TypeScriptExtractor) FileExtensions() []string {
	return []string{".ts", ".tsx", ".mts", ".cts"}
}

// Extract parses a TypeScript file and returns structured module information.
func (e *TypeScriptExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	// Parse imports using the import parser
	imports, err := e.importParser.ParseImportsFromBytes(content, filePath)
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

	// Extract all constructs
	functions := e.extractFunctions(root, content)
	classes := e.extractClasses(root, content)
	interfaces := e.extractInterfaces(root, content)
	enums := e.extractEnums(root, content)

	return &types.ModuleInfo{
		Path:       filePath,
		Functions:  functions,
		Classes:    classes,
		Interfaces: interfaces,
		Enums:      enums,
		Imports:    imports,
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
	}, nil
}

// extractFunctions extracts all function definitions from the AST.
func (e *TypeScriptExtractor) extractFunctions(node *sitter.Node, content []byte) []types.Function {
	var functions []types.Function
	e.walkForFunctions(node, content, &functions, false)
	return functions
}

// walkForFunctions recursively walks the AST to find function definitions.
func (e *TypeScriptExtractor) walkForFunctions(node *sitter.Node, content []byte, functions *[]types.Function, isMethod bool) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "function_declaration":
		fn := e.parseFunction(node, content, isMethod)
		if fn != nil {
			*functions = append(*functions, *fn)
		}
		return // Don't traverse into function bodies
	case "function":
		// Arrow function assigned to variable: const fn = () => {}
		// Skip here - will be handled via variable_declarator
		return
	case "class_declaration":
		// Don't traverse into classes for top-level functions
		// Class methods will be handled by extractClasses
		return
	}

	// Recursively walk children
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForFunctions(node.Child(i), content, functions, isMethod)
	}
}

// parseFunction extracts function information from a function_declaration node.
func (e *TypeScriptExtractor) parseFunction(node *sitter.Node, content []byte, isMethod bool) *types.Function {
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
		case "function":
		case "identifier":
			name = e.nodeText(child, content)
		case "property_signature":
			// Check for property name in property_signature
			for j := 0; j < int(child.ChildCount()); j++ {
				propChild := child.Child(j)
				if propChild != nil && propChild.Type() == "property_identifier" {
					name = e.nodeText(propChild, content)
					break
				}
			}
		case "parameters":
			params = e.nodeText(child, content)
		case "return_type":
			returnType = e.extractReturnType(child, content)
		case "statement_block":
			docstring = e.extractDocstring(child, content)
		}
	}

	// Also check for export statement that wraps function
	if name == "" {
		name = e.extractNameFromExport(node, content)
	}

	return &types.Function{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		Docstring:  docstring,
		LineNumber: lineNumber,
		IsMethod:   isMethod,
		IsAsync:    isAsync,
		Decorators: nil,
	}
}

// extractNameFromExport extracts function name from export statement.
func (e *TypeScriptExtractor) extractNameFromExport(node *sitter.Node, content []byte) string {
	parent := node.Parent()
	if parent == nil {
		return ""
	}

	if parent.Type() == "export_statement" {
		for i := 0; i < int(parent.ChildCount()); i++ {
			child := parent.Child(i)
			if child != nil && child.Type() == "function_declaration" {
				// Recurse to get name
				return e.extractFunctionName(child, content)
			}
		}
	}
	return ""
}

// extractFunctionName extracts function name from function node.
func (e *TypeScriptExtractor) extractFunctionName(node *sitter.Node, content []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "identifier" {
			return e.nodeText(child, content)
		}
	}
	return ""
}

// extractReturnType extracts the return type from a return_type node.
func (e *TypeScriptExtractor) extractReturnType(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Get the type after the colon
	text := e.nodeText(node, content)
	if text != "" {
		// Remove the colon prefix if present
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, ":") {
			text = strings.TrimPrefix(text, ":")
			text = strings.TrimSpace(text)
		}
		return text
	}

	return ""
}

// extractDocstring extracts the docstring from a function or class body.
// In TypeScript, JSDoc comments start with /** and end with */
func (e *TypeScriptExtractor) extractDocstring(blockNode *sitter.Node, content []byte) string {
	if blockNode == nil {
		return ""
	}

	// Get parent to check for comments before this block
	parent := blockNode.Parent()
	if parent == nil {
		return ""
	}

	// Look for comments that are JSDoc style before this node
	for i := 0; i < int(parent.ChildCount()); i++ {
		child := parent.Child(i)
		if child == nil {
			continue
		}

		if child == blockNode {
			// Found the block, stop looking
			break
		}

		if child.Type() == "comment" {
			text := e.nodeText(child, content)
			if strings.HasPrefix(text, "/**") {
				return text
			}
		}
	}

	return ""
}

// extractClasses extracts all class definitions from the AST.
func (e *TypeScriptExtractor) extractClasses(node *sitter.Node, content []byte) []types.Class {
	var classes []types.Class
	e.walkForClasses(node, content, &classes)
	return classes
}

// walkForClasses recursively walks the AST to find class definitions.
func (e *TypeScriptExtractor) walkForClasses(node *sitter.Node, content []byte, classes *[]types.Class) {
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
func (e *TypeScriptExtractor) parseClass(node *sitter.Node, content []byte) *types.Class {
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
		case "type_identifier":
			name = e.nodeText(child, content)
		case "identifier":
			// Some parsers use identifier instead of type_identifier
			if name == "" {
				name = e.nodeText(child, content)
			}
		case "class_heritage":
			bases = e.parseClassHeritage(child, content)
		case "class_body":
			docstring = e.extractDocstringFromClassBody(child, content)
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

// parseClassHeritage extracts base class names from class_heritage node.
func (e *TypeScriptExtractor) parseClassHeritage(node *sitter.Node, content []byte) []string {
	var bases []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "extends":
			// Next should be the base class
		case "identifier":
			base := e.nodeText(child, content)
			if base != "" {
				bases = append(bases, base)
			}
		case "member_expression":
			// Module-qualified base class: extends BaseClass
			bases = append(bases, e.nodeText(child, content))
		case "type_identifier":
			bases = append(bases, e.nodeText(child, content))
		}
	}

	return bases
}

// extractDocstringFromClassBody extracts docstring from class body.
// Looks for JSDoc comments before the class.
func (e *TypeScriptExtractor) extractDocstringFromClassBody(node *sitter.Node, content []byte) string {
	parent := node.Parent()
	if parent == nil {
		return ""
	}

	// Look for comments before the class declaration
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
			if strings.HasPrefix(text, "/**") {
				return text
			}
		}
	}

	return ""
}

// extractMethods extracts all method definitions from a class body.
func (e *TypeScriptExtractor) extractMethods(classNode *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	// Find the class_body node within the class declaration
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

		// method_definition in TypeScript
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
func (e *TypeScriptExtractor) parseMethod(node *sitter.Node, content []byte) types.Method {
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
		case "property_name", "property_identifier":
			name = e.nodeText(child, content)
		case "parameters", "formal_parameters":
			params = e.nodeText(child, content)
		case "return_type":
			returnType = e.extractReturnType(child, content)
		case "type_annotation":
			// Also check type_annotation for return type
			returnType = e.nodeText(child, content)
		}
	}

	return types.Method{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		LineNumber: lineNumber,
		IsMethod:   true,
		IsAsync:    isAsync,
	}
}

// extractInterfaces extracts all interface definitions from the AST.
func (e *TypeScriptExtractor) extractInterfaces(node *sitter.Node, content []byte) []types.Interface {
	var interfaces []types.Interface
	e.walkForInterfaces(node, content, &interfaces)
	return interfaces
}

// walkForInterfaces recursively walks the AST to find interface definitions.
func (e *TypeScriptExtractor) walkForInterfaces(node *sitter.Node, content []byte, interfaces *[]types.Interface) {
	if node == nil {
		return
	}

	if node.Type() == "interface_declaration" {
		iface := e.parseInterface(node, content)
		if iface != nil {
			*interfaces = append(*interfaces, *iface)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForInterfaces(node.Child(i), content, interfaces)
	}
}

// parseInterface extracts interface information from an interface_declaration node.
func (e *TypeScriptExtractor) parseInterface(node *sitter.Node, content []byte) *types.Interface {
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
		case "interface":
		case "type_identifier":
			name = e.nodeText(child, content)
		case "class_heritage":
			bases = e.parseInterfaceHeritage(child, content)
		case "object_type":
			// Interface body - extract methods
			// For now, we don't extract interface methods
		}
	}

	return &types.Interface{
		Name:       name,
		Bases:      bases,
		Docstring:  docstring,
		Methods:    nil,
		LineNumber: lineNumber,
	}
}

// parseInterfaceHeritage extracts extended interfaces.
func (e *TypeScriptExtractor) parseInterfaceHeritage(node *sitter.Node, content []byte) []string {
	var bases []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "extends":
		case "type_identifier":
			base := e.nodeText(child, content)
			if base != "" {
				bases = append(bases, base)
			}
		}
	}

	return bases
}

// extractEnums extracts all enum definitions from the AST.
func (e *TypeScriptExtractor) extractEnums(node *sitter.Node, content []byte) []types.Enum {
	var enums []types.Enum
	e.walkForEnums(node, content, &enums)
	return enums
}

// walkForEnums recursively walks the AST to find enum definitions.
func (e *TypeScriptExtractor) walkForEnums(node *sitter.Node, content []byte, enums *[]types.Enum) {
	if node == nil {
		return
	}

	if node.Type() == "enum_declaration" {
		enum := e.parseEnum(node, content)
		if enum != nil {
			*enums = append(*enums, *enum)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForEnums(node.Child(i), content, enums)
	}
}

// parseEnum extracts enum information from an enum_declaration node.
func (e *TypeScriptExtractor) parseEnum(node *sitter.Node, content []byte) *types.Enum {
	var name string
	var variants []string

	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "enum":
		case "identifier":
			name = e.nodeText(child, content)
		case "enum_body":
			variants = e.parseEnumBody(child, content)
		}
	}

	return &types.Enum{
		Name:       name,
		Variants:   variants,
		LineNumber: lineNumber,
	}
}

// parseEnumBody extracts enum variants from enum_body node.
func (e *TypeScriptExtractor) parseEnumBody(node *sitter.Node, content []byte) []string {
	var variants []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "enum_member" {
			for j := 0; j < int(child.ChildCount()); j++ {
				memberChild := child.Child(j)
				if memberChild != nil && (memberChild.Type() == "property_identifier" || memberChild.Type() == "identifier") {
					variant := e.nodeText(memberChild, content)
					if variant != "" {
						variants = append(variants, variant)
					}
					break
				}
			}
		}
	}

	return variants
}

// extractTypeAliases extracts type alias declarations (type X = Y).
func (e *TypeScriptExtractor) extractTypeAliases(node *sitter.Node, content []byte) []string {
	var typeAliases []string
	e.walkForTypeAliases(node, content, &typeAliases)
	return typeAliases
}

// walkForTypeAliases recursively walks the AST to find type alias declarations.
func (e *TypeScriptExtractor) walkForTypeAliases(node *sitter.Node, content []byte, typeAliases *[]string) {
	if node == nil {
		return
	}

	if node.Type() == "type_alias_declaration" {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "type_identifier" {
				alias := e.nodeText(child, content)
				if alias != "" {
					*typeAliases = append(*typeAliases, alias)
				}
				break
			}
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForTypeAliases(node.Child(i), content, typeAliases)
	}
}

// nodeText extracts the text content of a node from the source.
func (e *TypeScriptExtractor) nodeText(node *sitter.Node, content []byte) string {
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

// ExtractFunctions extracts only function definitions from a TypeScript file.
func (e *TypeScriptExtractor) ExtractFunctions(filePath string) ([]types.Function, error) {
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

// ExtractClasses extracts only class definitions from a TypeScript file.
func (e *TypeScriptExtractor) ExtractClasses(filePath string) ([]types.Class, error) {
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

// ExtractFromBytes extracts module information from TypeScript source code bytes.
func (e *TypeScriptExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
	// Parse imports using the import parser
	imports, err := e.importParser.ParseImportsFromBytes(content, filePath)
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

	// Extract all constructs
	functions := e.extractFunctions(root, content)
	classes := e.extractClasses(root, content)
	interfaces := e.extractInterfaces(root, content)
	enums := e.extractEnums(root, content)

	return &types.ModuleInfo{
		Path:       filePath,
		Functions:  functions,
		Classes:    classes,
		Interfaces: interfaces,
		Enums:      enums,
		Imports:    imports,
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
	}, nil
}

func NewTypeScriptParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())
	return parser
}
