// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"strings"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"
)

// CPPExtractor implements the Extractor interface for C++ files.
// It uses tree-sitter to parse C++ source code and extract structured information
// about functions, classes, structs, templates, and includes.
type CPPExtractor struct {
	*BaseExtractor
}

// NewCPPExtractor creates a new C++ extractor with initialized parser.
func NewCPPExtractor() Extractor {
	return &CPPExtractor{
		BaseExtractor: NewBaseExtractor(NewCPPParser(), CPP),
	}
}

// NewCPPParser creates a new tree-sitter parser for C++.
func NewCPPParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(cpp.GetLanguage())
	return parser
}

// Language returns the language identifier for C++.
func (e *CPPExtractor) Language() Language {
	return CPP
}

// FileExtensions returns the file extensions supported by C++.
func (e *CPPExtractor) FileExtensions() []string {
	return []string{".cpp", ".hpp", ".cc", ".hh", ".cxx", ".hxx"}
}

// Extract parses a C++ file and returns structured module information.
func (e *CPPExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	return e.ExtractFromBytes(content, filePath)
}

// ExtractFromBytes extracts module information from C++ source code bytes.
func (e *CPPExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
	// Parse the full AST
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()

	// Extract all constructs
	includes := e.extractIncludes(root, content)
	functions := e.extractFunctions(root, content)
	classes := e.extractClasses(root, content)
	structs := e.extractStructs(root, content)
	templates := e.extractTemplates(root, content)

	// Add template info to classes for representation
	for _, tmpl := range templates {
		classes = append(classes, types.Class{
			Name:       tmpl.Name,
			Docstring:  tmpl.Docstring,
			Methods:    []types.Method{},
			LineNumber: tmpl.LineNumber,
		})
	}

	return &types.ModuleInfo{
		Path:      filePath,
		Functions: functions,
		Classes:   classes,
		Imports:   includes,
		Structs:   structs,
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
	}, nil
}

// extractIncludes extracts all #include directives from the AST.
func (e *CPPExtractor) extractIncludes(node *sitter.Node, content []byte) []types.Import {
	var includes []types.Import
	e.walkForIncludes(node, content, &includes)
	return includes
}

// walkForIncludes recursively walks the AST to find include directives.
func (e *CPPExtractor) walkForIncludes(node *sitter.Node, content []byte, includes *[]types.Import) {
	if node == nil {
		return
	}

	if node.Type() == "preproc_include" {
		imp := e.parseIncludeDirective(node, content)
		if imp != nil {
			*includes = append(*includes, *imp)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForIncludes(node.Child(i), content, includes)
	}
}

// parseIncludeDirective extracts information from a preproc_include node.
func (e *CPPExtractor) parseIncludeDirective(node *sitter.Node, content []byte) *types.Import {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var module string
	var isSystem bool

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "string_literal":
			// Local include: #include "header.hpp"
			module = e.nodeText(child, content)
			module = strings.Trim(module, `"`)
		case "system_lib_string":
			// System include: #include <iostream>
			module = e.nodeText(child, content)
			module = strings.Trim(module, "<>")
			isSystem = true
		}
	}

	if module == "" {
		return nil
	}

	return &types.Import{
		Module:     module,
		Names:      []string{},
		IsFrom:     isSystem,
		LineNumber: lineNumber,
	}
}

// extractFunctions extracts all function definitions from the AST.
func (e *CPPExtractor) extractFunctions(node *sitter.Node, content []byte) []types.Function {
	var functions []types.Function
	e.walkForFunctions(node, content, &functions)
	return functions
}

// walkForFunctions recursively walks the AST to find function definitions.
func (e *CPPExtractor) walkForFunctions(node *sitter.Node, content []byte, functions *[]types.Function) {
	if node == nil {
		return
	}

	// Skip functions inside classes (those are methods)
	if node.Type() == "function_definition" || node.Type() == "template_function" {
		// Check if this is a standalone function (not inside a class)
		if !e.isInsideClass(node) {
			fn := e.parseFunctionDefinition(node, content)
			if fn != nil {
				*functions = append(*functions, *fn)
			}
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForFunctions(node.Child(i), content, functions)
	}
}

// isInsideClass checks if a node is inside a class definition.
func (e *CPPExtractor) isInsideClass(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "class_specifier", "struct_specifier":
			return true
		}
		parent = parent.Parent()
	}
	return false
}

