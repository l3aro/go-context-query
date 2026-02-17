package dfg

import (
	"fmt"
	"os"
	"path/filepath"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

// goDefUseVisitor walks Go AST to extract variable references.
type goDefUseVisitor struct {
	content   []byte
	funcName  string
	refs      []VarRef
	variables map[string][]VarRef
}

func newGoDefUseVisitor(content []byte, funcName string) *goDefUseVisitor {
	return &goDefUseVisitor{
		content:   content,
		funcName:  funcName,
		refs:      make([]VarRef, 0),
		variables: make(map[string][]VarRef),
	}
}

// ExtractDFG extracts the Data Flow Graph for a function.
// It dispatches to the appropriate language-specific implementation based on file extension.
func ExtractDFG(filePath string, functionName string) (*DFGInfo, error) {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".go":
		return extractGoDFG(filePath, functionName)
	case ".py":
		return extractPythonDFG(filePath, functionName)
	case ".ts", ".tsx", ".js", ".jsx":
		return extractTypeScriptDFG(filePath, functionName)
	case ".rs":
		return extractRustDFG(filePath, functionName)
	case ".java":
		return extractJavaDFG(filePath, functionName)
	case ".c", ".h":
		return extractCDFG(filePath, functionName)
	case ".cpp", ".cc", ".cxx", ".hpp":
		return extractCppDFG(filePath, functionName)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}
}

// extractGoDFG extracts DFG for Go files.
func extractGoDFG(filePath string, functionName string) (*DFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	tree := parser.Parse(nil, content)
	defer tree.Close()

	root := tree.RootNode()
	funcNode := findGoFunction(root, functionName, content)
	if funcNode == nil {
		return nil, fmt.Errorf("function %q not found in %s", functionName, filePath)
	}

	visitor := newGoDefUseVisitor(content, functionName)
	visitor.extractReferences(funcNode)

	analyzer := NewReachingDefsAnalyzer()
	edges := analyzer.ComputeDefUseChains(cfgInfo, visitor.refs)

	return &DFGInfo{
		FunctionName:  functionName,
		VarRefs:       visitor.refs,
		DataflowEdges: edges,
		Variables:     visitor.variables,
	}, nil
}

func findGoFunction(node *sitter.Node, funcName string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_declaration" {
		funcNameNode := findChildByTypeGo(node, "identifier", content)
		if funcNameNode != nil {
			name := nodeTextGo(funcNameNode, content)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "method_declaration" {
		funcNameNode := findChildByTypeGo(node, "field_identifier", content)
		if funcNameNode != nil {
			name := nodeTextGo(funcNameNode, content)
			if name == funcName {
				return node
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "type_declaration" ||
			child.Type() == "import_declaration" ||
			child.Type() == "import_spec" {
			continue
		}
		result := findGoFunction(child, funcName, content)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *goDefUseVisitor) extractReferences(funcNode *sitter.Node) {
	if funcNode == nil {
		return
	}

	v.extractParameters(funcNode)

	blockNode := findBlockGo(funcNode)
	if blockNode == nil {
		return
	}

	v.walkBlock(blockNode)
}

func (v *goDefUseVisitor) extractParameters(funcNode *sitter.Node) {
	paramsNode := findChildByTypeGo(funcNode, "parameter_list", v.content)
	if paramsNode == nil {
		return
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "parameter_declaration" {
			nameNode := findChildByTypeGo(child, "identifier", v.content)
			if nameNode != nil {
				name := nodeTextGo(nameNode, v.content)
				if name != "" && !isGoBuiltin(name) {
					ref := VarRef{
						Name:    name,
						RefType: RefTypeDefinition,
						Line:    int(nameNode.StartPoint().Row) + 1,
						Column:  int(nameNode.StartPoint().Column) + 1,
					}
					v.addRef(ref)
				}
			}

			v.extractIdentifiersFromNode(child, RefTypeDefinition)
		}
	}

	if funcNode.Type() == "method_declaration" {
		receiverNode := findChildByTypeGo(funcNode, "parameter_list", v.content)
		if receiverNode != nil && receiverNode.ChildCount() > 0 {
			receiver := receiverNode.Child(0)
			if receiver != nil {
				v.extractIdentifiersFromNode(receiver, RefTypeDefinition)
			}
		}
	}
}

func (v *goDefUseVisitor) walkBlock(blockNode *sitter.Node) {
	if blockNode == nil {
		return
	}

	for i := 0; i < int(blockNode.ChildCount()); i++ {
		child := blockNode.Child(i)
		if child == nil {
			continue
		}

		v.processNode(child)
	}
}

func (v *goDefUseVisitor) processNode(node *sitter.Node) {
	switch node.Type() {
	case "var_spec":
		v.processVarSpec(node)
	case "short_var_declaration":
		v.processShortVarDecl(node)
	case "assignment_expression":
		v.processAssignment(node)
	case "compound_assignment_expression":
		v.processCompoundAssignment(node)
	case "for_statement":
		v.processForStatement(node)
	case "return_statement":
		v.processReturnStatement(node)
	case "if_statement":
		v.processIfStatement(node)
	case "switch_statement":
		v.processSwitchStatement(node)
	case "block":
		v.walkBlock(node)
	default:
		v.extractUses(node)
	}
}

func (v *goDefUseVisitor) processVarSpec(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "identifier" {
			name := nodeTextGo(child, v.content)
			if name != "" && !isGoBuiltin(name) {
				ref := VarRef{
					Name:    name,
					RefType: RefTypeDefinition,
					Line:    int(child.StartPoint().Row) + 1,
					Column:  int(child.StartPoint().Column) + 1,
				}
				v.addRef(ref)
			}
		}
	}

	v.extractUses(node)
}

func (v *goDefUseVisitor) processShortVarDecl(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "left" {
			v.extractIdentifiersFromNode(child, RefTypeDefinition)
		}
	}

	v.extractUses(node)
}

func (v *goDefUseVisitor) processAssignment(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "left" {
			v.extractIdentifiersFromNode(child, RefTypeUpdate)
		}
	}

	v.extractUses(node)
}

func (v *goDefUseVisitor) processCompoundAssignment(node *sitter.Node) {
	v.extractIdentifiersFromNode(node, RefTypeUpdate)
	v.extractUses(node)
}

func (v *goDefUseVisitor) processForStatement(node *sitter.Node) {
	rangeClause := findChildByTypeGo(node, "range_clause", v.content)
	if rangeClause != nil {
		v.processRangeClause(rangeClause)
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "init", "condition", "post":
			v.extractUses(child)
		case "block":
			v.walkBlock(child)
		}
	}
}

