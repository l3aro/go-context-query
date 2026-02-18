// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"
)

// rubyParserPool is a pool of reusable tree-sitter parsers for Ruby.
var rubyParserPool = sync.Pool{
	New: func() interface{} {
		parser := sitter.NewParser()
		parser.SetLanguage(ruby.GetLanguage())
		return parser
	},
}

// RubyExtractor implements the Extractor interface for Ruby files.
// It uses tree-sitter to parse Ruby source code and extract structured information
// about methods, classes, modules, and require statements.
type RubyExtractor struct {
	*BaseExtractor
}

// NewRubyExtractor creates a new Ruby extractor with initialized parser.
func NewRubyExtractor() Extractor {
	return &RubyExtractor{
		BaseExtractor: NewBaseExtractor(NewRubyParser(), Ruby),
	}
}

func NewRubyParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(ruby.GetLanguage())
	return parser
}

// Language returns the language identifier for Ruby.
func (e *RubyExtractor) Language() Language {
	return Ruby
}

// FileExtensions returns the file extensions supported by Ruby.
func (e *RubyExtractor) FileExtensions() []string {
	return []string{".rb", ".erb", ".gemspec"}
}

// Extract parses a Ruby file and returns structured module information.
func (e *RubyExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	return e.ExtractFromBytes(content, filePath)
}

// ExtractFromBytes extracts module information from Ruby source code bytes.
func (e *RubyExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
	// Parse the full AST
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()

	// Extract all constructs
	imports := e.extractRequires(root, content)
	functions := e.extractMethods(root, content, false)
	classes := e.extractClasses(root, content)
	modules := e.extractModules(root, content)

	return &types.ModuleInfo{
		Path:      filePath,
		Functions: functions,
		Classes:   classes,
		Imports:   imports,
		CallGraph: types.CallGraph{
			Edges: []types.CallGraphEdge{},
		},
		Traits: convertModulesToTraits(modules),
	}, nil
}

// convertModulesToTraits converts Ruby modules to Trait structures for storage.
func convertModulesToTraits(modules []RubyModule) []types.Trait {
	traits := make([]types.Trait, 0, len(modules))
	for _, mod := range modules {
		methods := make([]types.Method, 0, len(mod.Methods))
		for _, fn := range mod.Methods {
			methods = append(methods, types.Method{
				Name:       fn.Name,
				Params:     fn.Params,
				ReturnType: fn.ReturnType,
				Docstring:  fn.Docstring,
				LineNumber: fn.LineNumber,
				IsMethod:   fn.IsMethod,
				IsAsync:    fn.IsAsync,
			})
		}
		traits = append(traits, types.Trait{
			Name:       mod.Name,
			Docstring:  mod.Docstring,
			Methods:    methods,
			LineNumber: mod.LineNumber,
		})
	}
	return traits
}

// RubyModule represents a Ruby module definition.
type RubyModule struct {
	Name       string
	Docstring  string
	Methods    []types.Function
	LineNumber int
}

// extractRequires extracts all require and require_relative statements.
func (e *RubyExtractor) extractRequires(node *sitter.Node, content []byte) []types.Import {
	var imports []types.Import
	e.walkForRequires(node, content, &imports)
	return imports
}

// walkForRequires recursively walks the AST to find require statements.
func (e *RubyExtractor) walkForRequires(node *sitter.Node, content []byte, imports *[]types.Import) {
	if node == nil {
		return
	}

	if node.Type() == "call" {
		imp := e.parseRequireCall(node, content)
		if imp != nil {
			*imports = append(*imports, *imp)
			return
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForRequires(node.Child(i), content, imports)
	}
}

// parseRequireCall extracts information from a require/require_relative call.
func (e *RubyExtractor) parseRequireCall(node *sitter.Node, content []byte) *types.Import {
	if node == nil || node.Type() != "call" {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1
	var funcName string
	var arg string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			funcName = e.nodeText(child, content)
		case "argument_list":
			arg = e.parseArgumentList(child, content)
		}
	}

	if funcName != "require" && funcName != "require_relative" {
		return nil
	}

	return &types.Import{
		Module:     arg,
		Names:      []string{},
		IsFrom:     funcName == "require_relative",
		LineNumber: lineNumber,
	}
}

// parseArgumentList extracts the first string argument from an argument list.
func (e *RubyExtractor) parseArgumentList(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "string":
			return e.extractStringContent(child, content)
		case "simple_symbol":
			// Handle symbol arguments like :json
			text := e.nodeText(child, content)
			if strings.HasPrefix(text, ":") {
				return text[1:]
			}
			return text
		}
	}

	return ""
}

