// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// goParserPool is a pool of reusable tree-sitter parsers for Go.
var goParserPool = sync.Pool{
	New: func() interface{} {
		parser := sitter.NewParser()
		parser.SetLanguage(golang.GetLanguage())
		return parser
	},
}

// GoExtractor implements the Extractor interface for Go files.
// It uses tree-sitter to parse Go source code and extract structured information
// about functions, methods, structs, interfaces, types, and imports.
type GoExtractor struct {
	*BaseExtractor
}

// NewGoExtractor creates a new Go extractor with initialized parser.
func NewGoExtractor() Extractor {
	return &GoExtractor{
		BaseExtractor: NewBaseExtractor(NewGoParser(), Go),
	}
}

func NewGoParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	return parser
}

// Language returns the language identifier for Go.
func (e *GoExtractor) Language() Language {
	return Go
}

// FileExtensions returns the file extensions supported by Go.
func (e *GoExtractor) FileExtensions() []string {
	return []string{".go"}
}

// Extract parses a Go file and returns structured module information.
func (e *GoExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	// Parse the full AST
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()

	// Single-pass extraction: traverse AST once and collect all constructs
	state := &ExtractorState{}
	e.extractAll(root, content, state)

	// Combine functions and methods
	allFunctions := append(state.functions, state.methods...)

	// Combine structs and interfaces into a single list for Classes
	classes := make([]types.Class, 0, len(state.structs)+len(state.interfaces))
	for _, s := range state.structs {
		classes = append(classes, types.Class{
			Name:       s.Name,
			Docstring:  s.Docstring,
			Methods:    []types.Method{},
			LineNumber: s.LineNumber,
		})
	}
	for _, i := range state.interfaces {
		classes = append(classes, types.Class{
			Name:       i.Name,
			Docstring:  i.Docstring,
			Methods:    i.Methods,
			LineNumber: i.LineNumber,
		})
	}

	return &types.ModuleInfo{
		Path:       filePath,
		Functions:  allFunctions,
		Classes:    classes,
		Imports:    state.imports,
		Structs:    state.structs,
		Interfaces: state.interfaces,
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
	}, nil
}

// extractImports extracts all import statements from the AST.
func (e *GoExtractor) extractImports(node *sitter.Node, content []byte) []types.Import {
	var imports []types.Import
	e.walkForImports(node, content, &imports)
	return imports
}

// walkForImports recursively walks the AST to find import declarations.
func (e *GoExtractor) walkForImports(node *sitter.Node, content []byte, imports *[]types.Import) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "import_declaration":
		e.parseImportDeclaration(node, content, imports)
	case "import_spec_list":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "import_spec" {
				spec := e.parseImportSpec(child, content)
				if spec != nil {
					*imports = append(*imports, types.Import{
						Module:     spec.Module,
						Names:      spec.Names,
						IsFrom:     false,
						LineNumber: int(child.StartPoint().Row) + 1,
					})
				}
			}
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForImports(node.Child(i), content, imports)
	}
}

// parseImportDeclaration extracts information from an import_declaration node.
func (e *GoExtractor) parseImportDeclaration(node *sitter.Node, content []byte, imports *[]types.Import) {
	if node == nil {
		return
	}

	lineNumber := int(node.StartPoint().Row) + 1

	// Check for import with alias: import f "fmt"
	// or grouped imports: import ( ... )
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "import_spec":
			// Single import: import "fmt" or import f "fmt"
			spec := e.parseImportSpec(child, content)
			if spec != nil {
				*imports = append(*imports, types.Import{
					Module:     spec.Module,
					Names:      spec.Names,
					IsFrom:     false,
					LineNumber: lineNumber,
				})
			}
		case "import_spec_list":
			// Grouped imports: import ( "fmt" "os" ) - handled in walkForImports
			return
		}
	}
}

