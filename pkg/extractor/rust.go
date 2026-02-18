// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"
)

// rustParserPool is a pool of reusable tree-sitter parsers for Rust.
var rustParserPool = sync.Pool{
	New: func() interface{} {
		parser := sitter.NewParser()
		parser.SetLanguage(rust.GetLanguage())
		return parser
	},
}

// RustExtractor implements the Extractor interface for Rust files.
// It uses tree-sitter to parse Rust source code and extract structured information
// about functions, structs, traits, impl blocks, enums, and use statements.
type RustExtractor struct {
	*BaseExtractor
}

// NewRustExtractor creates a new Rust extractor with initialized parser.
func NewRustExtractor() Extractor {
	return &RustExtractor{
		BaseExtractor: NewBaseExtractor(NewRustParser(), Rust),
	}
}

func NewRustParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(rust.GetLanguage())
	return parser
}

// Language returns the language identifier for Rust.
func (e *RustExtractor) Language() Language {
	return Rust
}

// FileExtensions returns the file extensions supported by Rust.
func (e *RustExtractor) FileExtensions() []string {
	return []string{".rs", ".rlib"}
}

// Extract parses a Rust file and returns structured module information.
func (e *RustExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	return e.ExtractFromBytes(content, filePath)
}

// ExtractFromBytes extracts module information from Rust source code bytes.
func (e *RustExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
	// Parse the full AST
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()

	// Extract all constructs
	imports := e.extractImports(root, content)
	functions := e.extractFunctions(root, content)
	structs := e.extractStructs(root, content)
	traits := e.extractTraits(root, content)
	implBlocks := e.extractImplBlocks(root, content)
	enums := e.extractEnums(root, content)

	// Convert impl block methods to functions
	for _, impl := range implBlocks {
		for _, method := range impl.Methods {
			fn := types.Function{
				Name:       method.Name,
				Params:     method.Params,
				ReturnType: method.ReturnType,
				Docstring:  method.Docstring,
				LineNumber: method.LineNumber,
				IsMethod:   true,
			}
			functions = append(functions, fn)
		}
	}

	// Convert structs and traits to classes for compatibility
	classes := make([]types.Class, 0, len(structs)+len(traits))
	for _, s := range structs {
		classes = append(classes, types.Class{
			Name:       s.Name,
			Docstring:  s.Docstring,
			Methods:    []types.Method{},
			LineNumber: s.LineNumber,
		})
	}
	for _, t := range traits {
		classes = append(classes, types.Class{
			Name:       t.Name,
			Docstring:  t.Docstring,
			Methods:    t.Methods,
			LineNumber: t.LineNumber,
		})
	}

	return &types.ModuleInfo{
		Path:      filePath,
		Functions: functions,
		Classes:   classes,
		Imports:   imports,
		Structs:   structs,
		Traits:    traits,
		Enums:     enums,
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
	}, nil
}

// extractImports extracts all use statements from the AST.
func (e *RustExtractor) extractImports(node *sitter.Node, content []byte) []types.Import {
	var imports []types.Import
	e.walkForImports(node, content, &imports)
	return imports
}

// walkForImports recursively walks the AST to find use declarations.
func (e *RustExtractor) walkForImports(node *sitter.Node, content []byte, imports *[]types.Import) {
	if node == nil {
		return
	}

	if node.Type() == "use_declaration" {
		imp := e.parseUseDeclaration(node, content)
		if imp != nil {
			*imports = append(*imports, *imp)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForImports(node.Child(i), content, imports)
	}
}

// parseUseDeclaration extracts information from a use_declaration node.
func (e *RustExtractor) parseUseDeclaration(node *sitter.Node, content []byte) *types.Import {
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
		case "use_clause":
			module, names = e.parseUseClause(child, content)
		case "scoped_use_list":
			module, names = e.parseScopedUseList(child, content)
		case "use_list":
			module = ""
			names = e.parseUseList(child, content)
		case "identifier":
			// Simple use: use module::Item
			if module != "" {
				module = module + "::" + e.nodeText(child, content)
			} else {
				module = e.nodeText(child, content)
			}
		case "scoped_identifier":
			module = e.nodeText(child, content)
		}
	}

	if module == "" && len(names) == 0 {
		return nil
	}

	return &types.Import{
		Module:     module,
		Names:      names,
		IsFrom:     false,
		LineNumber: lineNumber,
	}
}

