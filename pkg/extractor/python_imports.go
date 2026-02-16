// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"os"
	"strings"

	"github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/user/go-context-query/pkg/types"
)

// PythonImportParser parses Python import statements using tree-sitter.
type PythonImportParser struct {
	parser *sitter.Parser
}

// NewPythonImportParser creates a new Python import parser.
func NewPythonImportParser() *PythonImportParser {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())
	return &PythonImportParser{parser: parser}
}

// ParseImports extracts all import statements from a Python file.
func (p *PythonImportParser) ParseImports(filePath string) ([]types.Import, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

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

// ParseImportsFromBytes extracts imports from Python source code bytes.
func (p *PythonImportParser) ParseImportsFromBytes(content []byte) ([]types.Import, error) {
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
func (p *PythonImportParser) walkNode(node *sitter.Node, content []byte, imports *[]types.Import) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "import_statement":
		imp := p.parseImportStatement(node, content)
		if imp != nil {
			*imports = append(*imports, *imp)
		}
	case "import_from_statement":
		imp := p.parseImportFromStatement(node, content)
		if imp != nil {
			*imports = append(*imports, *imp)
		}
	}

	// Recursively walk children
	for i := 0; i < int(node.ChildCount()); i++ {
		p.walkNode(node.Child(i), content, imports)
	}
}

// parseImportStatement parses "import x" or "import x as y" statements.
// import_statement: "import" _import_list
// _import_list: (dotted_name | aliased_import) ("," (dotted_name | aliased_import))*
func (p *PythonImportParser) parseImportStatement(node *sitter.Node, content []byte) *types.Import {
	// import_statement has children: "import" keyword + import list items
	// We need to find dotted_name and aliased_import nodes

	var names []string
	lineNumber := int(node.StartPoint().Row) + 1 // Convert to 1-based

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "dotted_name":
			// Simple import: import os
			name := p.nodeText(child, content)
			if name != "" {
				names = append(names, name)
			}
		case "aliased_import":
			// Aliased import: import x as y
			name := p.parseAliasedImport(child, content)
			if name != "" {
				names = append(names, name)
			}
		}
	}

	if len(names) == 0 {
		return nil
	}

	// For regular import statements, each name is treated as its own module
	// We return the first one as the primary module
	return &types.Import{
		Module:     names[0],
		Names:      names,
		IsFrom:     false,
		LineNumber: lineNumber,
	}
}

// parseImportFromStatement parses "from x import y" or "from x import y as z" statements.
// import_from_statement: "from" (relative_import | dotted_name) "import" (wildcard_import | _import_list)
func (p *PythonImportParser) parseImportFromStatement(node *sitter.Node, content []byte) *types.Import {
	var module string
	var names []string
	lineNumber := int(node.StartPoint().Row) + 1 // Convert to 1-based

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "dotted_name":
			// Module name: from os.path import ...
			// Note: The first dotted_name is the module, subsequent ones are imported names
			text := p.nodeText(child, content)
			if module == "" {
				module = text
			} else {
				names = append(names, text)
			}
		case "relative_import":
			// Relative import: from . import ... or from ..module import ...
			module = p.parseRelativeImport(child, content)
		case "wildcard_import":
			// from module import *
			names = append(names, "*")
		case "aliased_import":
			// from module import x as y
			name := p.parseAliasedImport(child, content)
			if name != "" {
				names = append(names, name)
			}
		}
	}

	// If we haven't collected names from aliased_import nodes, check for dotted_name children
	// that might be part of the import list
	if len(names) == 0 {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			if child.Type() == "dotted_name" {
				text := p.nodeText(child, content)
				if text != "" && text != module {
					names = append(names, text)
				}
			}
		}
	}

	return &types.Import{
		Module:     module,
		Names:      names,
		IsFrom:     true,
		LineNumber: lineNumber,
	}
}

// parseAliasedImport extracts the alias from "name as alias" pattern.
// aliased_import: dotted_name "as" identifier
// Returns the alias if present, otherwise returns the original name.
func (p *PythonImportParser) parseAliasedImport(node *sitter.Node, content []byte) string {
	var name, alias string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "dotted_name":
			name = p.nodeText(child, content)
		case "identifier":
			// This could be the alias after "as"
			if alias == "" {
				alias = p.nodeText(child, content)
			}
		}
	}

	// Return alias if present, otherwise return the original name
	if alias != "" {
		return alias
	}
	return name
}

// parseRelativeImport extracts the module name from a relative import.
// relative_import: import_prefix (dotted_name)?
// import_prefix: "."+
func (p *PythonImportParser) parseRelativeImport(node *sitter.Node, content []byte) string {
	var prefix string
	var module string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "import_prefix":
			prefix = p.nodeText(child, content)
		case "dotted_name":
			module = p.nodeText(child, content)
		}
	}

	if module != "" {
		return prefix + module
	}
	return prefix
}

