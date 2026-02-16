// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
)

// CSharpExtractor implements the Extractor interface for C# files.
type CSharpExtractor struct {
	*BaseExtractor
}

// NewCSharpExtractor creates a new C# extractor with initialized parser.
func NewCSharpExtractor() Extractor {
	return &CSharpExtractor{
		BaseExtractor: NewBaseExtractor(NewCSharpParser(), CSharp),
	}
}

// NewCSharpParser creates a new tree-sitter parser for C#.
func NewCSharpParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(csharp.GetLanguage())
	return parser
}

// Language returns the language identifier for C#.
func (e *CSharpExtractor) Language() Language {
	return CSharp
}

// FileExtensions returns the file extensions supported by C#.
func (e *CSharpExtractor) FileExtensions() []string {
	return []string{".cs", ".csx"}
}

// Extract parses a C# file and returns structured module information.
func (e *CSharpExtractor) Extract(filePath string) (*types.ModuleInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	return e.ExtractFromBytes(content, filePath)
}

// ExtractFromBytes extracts module information from C# source code bytes.
func (e *CSharpExtractor) ExtractFromBytes(content []byte, filePath string) (*types.ModuleInfo, error) {
	tree := e.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing file %s failed", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()

	classes := e.extractClasses(root, content)
	interfaces := e.extractInterfaces(root, content)
	imports := e.extractUsingStatements(root, content)

	return &types.ModuleInfo{
		Path:       filePath,
		Functions:  []types.Function{},
		Classes:    classes,
		Interfaces: interfaces,
		Imports:    imports,
		CallGraph:  types.CallGraph{Edges: []types.CallGraphEdge{}},
	}, nil
}

// extractUsingStatements extracts using statements.
func (e *CSharpExtractor) extractUsingStatements(node *sitter.Node, content []byte) []types.Import {
	var imports []types.Import
	e.walkForUsing(node, content, &imports)
	return imports
}

func (e *CSharpExtractor) walkForUsing(node *sitter.Node, content []byte, imports *[]types.Import) {
	if node == nil {
		return
	}
	if node.Type() == "using_directive" {
		lineNumber := int(node.StartPoint().Row) + 1
		module := e.getNodeText(node, content)
		*imports = append(*imports, types.Import{
			Module:     module,
			LineNumber: lineNumber,
		})
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkForUsing(node.Child(i), content, imports)
	}
}

// extractClasses extracts class definitions.
func (e *CSharpExtractor) extractClasses(node *sitter.Node, content []byte) []types.Class {
	var classes []types.Class
	e.walkForClasses(node, content, &classes)
	return classes
}

func (e *CSharpExtractor) walkForClasses(node *sitter.Node, content []byte, classes *[]types.Class) {
	if node == nil {
		return
	}
	if node.Type() == "class_declaration" {
		lineNumber := int(node.StartPoint().Row) + 1
		var name string
		var methods []types.Method

		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "identifier" {
				name = e.getNodeText(child, content)
			}
			if child != nil && child.Type() == "class_body" {
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
func (e *CSharpExtractor) extractInterfaces(node *sitter.Node, content []byte) []types.Interface {
	var interfaces []types.Interface
	e.walkForInterfaces(node, content, &interfaces)
	return interfaces
}

func (e *CSharpExtractor) walkForInterfaces(node *sitter.Node, content []byte, interfaces *[]types.Interface) {
	if node == nil {
		return
	}
	if node.Type() == "interface_declaration" {
		lineNumber := int(node.StartPoint().Row) + 1
		var name string

		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "identifier" {
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

// extractMethods extracts method definitions from a class body.
func (e *CSharpExtractor) extractMethods(node *sitter.Node, content []byte) []types.Method {
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
				case "identifier":
					name = e.getNodeText(mchild, content)
				case "parameter_list":
					params = e.getNodeText(mchild, content)
				case "predefined_type", "builtin_type":
					returnType = e.getNodeText(mchild, content)
				}
			}

			if name != "" {
				methods = append(methods, types.Method{
					Name:       name,
					Params:     params,
					ReturnType: returnType,
					LineNumber: lineNumber,
				})
			}
		}
	}
	return methods
}

// getNodeText extracts text from a node.
func (e *CSharpExtractor) getNodeText(node *sitter.Node, content []byte) string {
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
