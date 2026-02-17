package dfg

import (
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

type cppDefUseVisitor struct {
	content   []byte
	funcName  string
	refs      []VarRef
	variables map[string][]VarRef
}

func newCppDefUseVisitor(content []byte, funcName string) *cppDefUseVisitor {
	return &cppDefUseVisitor{
		content:   content,
		funcName:  funcName,
		refs:      make([]VarRef, 0),
		variables: make(map[string][]VarRef),
	}
}

func extractCppDFG(filePath string, functionName string) (*DFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractCppCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(cpp.GetLanguage())
	tree := parser.Parse(nil, content)
	defer tree.Close()

	root := tree.RootNode()
	funcNode := findCppFunction(root, functionName, content)
	if funcNode == nil {
		return nil, fmt.Errorf("function %q not found in %s", functionName, filePath)
	}

	visitor := newCppDefUseVisitor(content, functionName)
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

func findCppFunction(node *sitter.Node, funcName string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_definition" {
		funcNameNode := findChildByTypeCpp(node, "identifier", content)
		if funcNameNode != nil {
			name := nodeTextCpp(funcNameNode, content)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "class_specifier" {
		classBody := findChildByTypeCpp(node, "field_declaration_list", content)
		if classBody != nil {
			for i := 0; i < int(classBody.ChildCount()); i++ {
				child := classBody.Child(i)
				if child != nil {
					result := findCppFunction(child, funcName, content)
					if result != nil {
						return result
					}
				}
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
		result := findCppFunction(child, funcName, content)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *cppDefUseVisitor) extractReferences(funcNode *sitter.Node) {
	if funcNode == nil {
		return
	}

	v.extractParameters(funcNode)

	blockNode := findBlockCpp(funcNode)
	if blockNode == nil {
		return
	}

	v.walkBlock(blockNode)
}

func (v *cppDefUseVisitor) extractParameters(funcNode *sitter.Node) {
	paramsNode := findChildByTypeCpp(funcNode, "parameter_list", v.content)
	if paramsNode == nil {
		return
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "parameter_declaration" {
			v.extractParameterDeclaration(child)
		}
	}
}

func (v *cppDefUseVisitor) extractParameterDeclaration(paramNode *sitter.Node) {
	if paramNode == nil {
		return
	}

	for i := 0; i < int(paramNode.ChildCount()); i++ {
		child := paramNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "identifier" {
			name := nodeTextCpp(child, v.content)
			if name != "" && !isCppBuiltin(name) {
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

func (v *cppDefUseVisitor) walkBlock(blockNode *sitter.Node) {
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

func (v *cppDefUseVisitor) processNode(node *sitter.Node) {
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
	case "try_statement":
		v.processTryStatement(node)
	case "throw_statement":
		v.processThrowStatement(node)
	case "compound_statement":
		v.walkBlock(node)
	default:
		v.extractUses(node)
	}
}

func (v *cppDefUseVisitor) processDeclaration(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "init_declarator" {
			nameNode := findChildByTypeCpp(child, "identifier", v.content)
			if nameNode != nil {
				name := nodeTextCpp(nameNode, v.content)
				if name != "" && !isCppBuiltin(name) {
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

func (v *cppDefUseVisitor) processAssignment(node *sitter.Node) {
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

func (v *cppDefUseVisitor) processUpdateExpression(node *sitter.Node) {
	argument := findChildByTypeCpp(node, "argument", v.content)
	if argument != nil {
		v.extractIdentifiersFromNode(argument, RefTypeUpdate)
	}

	v.extractUses(node)
}

func (v *cppDefUseVisitor) processForStatement(node *sitter.Node) {
	init := findChildByTypeCpp(node, "init", v.content)
	if init != nil {
		v.processNode(init)
	}

	condition := findChildByTypeCpp(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	update := findChildByTypeCpp(node, "update", v.content)
	if update != nil {
		v.extractUses(update)
	}

	body := findChildByTypeCpp(node, "body", v.content)
	if body != nil {
		if body.Type() == "compound_statement" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}
}

func (v *cppDefUseVisitor) processWhileStatement(node *sitter.Node) {
	condition := findChildByTypeCpp(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	body := findChildByTypeCpp(node, "body", v.content)
	if body != nil {
		if body.Type() == "compound_statement" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}
}

func (v *cppDefUseVisitor) processDoStatement(node *sitter.Node) {
	body := findChildByTypeCpp(node, "body", v.content)
	if body != nil {
		if body.Type() == "compound_statement" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}

	condition := findChildByTypeCpp(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}
}

func (v *cppDefUseVisitor) processReturnStatement(node *sitter.Node) {
	v.extractUses(node)
}

func (v *cppDefUseVisitor) processIfStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
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

func (v *cppDefUseVisitor) processSwitchStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
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

func (v *cppDefUseVisitor) processTryStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "compound_statement":
			v.walkBlock(child)
		case "catch_clause":
			v.processCatchClause(child)
		}
	}
}

func (v *cppDefUseVisitor) processCatchClause(node *sitter.Node) {
	parameter := findChildByTypeCpp(node, "parameter", v.content)
	if parameter != nil {
		v.extractIdentifiersFromNode(parameter, RefTypeDefinition)
	}

	block := findChildByTypeCpp(node, "compound_statement", v.content)
	if block != nil {
		v.walkBlock(block)
	}
}

func (v *cppDefUseVisitor) processThrowStatement(node *sitter.Node) {
	v.extractUses(node)
}

func (v *cppDefUseVisitor) extractUses(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextCpp(node, v.content)
		if name != "" && !isCppBuiltin(name) {
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

func (v *cppDefUseVisitor) isDefinitionAtPosition(node *sitter.Node) bool {
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

func (v *cppDefUseVisitor) extractIdentifiersFromNode(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextCpp(node, v.content)
		if name != "" && !isCppBuiltin(name) {
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

func (v *cppDefUseVisitor) addRef(ref VarRef) {
	v.refs = append(v.refs, ref)
	v.variables[ref.Name] = append(v.variables[ref.Name], ref)
}

func findBlockCpp(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "compound_statement" {
		return node
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			result := findBlockCpp(child)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

func findChildByTypeCpp(node *sitter.Node, childType string, content []byte) *sitter.Node {
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

func nodeTextCpp(node *sitter.Node, content []byte) string {
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

func isCppBuiltin(name string) bool {
	builtins := map[string]bool{
		"auto": true, "break": true, "case": true, "char": true,
		"const": true, "continue": true, "default": true, "do": true,
		"double": true, "else": true, "enum": true, "extern": true,
		"float": true, "for": true, "goto": true, "if": true,
		"inline": true, "int": true, "long": true, "register": true,
		"restrict": true, "return": true, "short": true, "signed": true,
		"sizeof": true, "static": true, "struct": true, "switch": true,
		"typedef": true, "union": true, "unsigned": true, "void": true,
		"volatile": true, "while": true, "class": true, "public": true,
		"private": true, "protected": true, "virtual": true, "override": true,
		"final": true, "friend": true, "namespace": true, "using": true,
		"template": true, "typename": true, "try": true, "catch": true,
		"throw": true, "new": true, "delete": true, "this": true,
		"nullptr": true, "true": true, "false": true, "NULL": true,

		"std": true, "string": true, "vector": true, "map": true,
		"set": true, "list": true, "deque": true, "array": true,
		"pair": true, "tuple": true, "make_pair": true, "make_tuple": true,
		"cout": true, "cin": true, "endl": true, "printf": true,
		"scanf": true, "malloc": true, "free": true,
	}

	return builtins[name]
}