// parseImportSpec extracts information from an import_spec node.
func (e *GoExtractor) parseImportSpec(node *sitter.Node, content []byte) *struct {
	Module string
	Names  []string
} {
	if node == nil {
		return nil
	}

	var module string
	var names []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			// Alias: import f "fmt"
			names = append(names, e.nodeText(child, content))
		case "interpreted_string_literal":
			// The actual import path: import "fmt"
			module = e.nodeText(child, content)
			module = strings.Trim(module, `"`)
		case "import_path":
			// The actual import path
			module = e.nodeText(child, content)
			module = strings.Trim(module, `"`)
		}
	}

	if module == "" {
		return nil
	}

	return &struct {
		Module string
		Names  []string
	}{
		Module: module,
		Names:  names,
	}
}

// extractFunctions extracts all top-level function definitions from the AST.
func (e *GoExtractor) extractFunctions(node *sitter.Node, content []byte) []types.Function {
	var functions []types.Function
	e.walkForFunctions(node, content, &functions, false)
	return functions
}

// walkForFunctions recursively walks the AST to find function definitions.
func (e *GoExtractor) walkForFunctions(node *sitter.Node, content []byte, functions *[]types.Function, isMethod bool) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "function_declaration":
		fn := e.parseFunctionDeclaration(node, content, isMethod)
		if fn != nil {
			*functions = append(*functions, *fn)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForFunctions(node.Child(i), content, functions, isMethod)
	}
}

// parseFunctionDeclaration extracts information from a function_declaration node.
func (e *GoExtractor) parseFunctionDeclaration(node *sitter.Node, content []byte, isMethod bool) *types.Function {
	if node == nil {
		return nil
	}

	var name string
	var params string
	var returnType string

	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			name = e.nodeText(child, content)
		case "parameter_list":
			params = e.nodeText(child, content)
		case "pointer_type":
			// Handle pointer return types like *int
			returnType = e.nodeText(child, content)
		case "qualified_type":
			// Handle qualified types like fmt.Stringer
			returnType = e.nodeText(child, content)
		case "type_identifier":
			// Check if this is a return type (not part of parameters)
			// by checking if it's after the parameter_list
			// For simplicity, we'll capture all type identifiers
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "slice_type":
			returnType = e.nodeText(child, content)
		case "array_type":
			returnType = e.nodeText(child, content)
		case "map_type":
			returnType = e.nodeText(child, content)
		case "chan_type":
			returnType = e.nodeText(child, content)
		case "struct_type":
			returnType = e.nodeText(child, content)
		case "interface_type":
			returnType = e.nodeText(child, content)
		case "function_type":
			returnType = e.nodeText(child, content)
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
		IsMethod:   isMethod,
	}
}

// extractMethods extracts all method definitions from the AST.
func (e *GoExtractor) extractMethods(node *sitter.Node, content []byte) []types.Function {
	var methods []types.Function
	e.walkForMethods(node, content, &methods)
	return methods
}

// walkForMethods recursively walks the AST to find method declarations.
func (e *GoExtractor) walkForMethods(node *sitter.Node, content []byte, methods *[]types.Function) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "method_declaration":
		fn := e.parseMethodDeclaration(node, content)
		if fn != nil {
			*methods = append(*methods, *fn)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForMethods(node.Child(i), content, methods)
	}
}

// parseMethodDeclaration extracts information from a method_declaration node.
func (e *GoExtractor) parseMethodDeclaration(node *sitter.Node, content []byte) *types.Function {
	if node == nil {
		return nil
	}

	var name string
	var receiver string
	var params string
	var returnType string

	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "field_identifier":
			name = e.nodeText(child, content)
		case "parameter_list":
			// First parameter_list is the receiver, second is the method params
			if receiver == "" {
				receiver = e.nodeText(child, content)
			} else {
				params = e.nodeText(child, content)
			}
		case "pointer_type":
			returnType = e.nodeText(child, content)
		case "qualified_type":
			returnType = e.nodeText(child, content)
		case "type_identifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "slice_type":
			returnType = e.nodeText(child, content)
		case "array_type":
			returnType = e.nodeText(child, content)
		case "map_type":
			returnType = e.nodeText(child, content)
		case "chan_type":
			returnType = e.nodeText(child, content)
		case "struct_type":
			returnType = e.nodeText(child, content)
		case "interface_type":
			returnType = e.nodeText(child, content)
		case "function_type":
			returnType = e.nodeText(child, content)
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
		IsMethod:   true,
	}
}