func (v *goDefUseVisitor) processRangeClause(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "left" {
			v.extractIdentifiersFromNode(child, RefTypeDefinition)
		}
	}

	v.extractUses(node)
}

func (v *goDefUseVisitor) processReturnStatement(node *sitter.Node) {
	v.extractUses(node)
}

func (v *goDefUseVisitor) processIfStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "init", "condition":
			v.extractUses(child)
		case "consequence", "alternative":
			v.processNode(child)
		}
	}
}

func (v *goDefUseVisitor) processSwitchStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "init", "condition":
			v.extractUses(child)
		case "case_clause":
			v.processCaseClause(child)
		}
	}
}

func (v *goDefUseVisitor) processCaseClause(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "block" {
			v.walkBlock(child)
		}
	}
}

func (v *goDefUseVisitor) extractUses(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextGo(node, v.content)
		if name != "" && !isGoBuiltin(name) {
			if !v.isDefinitionAtPosition(node) {
				ref := VarRef{
					Name:    name,
					RefType: RefTypeUse,
					Line:    int(node.StartPoint().Row) + 1,
					Column:  int(node.StartPoint().Column) + 1,
				}
				v.addRef(ref)
			}
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			v.extractUses(child)
		}
	}
}

func (v *goDefUseVisitor) isDefinitionAtPosition(node *sitter.Node) bool {
	line := int(node.StartPoint().Row) + 1
	col := int(node.StartPoint().Column) + 1

	for _, ref := range v.refs {
		if ref.Line == line && ref.Column == col {
			if ref.RefType == RefTypeDefinition || ref.RefType == RefTypeUpdate {
				return true
			}
		}
	}
	return false
}

func (v *goDefUseVisitor) extractIdentifiersFromNode(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextGo(node, v.content)
		if name != "" && !isGoBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: refType,
				Line:    int(node.StartPoint().Row) + 1,
				Column:  int(node.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			v.extractIdentifiersFromNode(child, refType)
		}
	}
}

func (v *goDefUseVisitor) addRef(ref VarRef) {
	v.refs = append(v.refs, ref)
	v.variables[ref.Name] = append(v.variables[ref.Name], ref)
}

func findBlockGo(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "block" {
		return node
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			result := findBlockGo(child)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

func findChildByTypeGo(node *sitter.Node, childType string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == childType {
			return child
		}
	}

	return nil
}

func nodeTextGo(node *sitter.Node, content []byte) string {
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

func isGoBuiltin(name string) bool {
	builtins := map[string]bool{
		"append": true, "cap": true, "close": true, "complex": true,
		"copy": true, "delete": true, "imag": true, "len": true,
		"make": true, "new": true, "panic": true, "print": true,
		"println": true, "real": true, "recover": true,
		"break": true, "case": true, "chan": true, "const": true,
		"continue": true, "default": true, "defer": true, "else": true,
		"fallthrough": true, "for": true, "func": true, "go": true,
		"goto": true, "if": true, "import": true, "interface": true,
		"map": true, "package": true, "range": true, "return": true,
		"select": true, "struct": true, "switch": true, "type": true,
		"var": true, "true": true, "false": true, "nil": true, "iota": true,
		"bool": true, "byte": true, "complex64": true, "complex128": true,
		"error": true, "float32": true, "float64": true, "int": true,
		"int8": true, "int16": true, "int32": true, "int64": true,
		"rune": true, "string": true, "uint": true, "uint8": true,
		"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	}

	return builtins[name]
}
