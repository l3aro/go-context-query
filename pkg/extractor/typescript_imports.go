// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/l3aro/go-context-query/pkg/types"
	"github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TypeScriptImportParser parses TypeScript import statements using tree-sitter.
type TypeScriptImportParser struct {
	parser *sitter.Parser
}

// NewTypeScriptImportParser creates a new TypeScript import parser.
func NewTypeScriptImportParser() *TypeScriptImportParser {
	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())
	return &TypeScriptImportParser{parser: parser}
}

// ParseImports extracts all import statements from a TypeScript file.
func (p *TypeScriptImportParser) ParseImports(filePath string) ([]types.Import, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return p.ParseImportsFromBytes(content, filePath)
}

// ParseImportsFromBytes extracts imports from TypeScript source code bytes.
func (p *TypeScriptImportParser) ParseImportsFromBytes(content []byte, filePath string) ([]types.Import, error) {
	tree := p.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing failed")
	}
	defer tree.Close()

	root := tree.RootNode()
	var imports []types.Import

	p.walkNode(root, content, &imports)

	return imports, nil
}

// walkNode recursively walks the AST to find import statements.
func (p *TypeScriptImportParser) walkNode(node *sitter.Node, content []byte, imports *[]types.Import) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "import_statement":
		imp := p.parseImportStatement(node, content)
		if imp != nil {
			*imports = append(*imports, *imp)
		}
	case "import_clause":
		// import { x } from 'module' or import * as x from 'module'
		// handled by parent import_statement
	case "named_imports":
		// import { x, y } from 'module'
		// handled by parent import_statement
	case "namespace_import":
		// import * as x from 'module'
		// handled by parent import_statement
	case "external_module_reference":
		// require('module')
		imp := p.parseRequire(node, content)
		if imp != nil {
			*imports = append(*imports, *imp)
		}
	case "call_expression":
		// require('module')
		imp := p.parseRequire(node, content)
		if imp != nil {
			*imports = append(*imports, *imp)
		}
	}

	// Recursively walk children
	for i := 0; i < int(node.ChildCount()); i++ {
		p.walkNode(node.Child(i), content, imports)
	}
}

// parseImportStatement parses "import x from 'module'" or "import { x } from 'module'" statements.
func (p *TypeScriptImportParser) parseImportStatement(node *sitter.Node, content []byte) *types.Import {
	var module string
	var names []string
	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "import":
			// Skip the import keyword
		case "string":
			// Module path
			module = p.nodeText(child, content)
			module = p.cleanModulePath(module)
		case "import_clause":
			// Parse the import clause
			names = p.parseImportClause(child, content)
		case "namespace_import":
			// import * as x from 'module'
			for j := 0; j < int(child.ChildCount()); j++ {
				aliasChild := child.Child(j)
				if aliasChild != nil && aliasChild.Type() == "identifier" {
					names = append(names, "*"+p.nodeText(aliasChild, content))
					break
				}
			}
		case "named_imports":
			// import { x, y } from 'module'
			names = p.parseNamedImports(child, content)
		}
	}

	// Handle default import: import x from 'module' (x is the default import)
	if len(names) == 0 && module != "" {
		// This might be a default import, look for identifier in import_clause
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "import_clause" {
				for j := 0; j < int(child.ChildCount()); j++ {
					impChild := child.Child(j)
					if impChild != nil && impChild.Type() == "identifier" {
						names = append(names, p.nodeText(impChild, content))
						break
					}
				}
			}
		}
	}

	if module == "" {
		return nil
	}

	return &types.Import{
		Module:     module,
		Names:      names,
		IsFrom:     true,
		LineNumber: lineNumber,
	}
}

// parseImportClause parses the import clause to get imported names.
func (p *TypeScriptImportParser) parseImportClause(node *sitter.Node, content []byte) []string {
	var names []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "default_import":
			// import x from 'module'
			for j := 0; j < int(child.ChildCount()); j++ {
				impChild := child.Child(j)
				if impChild != nil && impChild.Type() == "identifier" {
					names = append(names, p.nodeText(impChild, content))
					break
				}
			}
		case "named_imports":
			// import { x } from 'module'
			names = append(names, p.parseNamedImports(child, content)...)
		case "namespace_import":
			// import * as x from 'module'
			for j := 0; j < int(child.ChildCount()); j++ {
				aliasChild := child.Child(j)
				if aliasChild != nil && aliasChild.Type() == "identifier" {
					names = append(names, "*"+p.nodeText(aliasChild, content))
					break
				}
			}
		}
	}

	return names
}

// parseNamedImports parses named imports: { x, y as z }.
func (p *TypeScriptImportParser) parseNamedImports(node *sitter.Node, content []byte) []string {
	var names []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "import_specifier":
			// { x } or { x as y }
			for j := 0; j < int(child.ChildCount()); j++ {
				specChild := child.Child(j)
				if specChild != nil && specChild.Type() == "identifier" {
					names = append(names, p.nodeText(specChild, content))
					break
				}
			}
		}
	}

	return names
}

// parseRequire parses require('module') calls.
func (p *TypeScriptImportParser) parseRequire(node *sitter.Node, content []byte) *types.Import {
	// Check if this is a require call
	isRequire := false
	var module string
	var names []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "identifier" && p.nodeText(child, content) == "require" {
			isRequire = true
		}

		if child.Type() == "arguments" {
			for j := 0; j < int(child.ChildCount()); j++ {
				arg := child.Child(j)
				if arg != nil && arg.Type() == "string" {
					module = p.cleanModulePath(p.nodeText(arg, content))
				}
			}
		}
	}

	if !isRequire || module == "" {
		return nil
	}

	lineNumber := int(node.StartPoint().Row) + 1

	// Try to extract the variable name being assigned to
	parent := node.Parent()
	if parent != nil && parent.Type() == "variable_declarator" {
		for i := 0; i < int(parent.ChildCount()); i++ {
			child := parent.Child(i)
			if child != nil && child.Type() == "identifier" {
				names = append(names, p.nodeText(child, content))
				break
			}
		}
	}

	return &types.Import{
		Module:     module,
		Names:      names,
		IsFrom:     false,
		LineNumber: lineNumber,
	}
}

// cleanModulePath removes quotes from module path.
func (p *TypeScriptImportParser) cleanModulePath(path string) string {
	// Remove quotes (single or double)
	path = strings.Trim(path, "\"'")
	return path
}

// nodeText extracts the text content of a node from the source.
func (p *TypeScriptImportParser) nodeText(node *sitter.Node, content []byte) string {
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

// GetModuleName extracts the module name from a file path.
// For 'import x from './module”, it returns 'module'
func GetModuleName(filePath string) string {
	// Get the base name without extension
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	if ext != "" {
		return strings.TrimSuffix(base, ext)
	}
	return base
}