// extractStructs extracts all struct definitions from the AST.
func (e *GoExtractor) extractStructs(node *sitter.Node, content []byte) []types.Struct {
	var structs []types.Struct
	e.walkForStructs(node, content, &structs)
	return structs
}

// walkForStructs recursively walks the AST to find struct definitions.
func (e *GoExtractor) walkForStructs(node *sitter.Node, content []byte, structs *[]types.Struct) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "type_declaration":
		strct := e.parseTypeDeclaration(node, content)
		if strct != nil && strct.IsStruct {
			*structs = append(*structs, strct.Struct)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForStructs(node.Child(i), content, structs)
	}
}

// parseTypeDeclaration extracts information from a type_declaration node.
func (e *GoExtractor) parseTypeDeclaration(node *sitter.Node, content []byte) *struct {
	IsStruct bool
	Struct   types.Struct
} {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1

	// type_declaration has a type_spec child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || child.Type() != "type_spec" {
			continue
		}

		var name string
		var fields []string
		var docstring string

		// Parse type_spec children
		for j := 0; j < int(child.ChildCount()); j++ {
			specChild := child.Child(j)
			if specChild == nil {
				continue
			}

			switch specChild.Type() {
			case "type_identifier":
				// This is the type name
				if name == "" {
					name = e.nodeText(specChild, content)
				} else {
					// Second type_identifier means it's a type alias: type MyType OtherType
					// Handle as a struct with no fields for now
					return &struct {
						IsStruct bool
						Struct   types.Struct
					}{
						IsStruct: false,
						Struct:   types.Struct{},
					}
				}
			case "struct_type":
				fields = e.parseStructFields(specChild, content)
			case "interface_type":
				// This is an interface, handled elsewhere
				return nil
			}
		}

		if name == "" {
			continue
		}

		return &struct {
			IsStruct bool
			Struct   types.Struct
		}{
			IsStruct: true,
			Struct: types.Struct{
				Name:       name,
				Fields:     fields,
				Docstring:  docstring,
				LineNumber: lineNumber,
			},
		}
	}

	return nil
}

// parseStructFields extracts field information from a struct_type node.
func (e *GoExtractor) parseStructFields(node *sitter.Node, content []byte) []string {
	var fields []string

	if node == nil {
		return fields
	}

	// Find field_declaration_list
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || child.Type() != "field_declaration_list" {
			continue
		}

		// Iterate through field declarations
		for j := 0; j < int(child.ChildCount()); j++ {
			fieldChild := child.Child(j)
			if fieldChild == nil {
				continue
			}

			switch fieldChild.Type() {
			case "field_declaration":
				field := e.nodeText(fieldChild, content)
				if field != "" {
					fields = append(fields, field)
				}
			case "anonymous_field":
				// Embedded field: type T struct { OtherType }
				field := e.nodeText(fieldChild, content)
				if field != "" {
					fields = append(fields, field)
				}
			}
		}
	}

	return fields
}

// extractInterfaces extracts all interface definitions from the AST.
func (e *GoExtractor) extractInterfaces(node *sitter.Node, content []byte) []types.Interface {
	var interfaces []types.Interface
	e.walkForInterfaces(node, content, &interfaces)
	return interfaces
}

// walkForInterfaces recursively walks the AST to find interface definitions.
func (e *GoExtractor) walkForInterfaces(node *sitter.Node, content []byte, interfaces *[]types.Interface) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "type_declaration":
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