// nodeText extracts the text content of a node from the source.
func (p *PythonImportParser) nodeText(node *sitter.Node, content []byte) string {
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

// ParsePythonImports is a convenience function to parse imports from a file.
func ParsePythonImports(filePath string) ([]types.Import, error) {
	parser := NewPythonImportParser()
	return parser.ParseImports(filePath)
}

// ParsePythonImportsFromBytes is a convenience function to parse imports from bytes.
func ParsePythonImportsFromBytes(content []byte) ([]types.Import, error) {
	parser := NewPythonImportParser()
	return parser.ParseImportsFromBytes(content)
}

// parseImportList extracts import names from an import list node.
// This handles both "import x, y, z" and "from m import x, y, z" patterns.
func (p *PythonImportParser) parseImportList(node *sitter.Node, content []byte) []string {
	var names []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "dotted_name":
			name := p.nodeText(child, content)
			if name != "" {
				names = append(names, name)
			}
		case "aliased_import":
			name := p.parseAliasedImport(child, content)
			if name != "" {
				names = append(names, name)
			}
		}
	}

	return names
}

// ExtractImportInfo provides detailed import information including aliases map.
// This extends the basic Import struct with alias mappings for more advanced use cases.
type ImportInfo struct {
	Module     string
	Names      []string
	Aliases    map[string]string // maps alias -> original name
	IsFrom     bool
	LineNumber int
}

// ParseImportsDetailed extracts imports with full alias information.
func (p *PythonImportParser) ParseImportsDetailed(filePath string) ([]ImportInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	tree := p.parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parsing failed")
	}
	defer tree.Close()

	root := tree.RootNode()
	var imports []ImportInfo

	p.walkNodeDetailed(root, content, &imports)

	return imports, nil
}

// walkNodeDetailed recursively walks the AST for detailed import extraction.
func (p *PythonImportParser) walkNodeDetailed(node *sitter.Node, content []byte, imports *[]ImportInfo) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "import_statement":
		imps := p.parseImportStatementDetailed(node, content)
		*imports = append(*imports, imps...)
	case "import_from_statement":
		imp := p.parseImportFromStatementDetailed(node, content)
		if imp != nil {
			*imports = append(*imports, *imp)
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		p.walkNodeDetailed(node.Child(i), content, imports)
	}
}

// parseImportStatementDetailed parses regular import statements with alias details.
func (p *PythonImportParser) parseImportStatementDetailed(node *sitter.Node, content []byte) []ImportInfo {
	var imports []ImportInfo
	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "dotted_name":
			name := p.nodeText(child, content)
			if name != "" {
				imports = append(imports, ImportInfo{
					Module:     name,
					Names:      []string{name},
					Aliases:    nil,
					IsFrom:     false,
					LineNumber: lineNumber,
				})
			}
		case "aliased_import":
			name, alias := p.parseAliasedImportDetailed(child, content)
			if name != "" {
				aliases := map[string]string{name: alias}
				if alias == "" {
					aliases = nil
				}
				imports = append(imports, ImportInfo{
					Module:     name,
					Names:      []string{name},
					Aliases:    aliases,
					IsFrom:     false,
					LineNumber: lineNumber,
				})
			}
		}
	}

	return imports
}

// parseImportFromStatementDetailed parses from imports with alias details.
func (p *PythonImportParser) parseImportFromStatementDetailed(node *sitter.Node, content []byte) *ImportInfo {
	var module string
	var names []string
	aliases := make(map[string]string)
	lineNumber := int(node.StartPoint().Row) + 1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "dotted_name":
			text := p.nodeText(child, content)
			// First dotted_name is the module, subsequent ones are imported names
			if module == "" {
				module = text
			} else {
				names = append(names, text)
			}
		case "relative_import":
			module = p.parseRelativeImport(child, content)
		case "wildcard_import":
			names = append(names, "*")
		case "aliased_import":
			name, alias := p.parseAliasedImportDetailed(child, content)
			if name != "" {
				names = append(names, name)
				if alias != "" && alias != name {
					aliases[name] = alias
				}
			}
		}
	}

	return &ImportInfo{
		Module:     module,
		Names:      names,
		Aliases:    aliases,
		IsFrom:     true,
		LineNumber: lineNumber,
	}
}

// parseAliasedImportDetailed extracts both name and alias.
func (p *PythonImportParser) parseAliasedImportDetailed(node *sitter.Node, content []byte) (name, alias string) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "dotted_name":
			name = p.nodeText(child, content)
		case "identifier":
			// The identifier after "as" is the alias
			if alias == "" {
				alias = p.nodeText(child, content)
			}
		}
	}

	return name, alias
}

// normalizeModuleName cleans up module names (removes extra whitespace, etc.).
func normalizeModuleName(name string) string {
	// Remove extra whitespace and normalize
	parts := strings.Fields(name)
	return strings.Join(parts, "")
}

// isValidImport checks if an import structure is valid (has required fields).
func isValidImport(imp *types.Import) bool {
	return imp != nil && (imp.Module != "" || len(imp.Names) > 0)
}

// IsRelativeImport checks if an import is relative (starts with dot).
func IsRelativeImport(module string) bool {
	return strings.HasPrefix(module, ".")
}

// GetRelativeLevel returns the relative import level (number of dots).
func GetRelativeLevel(module string) int {
	level := 0
	for _, ch := range module {
		if ch == '.' {
			level++
		} else {
			break
		}
	}
	return level
}