// parseUseClause extracts information from a use_clause node.
func (e *RustExtractor) parseUseClause(node *sitter.Node, content []byte) (string, []string) {
	if node == nil {
		return "", nil
	}

	var module string
	var names []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "scoped_identifier", "identifier":
			module = e.nodeText(child, content)
		case "use_list":
			names = e.parseUseList(child, content)
		case "use_as_clause":
			// Handle use Item as Alias
			orig, alias := e.parseUseAsClause(child, content)
			if alias != "" {
				names = append(names, alias)
				module = orig
			}
		}
	}

	return module, names
}

// parseScopedUseList extracts information from a scoped_use_list node (e.g., use std::io::{...}).
func (e *RustExtractor) parseScopedUseList(node *sitter.Node, content []byte) (string, []string) {
	if node == nil {
		return "", nil
	}

	var module string
	var names []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "scoped_identifier", "identifier":
			module = e.nodeText(child, content)
		case "use_list":
			names = e.parseUseList(child, content)
		}
	}

	return module, names
}

// parseUseList extracts names from a use_list node.
func (e *RustExtractor) parseUseList(node *sitter.Node, content []byte) []string {
	var names []string

	if node == nil {
		return names
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier", "self":
			names = append(names, e.nodeText(child, content))
		case "use_as_clause":
			_, alias := e.parseUseAsClause(child, content)
			if alias != "" {
				names = append(names, alias)
			}
		}
	}

	return names
}

// parseUseAsClause extracts original name and alias from a use_as_clause node.
func (e *RustExtractor) parseUseAsClause(node *sitter.Node, content []byte) (string, string) {
	if node == nil {
		return "", ""
	}

	var original, alias string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			if original == "" {
				original = e.nodeText(child, content)
			} else {
				alias = e.nodeText(child, content)
			}
		}
	}

	return original, alias
}

// extractFunctions extracts all top-level function definitions from the AST.
func (e *RustExtractor) extractFunctions(node *sitter.Node, content []byte) []types.Function {
	var functions []types.Function
	e.walkForFunctions(node, content, &functions)
	return functions
}

// walkForFunctions recursively walks the AST to find function definitions.
func (e *RustExtractor) walkForFunctions(node *sitter.Node, content []byte, functions *[]types.Function) {
	if node == nil {
		return
	}

	if node.Type() == "impl_item" {
		return
	}

	if node.Type() == "function_item" && (node.Parent() == nil || node.Parent().Type() == "source_file") {
		fn := e.parseFunctionItem(node, content, false)
		if fn != nil {
			*functions = append(*functions, *fn)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForFunctions(node.Child(i), content, functions)
	}
}

// parseFunctionItem extracts information from a function_item node.
func (e *RustExtractor) parseFunctionItem(node *sitter.Node, content []byte, isMethod bool) *types.Function {
	if node == nil {
		return nil
	}

	var name string
	var params string
	var returnType string
	var docstring string
	isAsync := false
	foundArrow := false

	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "function_modifiers":
			for j := 0; j < int(child.ChildCount()); j++ {
				modChild := child.Child(j)
				if modChild != nil && modChild.Type() == "async" {
					isAsync = true
				}
			}
		case "async":
			isAsync = true
		case "fn":
			// Function keyword, skip
		case "identifier":
			name = e.nodeText(child, content)
		case "parameters":
			params = e.nodeText(child, content)
		case "->":
			foundArrow = true
		case "primitive_type", "type_identifier", "scoped_type_identifier", "generic_type", "array_type", "reference_type":
			if foundArrow && returnType == "" {
				returnType = e.nodeText(child, content)
			}
		case "block":
			docstring = e.extractDocstring(child, content)
		}
	}

	if name == "" {
		return nil
	}

	return &types.Function{
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		Docstring:  docstring,
		LineNumber: lineNumber,
		IsMethod:   isMethod,
		IsAsync:    isAsync,
	}
}

