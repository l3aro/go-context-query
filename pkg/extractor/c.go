// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
)

// cParserPool is a pool of reusable tree-sitter parsers for C.
var cParserPool = sync.Pool{
	New: func() interface{} {
		parser := sitter.NewParser()
		parser.SetLanguage(c.GetLanguage())
		return parser
	},
}

// CExtractor implements the Extractor interface for C files.
// It uses tree-sitter to parse C source code and extract structured information
// about functions, structs, includes, and typedefs.
type CExtractor struct {
	*BaseExtractor
}

// NewCExtractor creates a new C extractor with initialized parser.
func NewCExtractor() Extractor {
	return &CExtractor{
		BaseExtractor: NewBaseExtractor(NewCParser(), C),
	}
}

func NewCParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(c.GetLanguage())
	return parser
}

// Language returns the language identifier for C.
func (e *CExtractor) Language() Language {
	return C
}

// FileExtensions returns the file extensions supported by C.
func (e *CExtractor) FileExtensions() []string {
	return []string{".c", ".h"}
}

// Extract parses a C file and returns structured module information.
func (e *CExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	return e.ExtractFromBytes(content, filePath)
}

// ExtractFromBytes extracts module information from C source code bytes.
func (e *CExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
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
	structs := e.extractStructs(root, content)
	typedefs := e.extractTypedefs(root, content)

	// Convert typedefs to classes for representation
	classes := make([]types.Class, 0, len(typedefs))
	for _, td := range typedefs {
		classes = append(classes, types.Class{
			Name:       td.Name,
			Docstring:  td.Docstring,
			Methods:    []types.Method{},
			LineNumber: td.LineNumber,
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
func (e *CExtractor) extractIncludes(node *sitter.Node, content []byte) []types.Import {
	var includes []types.Import
	e.walkForIncludes(node, content, &includes)
	return includes
}

// walkForIncludes recursively walks the AST to find include directives.
func (e *CExtractor) walkForIncludes(node *sitter.Node, content []byte, includes *[]types.Import) {
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
func (e *CExtractor) parseIncludeDirective(node *sitter.Node, content []byte) *types.Import {
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
			// Local include: #include "header.h"
			module = e.nodeText(child, content)
			module = strings.Trim(module, `"`)
		case "system_lib_string":
			// System include: #include <stdio.h>
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
func (e *CExtractor) extractFunctions(node *sitter.Node, content []byte) []types.Function {
	var functions []types.Function
	e.walkForFunctions(node, content, &functions)
	return functions
}

// walkForFunctions recursively walks the AST to find function definitions.
func (e *CExtractor) walkForFunctions(node *sitter.Node, content []byte, functions *[]types.Function) {
	if node == nil {
		return
	}

	if node.Type() == "function_definition" {
		fn := e.parseFunctionDefinition(node, content)
		if fn != nil {
			*functions = append(*functions, *fn)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForFunctions(node.Child(i), content, functions)
	}
}

// parseFunctionDefinition extracts information from a function_definition node.
func (e *CExtractor) parseFunctionDefinition(node *sitter.Node, content []byte) *types.Function {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var params string
	var returnType string
	var declarator *sitter.Node

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
			// Function returning pointer: int *func()
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
			// Types like unsigned int, long long, etc.
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "struct_specifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "enum_specifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "typedef_type":
			if returnType == "" {
				returnType = e.nodeText(child, content)
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

	return &types.Function{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		LineNumber: lineNumber,
		IsMethod:   false,
	}
}

// extractFunctionDeclarator extracts name and params from a function declarator.
func (e *CExtractor) extractFunctionDeclarator(node *sitter.Node, content []byte, name, params, returnType *string) {
	if node == nil {
		return
	}

	switch node.Type() {
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
				// Handle complex declarators
				e.extractFunctionDeclarator(child, content, name, params, returnType)
			case "abstract_function_declarator":
				// Function pointer parameters
			case "parameter_list":
				*params = e.nodeText(child, content)
			}
		}
	case "pointer_declarator":
		// Handle pointer return types
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			switch child.Type() {
			case "function_declarator":
				e.extractFunctionDeclarator(child, content, name, params, returnType)
			case "pointer_declarator":
				// Nested pointer
				e.extractFunctionDeclarator(child, content, name, params, returnType)
			case "identifier":
				if *name == "" {
					*name = e.nodeText(child, content)
				}
			case "*":
				// Pointer marker
				if *returnType != "" && !strings.HasSuffix(*returnType, "*") {
					*returnType = *returnType + "*"
				}
			}
		}
	case "parenthesized_declarator":
		// Declarators in parentheses
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

// extractStructs extracts all struct definitions from the AST.
func (e *CExtractor) extractStructs(node *sitter.Node, content []byte) []types.Struct {
	var structs []types.Struct
	e.walkForStructs(node, content, &structs)
	return structs
}

// walkForStructs recursively walks the AST to find struct definitions.
func (e *CExtractor) walkForStructs(node *sitter.Node, content []byte, structs *[]types.Struct) {
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
func (e *CExtractor) parseStructSpecifier(node *sitter.Node, content []byte) *types.Struct {
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

	// Only return if we have a named struct with a body
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
func (e *CExtractor) extractStructFields(node *sitter.Node, content []byte) []string {
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

// TypedefInfo represents a typedef definition in C.
type TypedefInfo struct {
	Name       string
	Underlying string
	Docstring  string
	LineNumber int
}

// extractTypedefs extracts all typedef definitions from the AST.
func (e *CExtractor) extractTypedefs(node *sitter.Node, content []byte) []TypedefInfo {
	var typedefs []TypedefInfo
	e.walkForTypedefs(node, content, &typedefs)
	return typedefs
}

// walkForTypedefs recursively walks the AST to find typedef declarations.
func (e *CExtractor) walkForTypedefs(node *sitter.Node, content []byte, typedefs *[]TypedefInfo) {
	if node == nil {
		return
	}

	if node.Type() == "type_definition" {
		td := e.parseTypeDefinition(node, content)
		if td != nil {
			*typedefs = append(*typedefs, *td)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForTypedefs(node.Child(i), content, typedefs)
	}
}

// parseTypeDefinition extracts information from a type_definition node.
func (e *CExtractor) parseTypeDefinition(node *sitter.Node, content []byte) *TypedefInfo {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var underlying []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "type_identifier":
			// This is the new type name
			if name == "" {
				name = e.nodeText(child, content)
			} else {
				// First one was the underlying type
				underlying = append(underlying, e.nodeText(child, content))
			}
		case "primitive_type":
			underlying = append(underlying, e.nodeText(child, content))
		case "struct_specifier":
			underlying = append(underlying, e.nodeText(child, content))
		case "union_specifier":
			underlying = append(underlying, e.nodeText(child, content))
		case "enum_specifier":
			underlying = append(underlying, e.nodeText(child, content))
		case "sized_type_specifier":
			underlying = append(underlying, e.nodeText(child, content))
		case "pointer_declarator":
			underlying = append(underlying, e.nodeText(child, content))
		case "function_declarator":
			underlying = append(underlying, e.nodeText(child, content))
		case "abstract_pointer_declarator":
			underlying = append(underlying, e.nodeText(child, content))
		}
	}

	if name == "" {
		return nil
	}

	return &TypedefInfo{
		Name:       name,
		Underlying: strings.Join(underlying, " "),
		LineNumber: lineNumber,
	}
}

// nodeText extracts the text content of a node from the source.
func (e *CExtractor) nodeText(node *sitter.Node, content []byte) string {
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

// ExtractFunctions extracts only function definitions from a C file.
func (e *CExtractor) ExtractFunctions(filePath string) ([]types.Function, error) {
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