// parseFunctionDefinition extracts information from a function_definition node.
func (e *CPPExtractor) parseFunctionDefinition(node *sitter.Node, content []byte) *types.Function {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var params string
	var returnType string
	var declarator *sitter.Node
	var isTemplate bool

	// Check if this is a template function
	if node.Type() == "template_function" {
		isTemplate = true
		// Find the function_definition inside template_declaration
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "function_definition" {
				declarator = child
				break
			}
		}
	}

	// First pass: find the declarator and return type
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "function_declarator":
			declarator = child
		case "pointer_declarator":
			// Function returning pointer
			declarator = child
		case "reference_declarator":
			// Function returning reference
			declarator = child
		case "primitive_type":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "type_identifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "sized_type_specifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "struct_specifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "class_specifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "scoped_type_identifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "decltype":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "auto":
			if returnType == "" {
				returnType = "auto"
			}
		}
	}

	// Second pass: extract function name and params from declarator
	if declarator != nil {
		e.extractFunctionDeclarator(declarator, content, &name, &params, &returnType)
	}

	if name == "" {
		return nil
	}

	fn := &types.Function{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		LineNumber: lineNumber,
		IsMethod:   false,
	}

	if isTemplate {
		fn.Docstring = "template function"
	}

	return fn
}

// extractFunctionDeclarator extracts name and params from a function declarator.
func (e *CPPExtractor) extractFunctionDeclarator(node *sitter.Node, content []byte, name, params, returnType *string) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "function_definition":
		// Recursively process function definition children
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			e.extractFunctionDeclarator(child, content, name, params, returnType)
		}
	case "function_declarator":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			switch child.Type() {
			case "identifier":
				if *name == "" {
					*name = e.nodeText(child, content)
				}
			case "parenthesized_declarator":
				e.extractFunctionDeclarator(child, content, name, params, returnType)
			case "parameter_list":
				*params = e.nodeText(child, content)
			case "field_identifier":
				// For function definitions with qualified names
				if *name == "" {
					*name = e.nodeText(child, content)
				}
			case "qualified_identifier":
				// Class::method style names
				if *name == "" {
					*name = e.nodeText(child, content)
				}
			case "destructor_name":
				// Destructor
				if *name == "" {
					*name = "~" + e.nodeText(child, content)
				}
			}
		}
	case "pointer_declarator":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			switch child.Type() {
			case "function_declarator":
				e.extractFunctionDeclarator(child, content, name, params, returnType)
			case "pointer_declarator":
				e.extractFunctionDeclarator(child, content, name, params, returnType)
			case "identifier":
				if *name == "" {
					*name = e.nodeText(child, content)
				}
			case "*":
				if *returnType != "" && !strings.HasSuffix(*returnType, "*") {
					*returnType = *returnType + "*"
				}
			}
		}
	case "reference_declarator":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			switch child.Type() {
			case "function_declarator":
				e.extractFunctionDeclarator(child, content, name, params, returnType)
			case "identifier":
				if *name == "" {
					*name = e.nodeText(child, content)
				}
			case "&":
				if *returnType != "" && !strings.HasSuffix(*returnType, "&") {
					*returnType = *returnType + "&"
				}
			}
		}
	case "parenthesized_declarator":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			if child.Type() == "pointer_declarator" || child.Type() == "function_declarator" {
				e.extractFunctionDeclarator(child, content, name, params, returnType)
			}
		}
	}
}

// extractClasses extracts all class definitions from the AST.
func (e *CPPExtractor) extractClasses(node *sitter.Node, content []byte) []types.Class {
	var classes []types.Class
	e.walkForClasses(node, content, &classes)
	return classes
}

// walkForClasses recursively walks the AST to find class definitions.
func (e *CPPExtractor) walkForClasses(node *sitter.Node, content []byte, classes *[]types.Class) {
	if node == nil {
		return
	}

	if node.Type() == "class_specifier" {
		class := e.parseClassSpecifier(node, content)
		if class != nil {
			*classes = append(*classes, *class)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForClasses(node.Child(i), content, classes)
	}
}

// parseClassSpecifier extracts information from a class_specifier node.
func (e *CPPExtractor) parseClassSpecifier(node *sitter.Node, content []byte) *types.Class {
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
		case "type_identifier":
			name = e.nodeText(child, content)
		case "identifier":
			if name == "" {
				name = e.nodeText(child, content)
			}
		case "base_class_clause":
			bases = e.extractBaseClasses(child, content)
		case "field_declaration_list":
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

// extractBaseClasses extracts base class names from a base_class_clause node.
func (e *CPPExtractor) extractBaseClasses(node *sitter.Node, content []byte) []string {
	var bases []string

	if node == nil {
		return bases
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "type_identifier":
			bases = append(bases, e.nodeText(child, content))
		case "scoped_type_identifier":
			bases = append(bases, e.nodeText(child, content))
		case "qualified_identifier":
			bases = append(bases, e.nodeText(child, content))
		}
	}

	return bases
}

// extractClassDocstring extracts documentation from a class body.
func (e *CPPExtractor) extractClassDocstring(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for comment at the start of the class body
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "comment" {
			return e.nodeText(child, content)
		}
	}

	return ""
}