// extractStringContent extracts the content from a string node (removes quotes).
func (e *RubyExtractor) extractStringContent(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	text := e.nodeText(node, content)
	// Remove surrounding quotes
	text = strings.TrimPrefix(text, `"`)
	text = strings.TrimSuffix(text, `"`)
	text = strings.TrimPrefix(text, "'")
	text = strings.TrimSuffix(text, "'")
	return text
}

// extractMethods extracts all method definitions from the AST.
// If isClassMethod is true, only class methods (def self.method) are extracted.
func (e *RubyExtractor) extractMethods(node *sitter.Node, content []byte, isClassMethod bool) []types.Function {
	var functions []types.Function
	e.walkForMethods(node, content, &functions, isClassMethod, false)
	return functions
}

// walkForMethods recursively walks the AST to find method definitions.
func (e *RubyExtractor) walkForMethods(node *sitter.Node, content []byte, functions *[]types.Function, isClassMethod bool, insideClass bool) {
	if node == nil {
		return
	}

	if node.Type() == "method" {
		fn := e.parseMethod(node, content, insideClass)
		if fn != nil {
			// Filter by class method status if requested
			if !isClassMethod || (isClassMethod && fn.IsMethod && strings.HasPrefix(fn.Name, "self.")) {
				*functions = append(*functions, *fn)
			}
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForMethods(node.Child(i), content, functions, isClassMethod, insideClass)
	}
}

// parseMethod extracts information from a method node.
func (e *RubyExtractor) parseMethod(node *sitter.Node, content []byte, isMethod bool) *types.Function {
	if node == nil || node.Type() != "method" {
		return nil
	}

	var name string
	var params string
	var docstring string
	isSingleton := false

	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "def":
		case "identifier":
			name = e.nodeText(child, content)
		case "singleton_method":
			isSingleton = true
			name = e.parseSingletonMethodName(child, content)
		case "method_parameters":
			params = e.nodeText(child, content)
		case "body_statement":
			docstring = e.extractMethodDocstring(child, content)
		}
	}

	if name == "" {
		return nil
	}

	return &types.Function{
		Name:       name,
		Params:     params,
		ReturnType: "", // Ruby is dynamically typed
		Docstring:  docstring,
		LineNumber: lineNumber,
		IsMethod:   isMethod || isSingleton,
		IsAsync:    false, // Ruby doesn't have async/await syntax
	}
}

// parseSingletonMethodName extracts the name from a singleton method (def self.method).
func (e *RubyExtractor) parseSingletonMethodName(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for the identifier in the singleton method
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "identifier" {
			return "self." + e.nodeText(child, content)
		}
	}

	return ""
}

// extractMethodDocstring extracts documentation from a method body.
func (e *RubyExtractor) extractMethodDocstring(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for comment nodes before the actual code
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "comment" {
			comment := e.nodeText(child, content)
			// Remove # prefix
			comment = strings.TrimPrefix(comment, "#")
			comment = strings.TrimSpace(comment)
			if comment != "" {
				return comment
			}
		}
	}

	return ""
}

// extractClasses extracts all class definitions from the AST.
func (e *RubyExtractor) extractClasses(node *sitter.Node, content []byte) []types.Class {
	var classes []types.Class
	e.walkForClasses(node, content, &classes)
	return classes
}

// walkForClasses recursively walks the AST to find class definitions.
func (e *RubyExtractor) walkForClasses(node *sitter.Node, content []byte, classes *[]types.Class) {
	if node == nil {
		return
	}

	if node.Type() == "class" {
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

// parseClass extracts information from a class node.
func (e *RubyExtractor) parseClass(node *sitter.Node, content []byte) *types.Class {
	if node == nil || node.Type() != "class" {
		return nil
	}

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
			// Skip class keyword
		case "constant":
			name = e.nodeText(child, content)
		case "superclass":
			bases = e.parseSuperclass(child, content)
		case "body_statement":
			docstring = e.extractClassDocstring(child, content)
			// Also extract methods from the class body
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

// parseSuperclass extracts the parent class name from a superclass node.
func (e *RubyExtractor) parseSuperclass(node *sitter.Node, content []byte) []string {
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
		case "constant":
			base := e.nodeText(child, content)
			if base != "" {
				bases = append(bases, base)
			}
		case "scope_resolution":
			// Handle namespaced superclass like Parent::BaseClass
			base := e.nodeText(child, content)
			if base != "" {
				bases = append(bases, base)
			}
		}
	}

	return bases
}

// extractClassDocstring extracts documentation from a class body.
func (e *RubyExtractor) extractClassDocstring(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for comment nodes
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "comment" {
			comment := e.nodeText(child, content)
			// Remove # prefix
			comment = strings.TrimPrefix(comment, "#")
			comment = strings.TrimSpace(comment)
			if comment != "" {
				return comment
			}
		}
	}

	return ""
}

