// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"sync"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"
)

// phpParserPool is a pool of reusable tree-sitter parsers for PHP.
var phpParserPool = sync.Pool{
	New: func() interface{} {
		parser := sitter.NewParser()
		parser.SetLanguage(php.GetLanguage())
		return parser
	},
}

// PHPExtractor implements the Extractor interface for PHP files.
type PHPExtractor struct {
	*BaseExtractor
}

// NewPHPExtractor creates a new PHP extractor with initialized parser.
func NewPHPExtractor() Extractor {
	return &PHPExtractor{
		BaseExtractor: NewBaseExtractor(NewPHPParser(), PHP),
	}
}

func NewPHPParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(php.GetLanguage())
	return parser
}

// Language returns the language identifier for PHP.
func (e *PHPExtractor) Language() Language {
	return PHP
}

// FileExtensions returns the file extensions supported by PHP.
func (e *PHPExtractor) FileExtensions() []string {
	return []string{".php", ".phtml"}
}

// Extract parses a PHP file and returns structured module information.
func (e *PHPExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	return e.ExtractFromBytes(content, filePath)
}

// ExtractFromBytes extracts module information from PHP source code bytes.
func (e *PHPExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()

	classes := e.extractClasses(root, content)
	interfaces := e.extractInterfaces(root, content)
	traits := e.extractTraits(root, content)
	functions := e.extractFunctions(root, content)
	imports := e.extractUseStatements(root, content)

	return &types.ModuleInfo{
		Path:       filePath,
		Functions:  functions,
		Classes:    classes,
		Imports:    imports,
		Traits:     traits,
		Interfaces: interfaces,
		Language:   string(e.Language()),
		CallGraph:  types.CallGraph{Edges: []types.CallGraphEdge{}},
	}, nil
}

// extractUseStatements extracts use statements.
func (e *PHPExtractor) extractUseStatements(node *sitter.Node, content []byte) []types.Import {
	var imports []types.Import
	e.walkForUse(node, content, &imports)
	return imports
}

func (e *PHPExtractor) walkForUse(node *sitter.Node, content []byte, imports *[]types.Import) {
	if node == nil {
		return
	}
	if node.Type() == "use_declaration" || node.Type() == "use_statement" {
		lineNumber := int(node.StartPoint().Row) + 1
		module := e.getNodeText(node, content)
		*imports = append(*imports, types.Import{
			Module:     module,
			LineNumber: lineNumber,
		})
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForUse(node.Child(i), content, imports)
	}
}

// extractClasses extracts class definitions.
func (e *PHPExtractor) extractClasses(node *sitter.Node, content []byte) []types.Class {
	var classes []types.Class
	e.walkForClasses(node, content, &classes)
	return classes
}

func (e *PHPExtractor) walkForClasses(node *sitter.Node, content []byte, classes *[]types.Class) {
	if node == nil {
		return
	}
	if node.Type() == "class_declaration" {
		lineNumber := int(node.StartPoint().Row) + 1
		var name string
		var methods []types.Method

		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "name" {
				name = e.getNodeText(child, content)
			}
			if child != nil && child.Type() == "declaration_list" {
				methods = e.extractMethods(child, content)
			}
		}

		if name != "" {
			*classes = append(*classes, types.Class{
				Name:       name,
				Methods:    methods,
				LineNumber: lineNumber,
			})
		}
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForClasses(node.Child(i), content, classes)
	}
}

// extractInterfaces extracts interface definitions.
func (e *PHPExtractor) extractInterfaces(node *sitter.Node, content []byte) []types.Interface {
	var interfaces []types.Interface
	e.walkForInterfaces(node, content, &interfaces)
	return interfaces
}

func (e *PHPExtractor) walkForInterfaces(node *sitter.Node, content []byte, interfaces *[]types.Interface) {
	if node == nil {
		return
	}
	if node.Type() == "interface_declaration" {
		lineNumber := int(node.StartPoint().Row) + 1
		var name string

		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "name" {
				name = e.getNodeText(child, content)
			}
		}

		if name != "" {
			*interfaces = append(*interfaces, types.Interface{
				Name:       name,
				LineNumber: lineNumber,
			})
		}
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForInterfaces(node.Child(i), content, interfaces)
	}
}

// extractTraits extracts trait definitions.
func (e *PHPExtractor) extractTraits(node *sitter.Node, content []byte) []types.Trait {
	var traits []types.Trait
	e.walkForTraits(node, content, &traits)
	return traits
}

func (e *PHPExtractor) walkForTraits(node *sitter.Node, content []byte, traits *[]types.Trait) {
	if node == nil {
		return
	}
	if node.Type() == "trait_declaration" {
		lineNumber := int(node.StartPoint().Row) + 1
		var name string

		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "name" {
				name = e.getNodeText(child, content)
			}
		}

		if name != "" {
			*traits = append(*traits, types.Trait{
				Name:       name,
				LineNumber: lineNumber,
			})
		}
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForTraits(node.Child(i), content, traits)
	}
}

// extractFunctions extracts function definitions.
func (e *PHPExtractor) extractFunctions(node *sitter.Node, content []byte) []types.Function {
	var functions []types.Function
	e.walkForFunctions(node, content, &functions)
	return functions
}

func (e *PHPExtractor) walkForFunctions(node *sitter.Node, content []byte, functions *[]types.Function) {
	if node == nil {
		return
	}
	if node.Type() == "function_definition" {
		lineNumber := int(node.StartPoint().Row) + 1
		var name, params string

		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "name" {
				name = e.getNodeText(child, content)
			}
			if child != nil && child.Type() == "formal_parameters" {
				params = e.getNodeText(child, content)
			}
		}

		if name != "" {
			*functions = append(*functions, types.Function{
				Name:       name,
				Params:     params,
				LineNumber: lineNumber,
			})
		}
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForFunctions(node.Child(i), content, functions)
	}
}

// extractMethods extracts method definitions from a class body.
func (e *PHPExtractor) extractMethods(node *sitter.Node, content []byte) []types.Method {
	var methods []types.Method
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "method_declaration" {
			lineNumber := int(child.StartPoint().Row) + 1
			var name, params, returnType string

			for j := 0; j < int(child.ChildCount()); j++ {
				mchild := child.Child(j)
				if mchild == nil {
					continue
				}
				switch mchild.Type() {
				case "name":
					name = e.getNodeText(mchild, content)
				case "formal_parameters":
					params = e.getNodeText(mchild, content)
				case "primitive_type":
					returnType = e.getNodeText(mchild, content)
				}
			}

			if name != "" {
				methods = append(methods, types.Method{
					Name:       name,
					Params:     params,
					ReturnType: returnType,
					LineNumber: lineNumber,
					IsMethod:   true,
				})
			}
		}
	}
	return methods
}

// getNodeText extracts text from a node.
func (e *PHPExtractor) getNodeText(node *sitter.Node, content []byte) string {
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