// parseReturnType extracts the return type from a return_type node.
func (e *RustExtractor) parseReturnType(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// return_type node starts with ->, skip it and get the actual type
	var typeText strings.Builder
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() != "->" {
			typeText.WriteString(e.nodeText(child, content))
		}
	}

	return strings.TrimSpace(typeText.String())
}

// extractDocstring extracts documentation comments from a block.
func (e *RustExtractor) extractDocstring(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// In Rust, doc comments are outer attributes (/// or //!)
	// For simplicity, we'll look for line comments starting with ///
	return ""
}

// extractStructs extracts all struct definitions from the AST.
func (e *RustExtractor) extractStructs(node *sitter.Node, content []byte) []types.Struct {
	var structs []types.Struct
	e.walkForStructs(node, content, &structs)
	return structs
}

// walkForStructs recursively walks the AST to find struct definitions.
func (e *RustExtractor) walkForStructs(node *sitter.Node, content []byte, structs *[]types.Struct) {
	if node == nil {
		return
	}

	if node.Type() == "struct_item" {
		strct := e.parseStructItem(node, content)
		if strct != nil {
			*structs = append(*structs, *strct)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForStructs(node.Child(i), content, structs)
	}
}

// parseStructItem extracts information from a struct_item node.
func (e *RustExtractor) parseStructItem(node *sitter.Node, content []byte) *types.Struct {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var fields []string
	var docstring string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "struct":
		case "type_identifier":
			name = e.nodeText(child, content)
		case "type_parameters":
			// Generic parameters, append to name
			name += e.nodeText(child, content)
		case "field_declaration_list":
			fields = e.parseFieldDeclarationList(child, content)
		case "unit_type":
			// Unit struct: struct Name;
			// No fields
		}
	}

	if name == "" {
		return nil
	}

	return &types.Struct{
		Name:       name,
		Fields:     fields,
		Docstring:  docstring,
		LineNumber: lineNumber,
	}
}

// parseFieldDeclarationList extracts fields from a field_declaration_list node.
func (e *RustExtractor) parseFieldDeclarationList(node *sitter.Node, content []byte) []string {
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
			if field != "" && field != "{" && field != "}" && field != "," {
				fields = append(fields, field)
			}
		}
	}

	return fields
}

// extractTraits extracts all trait definitions from the AST.
func (e *RustExtractor) extractTraits(node *sitter.Node, content []byte) []types.Trait {
	var traits []types.Trait
	e.walkForTraits(node, content, &traits)
	return traits
}

// walkForTraits recursively walks the AST to find trait definitions.
func (e *RustExtractor) walkForTraits(node *sitter.Node, content []byte, traits *[]types.Trait) {
	if node == nil {
		return
	}

	if node.Type() == "trait_item" {
		trait := e.parseTraitItem(node, content)
		if trait != nil {
			*traits = append(*traits, *trait)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForTraits(node.Child(i), content, traits)
	}
}

// parseTraitItem extracts information from a trait_item node.
func (e *RustExtractor) parseTraitItem(node *sitter.Node, content []byte) *types.Trait {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var methods []types.Method
	var docstring string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "trait":
		case "type_identifier":
			name = e.nodeText(child, content)
		case "type_parameters":
			name += e.nodeText(child, content)
		case "declaration_list":
			methods = e.extractTraitMethods(child, content)
		}
	}

	if name == "" {
		return nil
	}

	return &types.Trait{
		Name:       name,
		Methods:    methods,
		Docstring:  docstring,
		LineNumber: lineNumber,
	}
}

// extractTraitMethods extracts method signatures from a trait declaration list.
func (e *RustExtractor) extractTraitMethods(node *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	if node == nil {
		return methods
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "function_item" || child.Type() == "function_signature_item" {
			fn := e.parseFunctionItem(child, content, false)
			if fn != nil {
				methods = append(methods, types.Method{
					Name:       fn.Name,
					Params:     fn.Params,
					ReturnType: fn.ReturnType,
					Docstring:  fn.Docstring,
					LineNumber: fn.LineNumber,
					IsMethod:   false,
					IsAsync:    fn.IsAsync,
				})
			}
		}
	}

	return methods
}

// ImplBlock represents an impl block with its methods.
type ImplBlock struct {
	Type    string
	Methods []types.Method
}