// parseInterfaceDeclaration extracts information from a type_declaration containing an interface.
func (e *GoExtractor) parseInterfaceDeclaration(node *sitter.Node, content []byte) *types.Interface {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1

	// type_declaration has a type_spec child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || child.Type() != "type_spec" {
			continue
		}

		var name string
		var methods []types.Method

		// Parse type_spec children
		for j := 0; j < int(child.ChildCount()); j++ {
			specChild := child.Child(j)
			if specChild == nil {
				continue
			}

			switch specChild.Type() {
			case "type_identifier":
				name = e.nodeText(specChild, content)
			case "interface_type":
				methods = e.parseInterfaceMethods(specChild, content)
			}
		}

		if name == "" {
			continue
		}

		return &types.Interface{
			Name:       name,
			Methods:    methods,
			LineNumber: lineNumber,
		}
	}

	return nil
}

// parseInterfaceMethods extracts method signatures from an interface_type node.
func (e *GoExtractor) parseInterfaceMethods(node *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	if node == nil {
		return methods
	}

	// Find method elements (methods in the interface)
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "method_elem":
			method := e.parseMethodElem(child, content)
			if method != nil {
				methods = append(methods, *method)
			}
		case "field_declaration":
			// Embedded interface: type Reader interface { OtherInterface }
			method := e.parseEmbeddedInterfaceField(child, content)
			if method != nil {
				methods = append(methods, *method)
			}
		}
	}

	return methods
}

// parseMethodElem extracts information from a method_elem node.
func (e *GoExtractor) parseMethodElem(node *sitter.Node, content []byte) *types.Method {
	if node == nil {
		return nil
	}

	var name string
	var params string
	var returnType string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "field_identifier":
			name = e.nodeText(child, content)
		case "parameter_list":
			if params == "" {
				params = e.nodeText(child, content)
			} else {
				// Second parameter list is the return type
				returnType = e.nodeText(child, content)
			}
		case "pointer_type":
			returnType = e.nodeText(child, content)
		case "qualified_type":
			returnType = e.nodeText(child, content)
		case "type_identifier":
			if returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "slice_type":
			returnType = e.nodeText(child, content)
		case "array_type":
			returnType = e.nodeText(child, content)
		case "map_type":
			returnType = e.nodeText(child, content)
		case "chan_type":
			returnType = e.nodeText(child, content)
		}
	}

	if name == "" {
		return nil
	}

	return &types.Method{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
	}
}

// parseEmbeddedInterfaceField extracts method from an embedded interface field.
func (e *GoExtractor) parseEmbeddedInterfaceField(node *sitter.Node, content []byte) *types.Method {
	if node == nil {
		return nil
	}

	// Embedded interface: just the type name
	name := e.nodeText(node, content)
	if name == "" {
		return nil
	}

	return &types.Method{
		Name: name,
	}
}

