package dfg

import (
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

type cDefUseVisitor struct {
	content   []byte
	funcName  string
	refs      []VarRef
	variables map[string][]VarRef
}

func newCDefUseVisitor(content []byte, funcName string) *cDefUseVisitor {
	return &cDefUseVisitor{
		content:   content,
		funcName:  funcName,
		refs:      make([]VarRef, 0),
		variables: make(map[string][]VarRef),
	}
}

func extractCDFG(filePath string, functionName string) (*DFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractCCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(c.GetLanguage())
	tree := parser.Parse(nil, content)
	defer tree.Close()

	root := tree.RootNode()
	funcNode := findCFunction(root, functionName, content)
	if funcNode == nil {
		return nil, fmt.Errorf("function %q not found in %s", functionName, filePath)
	}

	visitor := newCDefUseVisitor(content, functionName)
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

func findCFunction(node *sitter.Node, funcName string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_definition" {
		funcNameNode := findChildByTypeC(node, "identifier", content)
		if funcNameNode != nil {
			name := nodeTextC(funcNameNode, content)
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
		if child.Type() == "preproc_include" || child.Type() == "comment" {
			continue
		}
		result := findCFunction(child, funcName, content)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *cDefUseVisitor) extractReferences(funcNode *sitter.Node) {
	if funcNode == nil {
		return
	}

	v.extractParameters(funcNode)

	blockNode := findBlockC(funcNode)
	if blockNode == nil {
		return
	}

	v.walkBlock(blockNode)
}

func (v *cDefUseVisitor) extractParameters(funcNode *sitter.Node) {
	paramsNode := findChildByTypeC(funcNode, "parameter_list", v.content)
	if paramsNode == nil {
		return
	}

	childCount := paramsNode.ChildCount()
	for i := 0; i < int(childCount); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "parameter_declaration" {
			v.extractParameterDeclaration(child)
		}
	}
}

func (v *cDefUseVisitor) extractParameterDeclaration(paramNode *sitter.Node) {
	if paramNode == nil {
		return
	}

	childCount := paramNode.ChildCount()
	for i := 0; i < int(childCount); i++ {
		child := paramNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "identifier" {
			name := nodeTextC(child, v.content)
			if name != "" && !isCBuiltin(name) {
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
}

func (v *cDefUseVisitor) walkBlock(blockNode *sitter.Node) {
	if blockNode == nil {
		return
	}

	childCount := blockNode.ChildCount()
	for i := 0; i < int(childCount); i++ {
		child := blockNode.Child(i)
		if child == nil {
			continue
		}

		v.processNode(child)
	}
}

func (v *cDefUseVisitor) processNode(node *sitter.Node) {
	switch node.Type() {
	case "declaration":
		v.processDeclaration(node)
	case "assignment_expression":
		v.processAssignment(node)
	case "update_expression":
		v.processUpdateExpression(node)
	case "for_statement":
		v.processForStatement(node)
	case "while_statement":
		v.processWhileStatement(node)
	case "do_statement":
		v.processDoStatement(node)
	case "return_statement":
		v.processReturnStatement(node)
	case "if_statement":
		v.processIfStatement(node)
	case "switch_statement":
		v.processSwitchStatement(node)
	case "compound_statement":
		v.walkBlock(node)
	default:
		v.extractUses(node)
	}
}

func (v *cDefUseVisitor) processDeclaration(node *sitter.Node) {
	childCount := node.ChildCount()
	for i := 0; i < int(childCount); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "init_declarator" {
			nameNode := findChildByTypeC(child, "identifier", v.content)
			if nameNode != nil {
				name := nodeTextC(nameNode, v.content)
				if name != "" && !isCBuiltin(name) {
					ref := VarRef{
						Name:    name,
						RefType: RefTypeDefinition,
						Line:    int(nameNode.StartPoint().Row) + 1,
						Column:  int(nameNode.StartPoint().Column) + 1,
					}
					v.addRef(ref)
				}
			}
		}
	}

	v.extractUses(node)
}

func (v *cDefUseVisitor) processAssignment(node *sitter.Node) {
	childCount := node.ChildCount()
	for i := 0; i < int(childCount); i++ {
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

func (v *cDefUseVisitor) processUpdateExpression(node *sitter.Node) {
	argument := findChildByTypeC(node, "argument", v.content)
	if argument != nil {
		v.extractIdentifiersFromNode(argument, RefTypeUpdate)
	}

	v.extractUses(node)
}

func (v *cDefUseVisitor) processForStatement(node *sitter.Node) {
	init := findChildByTypeC(node, "init", v.content)
	if init != nil {
		v.processNode(init)
	}

	condition := findChildByTypeC(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	update := findChildByTypeC(node, "update", v.content)
	if update != nil {
		v.extractUses(update)
	}

	body := findChildByTypeC(node, "body", v.content)
	if body != nil {
		if body.Type() == "compound_statement" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}
}

func (v *cDefUseVisitor) processWhileStatement(node *sitter.Node) {
	condition := findChildByTypeC(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	body := findChildByTypeC(node, "body", v.content)
	if body != nil {
		if body.Type() == "compound_statement" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}
}

func (v *cDefUseVisitor) processDoStatement(node *sitter.Node) {
	body := findChildByTypeC(node, "body", v.content)
	if body != nil {
		if body.Type() == "compound_statement" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}

	condition := findChildByTypeC(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}
}

func (v *cDefUseVisitor) processReturnStatement(node *sitter.Node) {
	v.extractUses(node)
}

func (v *cDefUseVisitor) processIfStatement(node *sitter.Node) {
	childCount := node.ChildCount()
	for i := 0; i < int(childCount); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "condition":
			v.extractUses(child)
		case "consequence", "alternative":
			if child.Type() == "compound_statement" {
				v.walkBlock(child)
			} else {
				v.processNode(child)
			}
		}
	}
}

func (v *cDefUseVisitor) processSwitchStatement(node *sitter.Node) {
	childCount := node.ChildCount()
	for i := 0; i < int(childCount); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "condition":
			v.extractUses(child)
		case "body":
			if child.Type() == "compound_statement" {
				v.walkBlock(child)
			} else {
				v.processNode(child)
			}
		}
	}
}

func (v *cDefUseVisitor) extractUses(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextC(node, v.content)
		if name != "" && !isCBuiltin(name) {
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

	childCount := node.ChildCount()
	for i := 0; i < int(childCount); i++ {
		child := node.Child(i)
		if child != nil {
			v.extractUses(child)
		}
	}
}

func (v *cDefUseVisitor) isDefinitionAtPosition(node *sitter.Node) bool {
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

func (v *cDefUseVisitor) extractIdentifiersFromNode(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextC(node, v.content)
		if name != "" && !isCBuiltin(name) {
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

	childCount := node.ChildCount()
	for i := 0; i < int(childCount); i++ {
		child := node.Child(i)
		if child != nil {
			v.extractIdentifiersFromNode(child, refType)
		}
	}
}

func (v *cDefUseVisitor) addRef(ref VarRef) {
	v.refs = append(v.refs, ref)
	v.variables[ref.Name] = append(v.variables[ref.Name], ref)
}

func findBlockC(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "compound_statement" {
		return node
	}

	childCount := node.ChildCount()
	for i := 0; i < int(childCount); i++ {
		child := node.Child(i)
		if child != nil {
			result := findBlockC(child)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

func findChildByTypeC(node *sitter.Node, childType string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	childCount := node.ChildCount()
	for i := 0; i < int(childCount); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == childType {
			return child
		}
	}

	return nil
}

func nodeTextC(node *sitter.Node, content []byte) string {
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

func isCBuiltin(name string) bool {
	builtins := map[string]bool{
		"auto": true, "break": true, "case": true, "char": true,
		"const": true, "continue": true, "default": true, "do": true,
		"double": true, "else": true, "enum": true, "extern": true,
		"float": true, "for": true, "goto": true, "if": true,
		"inline": true, "int": true, "long": true, "register": true,
		"restrict": true, "return": true, "short": true, "signed": true,
		"sizeof": true, "static": true, "struct": true, "switch": true,
		"typedef": true, "union": true, "unsigned": true, "void": true,
		"volatile": true, "while": true,

		"_Bool": true, "_Complex": true, "_Imaginary": true,
		"bool": true, "complex": true, "imaginary": true,

		"true": true, "false": true, "NULL": true, "null": true,

		"printf": true, "scanf": true, "malloc": true, "free": true,
		"memcpy": true, "memset": true, "strcpy": true,
		"strlen": true, "strcat": true, "strcmp": true, "fopen": true,
		"fclose": true, "fread": true, "fwrite": true, "fprintf": true,
		"fscanf": true, "exit": true, "abort": true, "assert": true,
	}

	return builtins[name]
}