// extractClassMethods extracts methods defined within a class body.
func (e *RubyExtractor) extractClassMethods(classNode *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "body_statement" {
			methods = append(methods, e.extractMethodsFromBody(child, content)...)
		}
	}

	return methods
}

func (e *RubyExtractor) extractMethodsFromBody(bodyNode *sitter.Node, content []byte) []types.Method {
	var methods []types.Method

	for j := 0; j < int(bodyNode.ChildCount()); j++ {
		bodyChild := bodyNode.Child(j)
		if bodyChild == nil {
			continue
		}

		switch bodyChild.Type() {
		case "method":
			fn := e.parseMethod(bodyChild, content, true)
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
		case "singleton_method":
			fn := e.parseSingletonMethod(bodyChild, content)
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

func (e *RubyExtractor) parseSingletonMethod(node *sitter.Node, content []byte) *types.Function {
	if node == nil || node.Type() != "singleton_method" {
		return nil
	}

	var name string
	var params string
	var docstring string

	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "def":
		case "identifier":
			if name == "" {
				name = "self." + e.nodeText(child, content)
			}
		case "method_parameters":
			params = e.nodeText(child, content)
		case "body_statement":
			docstring = e.extractMethodDocstring(child, content)
		}
	}

	if name == "" {
		return nil
	}

	return &types.Function{
		Name:       name,
		Params:     params,
		ReturnType: "",
		Docstring:  docstring,
		LineNumber: lineNumber,
		IsMethod:   true,
		IsAsync:    false,
	}
}

// extractModules extracts all module definitions from the AST.
func (e *RubyExtractor) extractModules(node *sitter.Node, content []byte) []RubyModule {
	var modules []RubyModule
	e.walkForModules(node, content, &modules)
	return modules
}

// walkForModules recursively walks the AST to find module definitions.
func (e *RubyExtractor) walkForModules(node *sitter.Node, content []byte, modules *[]RubyModule) {
	if node == nil {
		return
	}

	if node.Type() == "module" {
		mod := e.parseModule(node, content)
		if mod != nil {
			*modules = append(*modules, *mod)
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForModules(node.Child(i), content, modules)
	}
}

// parseModule extracts information from a module node.
func (e *RubyExtractor) parseModule(node *sitter.Node, content []byte) *RubyModule {
	if node == nil || node.Type() != "module" {
		return nil
	}

	var name string
	var docstring string

	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "module":
			// Skip module keyword
		case "constant":
			name = e.nodeText(child, content)
		case "scope_resolution":
			// Handle namespaced module
			name = e.nodeText(child, content)
		case "body_statement":
			docstring = e.extractModuleDocstring(child, content)
		}
	}

	if name == "" {
		return nil
	}

	methods := e.extractModuleMethods(node, content)

	return &RubyModule{
		Name:       name,
		Docstring:  docstring,
		Methods:    methods,
		LineNumber: lineNumber,
	}
}

// extractModuleDocstring extracts documentation from a module body.
func (e *RubyExtractor) extractModuleDocstring(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for comment nodes
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "comment" {
			comment := e.nodeText(child, content)
			// Remove # prefix
			comment = strings.TrimPrefix(comment, "#")
			comment = strings.TrimSpace(comment)
			if comment != "" {
				return comment
			}
		}
	}

	return ""
}

// extractModuleMethods extracts methods defined within a module body.
func (e *RubyExtractor) extractModuleMethods(moduleNode *sitter.Node, content []byte) []types.Function {
	var methods []types.Function

	for i := 0; i < int(moduleNode.ChildCount()); i++ {
		child := moduleNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "body_statement" {
			for j := 0; j < int(child.ChildCount()); j++ {
				bodyChild := child.Child(j)
				if bodyChild == nil {
					continue
				}

				if bodyChild.Type() == "method" {
					fn := e.parseMethod(bodyChild, content, true)
					if fn != nil {
						methods = append(methods, *fn)
					}
				}
			}
		}
	}

	return methods
}

// nodeText extracts the text content of a node from the source.
func (e *RubyExtractor) nodeText(node *sitter.Node, content []byte) string {
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

// ExtractFunctions extracts only function/method definitions from a Ruby file.
func (e *RubyExtractor) ExtractFunctions(filePath string) ([]types.Function, error) {
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
	functions := e.extractMethods(root, content, false)

	return functions, nil
}

// ExtractClasses extracts only class definitions from a Ruby file.
func (e *RubyExtractor) ExtractClasses(filePath string) ([]types.Class, error) {
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

// ExtractModules extracts only module definitions from a Ruby file.
func (e *RubyExtractor) ExtractModules(filePath string) ([]RubyModule, error) {
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
	modules := e.extractModules(root, content)

	return modules, nil
}