// nodeText extracts the text content of a node from the source.
func (e *GoExtractor) nodeText(node *sitter.Node, content []byte) string {
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

// ExtractFunctions extracts only function definitions from a Go file.
func (e *GoExtractor) ExtractFunctions(filePath string) ([]types.Function, error) {
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

// ExtractFromBytes extracts module information from Go source code bytes.
func (e *GoExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing content failed")
	}
	defer tree.Close()

	root := tree.RootNode()

	// Single-pass extraction
	state := &ExtractorState{}
	e.extractAll(root, content, state)

	allFunctions := append(state.functions, state.methods...)

	classes := make([]types.Class, 0, len(state.structs)+len(state.interfaces))
	for _, s := range state.structs {
		classes = append(classes, types.Class{
			Name:       s.Name,
			Docstring:  s.Docstring,
			Methods:    []types.Method{},
			LineNumber: s.LineNumber,
		})
	}
	for _, i := range state.interfaces {
		classes = append(classes, types.Class{
			Name:       i.Name,
			Docstring:  i.Docstring,
			Methods:    i.Methods,
			LineNumber: i.LineNumber,
		})
	}

	return &types.ModuleInfo{
		Path:       filePath,
		Functions:  allFunctions,
		Classes:    classes,
		Imports:    state.imports,
		Structs:    state.structs,
		Interfaces: state.interfaces,
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
	}, nil
}

// TypeAlias represents a type alias definition in Go.
type TypeAlias struct {
	Name       string
	Type       string
	LineNumber int
}

// ExtractorState holds all extraction results from a single-pass AST traversal.
type ExtractorState struct {
	imports    []types.Import
	functions  []types.Function
	methods    []types.Function
	structs    []types.Struct
	interfaces []types.Interface
}

// extractAll performs a single-pass AST traversal to extract all constructs.
// This is more efficient than calling separate extract functions that each walk the AST.
func (e *GoExtractor) extractAll(node *sitter.Node, content []byte, state *ExtractorState) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "import_declaration":
		e.extractImportDeclaration(node, content, state)
	case "import_spec_list":
		e.extractImportSpecList(node, content, state)
	case "function_declaration":
		if fn := e.parseFunctionDeclaration(node, content, false); fn != nil {
			state.functions = append(state.functions, *fn)
		}
	case "method_declaration":
		if fn := e.parseMethodDeclaration(node, content); fn != nil {
			state.methods = append(state.methods, *fn)
		}
	case "type_declaration":
		e.extractTypeDeclaration(node, content, state)
	}

	// Recurse to children
	for i := 0; i < int(node.ChildCount()); i++ {
		e.extractAll(node.Child(i), content, state)
	}
}

// extractImportDeclaration extracts imports from an import_declaration node.
func (e *GoExtractor) extractImportDeclaration(node *sitter.Node, content []byte, state *ExtractorState) {
	if node == nil {
		return
	}

	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "import_spec":
			spec := e.parseImportSpec(child, content)
			if spec != nil {
				state.imports = append(state.imports, types.Import{
					Module:     spec.Module,
					Names:      spec.Names,
					IsFrom:     false,
					LineNumber: lineNumber,
				})
			}
		case "import_spec_list":
			// Grouped imports handled separately
		}
	}
}

// extractImportSpecList extracts imports from an import_spec_list node.
func (e *GoExtractor) extractImportSpecList(node *sitter.Node, content []byte, state *ExtractorState) {
	if node == nil {
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "import_spec" {
			spec := e.parseImportSpec(child, content)
			if spec != nil {
				state.imports = append(state.imports, types.Import{
					Module:     spec.Module,
					Names:      spec.Names,
					IsFrom:     false,
					LineNumber: int(child.StartPoint().Row) + 1,
				})
			}
		}
	}
}

// extractTypeDeclaration extracts structs and interfaces from a type_declaration node.
func (e *GoExtractor) extractTypeDeclaration(node *sitter.Node, content []byte, state *ExtractorState) {
	if node == nil {
		return
	}

	lineNumber := int(node.StartPoint().Row) + 1

	// type_declaration has a type_spec child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || child.Type() != "type_spec" {
			continue
		}

		var name string
		var fields []string
		var methods []types.Method
		var isStruct bool
		var isInterface bool

		// Parse type_spec children
		for j := 0; j < int(child.ChildCount()); j++ {
			specChild := child.Child(j)
			if specChild == nil {
				continue
			}

			switch specChild.Type() {
			case "type_identifier":
				if name == "" {
					name = e.nodeText(specChild, content)
				} else {
					// Second type_identifier means it's a type alias
					return
				}
			case "struct_type":
				fields = e.parseStructFields(specChild, content)
				isStruct = true
			case "interface_type":
				methods = e.parseInterfaceMethods(specChild, content)
				isInterface = true
			}
		}

		if name == "" {
			continue
		}

		if isStruct {
			state.structs = append(state.structs, types.Struct{
				Name:       name,
				Fields:     fields,
				LineNumber: lineNumber,
			})
		} else if isInterface {
			state.interfaces = append(state.interfaces, types.Interface{
				Name:       name,
				Methods:    methods,
				LineNumber: lineNumber,
			})
		}
	}
}