// extractImplBlocks extracts all impl blocks from the AST.
func (e *RustExtractor) extractImplBlocks(node *sitter.Node, content []byte) []ImplBlock {
	var impls []ImplBlock
	e.walkForImplBlocks(node, content, &impls)
	return impls
}

// walkForImplBlocks recursively walks the AST to find impl blocks.
func (e *RustExtractor) walkForImplBlocks(node *sitter.Node, content []byte, impls *[]ImplBlock) {
	if node == nil {
		return
	}

	if node.Type() == "impl_item" {
		impl := e.parseImplItem(node, content)
		if impl != nil {
			*impls = append(*impls, *impl)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForImplBlocks(node.Child(i), content, impls)
	}
}

// parseImplItem extracts information from an impl_item node.
func (e *RustExtractor) parseImplItem(node *sitter.Node, content []byte) *ImplBlock {
	if node == nil {
		return nil
	}

	var implType string
	var methods []types.Method

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "impl":
			// Skip impl keyword
		case "type":
			// impl Trait for Type
			// Continue to find the actual type
		case "identifier", "scoped_type_identifier", "type_identifier":
			if implType == "" {
				implType = e.nodeText(child, content)
			}
		case "type_parameters":
			implType += e.nodeText(child, content)
		case "declaration_list":
			methods = e.extractImplMethods(child, content)
		}
	}

	if implType == "" {
		return nil
	}

	return &ImplBlock{
		Type:    implType,
		Methods: methods,
	}
}

// extractImplMethods extracts methods from an impl block.
func (e *RustExtractor) extractImplMethods(node *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	if node == nil {
		return methods
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "function_item" {
			fn := e.parseFunctionItem(child, content, true)
			if fn != nil {
				methods = append(methods, types.Method{
					Name:       fn.Name,
					Params:     fn.Params,
					ReturnType: fn.ReturnType,
					Docstring:  fn.Docstring,
					LineNumber: fn.LineNumber,
					IsMethod:   true,
					IsAsync:    fn.IsAsync,
				})
			}
		}
	}

	return methods
}

// extractEnums extracts all enum definitions from the AST.
func (e *RustExtractor) extractEnums(node *sitter.Node, content []byte) []types.Enum {
	var enums []types.Enum
	e.walkForEnums(node, content, &enums)
	return enums
}

// walkForEnums recursively walks the AST to find enum definitions.
func (e *RustExtractor) walkForEnums(node *sitter.Node, content []byte, enums *[]types.Enum) {
	if node == nil {
		return
	}

	if node.Type() == "enum_item" {
		enumItem := e.parseEnumItem(node, content)
		if enumItem != nil {
			*enums = append(*enums, *enumItem)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForEnums(node.Child(i), content, enums)
	}
}

// parseEnumItem extracts information from an enum_item node.
func (e *RustExtractor) parseEnumItem(node *sitter.Node, content []byte) *types.Enum {
	if node == nil {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var name string
	var variants []string
	var docstring string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "enum":
		case "type_identifier":
			name = e.nodeText(child, content)
		case "type_parameters":
			name += e.nodeText(child, content)
		case "enum_variant_list":
			variants = e.parseEnumVariantList(child, content)
		}
	}

	if name == "" {
		return nil
	}

	return &types.Enum{
		Name:       name,
		Variants:   variants,
		Docstring:  docstring,
		LineNumber: lineNumber,
	}
}

// parseEnumVariantList extracts variants from an enum_variant_list node.
func (e *RustExtractor) parseEnumVariantList(node *sitter.Node, content []byte) []string {
	var variants []string

	if node == nil {
		return variants
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "enum_variant" {
			variant := e.parseEnumVariant(child, content)
			if variant != "" {
				variants = append(variants, variant)
			}
		}
	}

	return variants
}

// parseEnumVariant extracts the name from an enum_variant node.
func (e *RustExtractor) parseEnumVariant(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "identifier" {
			return e.nodeText(child, content)
		}
	}

	return ""
}

// nodeText extracts the text content of a node from the source.
func (e *RustExtractor) nodeText(node *sitter.Node, content []byte) string {
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

// ExtractFunctions extracts only function definitions from a Rust file.
func (e *RustExtractor) ExtractFunctions(filePath string) ([]types.Function, error) {
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