// extractClassMethods extracts method definitions from a class body.
func (e *CPPExtractor) extractClassMethods(classNode *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	// Find the field_declaration_list node
	var classBody *sitter.Node
	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child != nil && child.Type() == "field_declaration_list" {
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

		if child.Type() == "function_definition" {
			method := e.parseFunctionDefinition(child, content)
			if method != nil {
				method.IsMethod = true
				methods = append(methods, *method)
			}
		}
	}

	return methods
}

// extractStructs extracts all struct definitions from the AST.
func (e *CPPExtractor) extractStructs(node *sitter.Node, content []byte) []types.Struct {
	var structs []types.Struct
	e.walkForStructs(node, content, &structs)
	return structs
}

// walkForStructs recursively walks the AST to find struct definitions.
func (e *CPPExtractor) walkForStructs(node *sitter.Node, content []byte, structs *[]types.Struct) {
	if node == nil {
		return
	}

	if node.Type() == "struct_specifier" {
		s := e.parseStructSpecifier(node, content)
		if s != nil {
			*structs = append(*structs, *s)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForStructs(node.Child(i), content, structs)
	}
}

// parseStructSpecifier extracts information from a struct_specifier node.
func (e *CPPExtractor) parseStructSpecifier(node *sitter.Node, content []byte) *types.Struct {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var fields []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "type_identifier":
			name = e.nodeText(child, content)
		case "identifier":
			if name == "" {
				name = e.nodeText(child, content)
			}
		case "field_declaration_list":
			fields = e.extractStructFields(child, content)
		}
	}

	if name == "" && len(fields) == 0 {
		return nil
	}

	return &types.Struct{
		Name:       name,
		Fields:     fields,
		LineNumber: lineNumber,
	}
}

// extractStructFields extracts field information from a field_declaration_list node.
func (e *CPPExtractor) extractStructFields(node *sitter.Node, content []byte) []string {
	var fields []string

	if node == nil {
		return fields
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "field_declaration" {
			field := e.nodeText(child, content)
			if field != "" {
				fields = append(fields, field)
			}
		}
	}

	return fields
}

// TemplateInfo represents a template definition in C++.
type TemplateInfo struct {
	Name       string
	Docstring  string
	LineNumber int
}

// extractTemplates extracts template declarations from the AST.
func (e *CPPExtractor) extractTemplates(node *sitter.Node, content []byte) []TemplateInfo {
	var templates []TemplateInfo
	e.walkForTemplates(node, content, &templates)
	return templates
}

// walkForTemplates recursively walks the AST to find template declarations.
func (e *CPPExtractor) walkForTemplates(node *sitter.Node, content []byte, templates *[]TemplateInfo) {
	if node == nil {
		return
	}

	if node.Type() == "template_declaration" {
		tmpl := e.parseTemplateDeclaration(node, content)
		if tmpl != nil {
			*templates = append(*templates, *tmpl)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForTemplates(node.Child(i), content, templates)
	}
}

// parseTemplateDeclaration extracts information from a template_declaration node.
func (e *CPPExtractor) parseTemplateDeclaration(node *sitter.Node, content []byte) *TemplateInfo {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1

	// Look for the function or class definition inside the template
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "function_definition":
			fn := e.parseFunctionDefinition(child, content)
			if fn != nil {
				return &TemplateInfo{
					Name:       fn.Name,
					LineNumber: lineNumber,
					Docstring:  "template",
				}
			}
		case "class_specifier":
			class := e.parseClassSpecifier(child, content)
			if class != nil {
				return &TemplateInfo{
					Name:       class.Name,
					LineNumber: lineNumber,
					Docstring:  "template class",
				}
			}
		case "struct_specifier":
			s := e.parseStructSpecifier(child, content)
			if s != nil {
				return &TemplateInfo{
					Name:       s.Name,
					LineNumber: lineNumber,
					Docstring:  "template struct",
				}
			}
		}
	}

	return nil
}

// nodeText extracts the text content of a node from the source.
func (e *CPPExtractor) nodeText(node *sitter.Node, content []byte) string {
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

// ExtractFunctions extracts only function definitions from a C++ file.
func (e *CPPExtractor) ExtractFunctions(filePath string) ([]types.Function, error) {
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
	return e.extractFunctions(root, content), nil
}

// ExtractClasses extracts only class definitions from a C++ file.
func (e *CPPExtractor) ExtractClasses(filePath string) ([]types.Class, error) {
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
