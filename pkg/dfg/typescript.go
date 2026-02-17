package dfg

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

type tsDefUseVisitor struct {
	content   []byte
	funcName  string
	refs      []VarRef
	variables map[string][]VarRef
}

func newTSDefUseVisitor(content []byte, funcName string) *tsDefUseVisitor {
	return &tsDefUseVisitor{
		content:   content,
		funcName:  funcName,
		refs:      make([]VarRef, 0),
		variables: make(map[string][]VarRef),
	}
}

func extractTypeScriptDFG(filePath string, functionName string) (*DFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractTSCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())
	tree := parser.Parse(nil, content)
	defer tree.Close()

	root := tree.RootNode()
	funcNode := findTSFunction(root, functionName, content)
	if funcNode == nil {
		return nil, fmt.Errorf("function %q not found in %s", functionName, filePath)
	}

	visitor := newTSDefUseVisitor(content, functionName)
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

func findTSFunction(node *sitter.Node, funcName string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_declaration" {
		funcNameNode := findChildByTypeTS(node, "identifier", content)
		if funcNameNode != nil {
			name := nodeTextTS(funcNameNode, content)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "method_definition" {
		propertyName := findChildByTypeTS(node, "property_name", content)
		if propertyName != nil {
			name := nodeTextTS(propertyName, content)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "lexical_declaration" {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "variable_declarator" {
				nameNode := findChildByTypeTS(child, "identifier", content)
				if nameNode != nil {
					name := nodeTextTS(nameNode, content)
					if name == funcName {
						return child
					}
				}
			}
		}
	}

	if node.Type() == "variable_declarator" {
		nameNode := findChildByTypeTS(node, "identifier", content)
		if nameNode != nil {
			name := nodeTextTS(nameNode, content)
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
		if child.Type() == "import_statement" ||
			child.Type() == "export_statement" ||
			child.Type() == "interface_declaration" ||
			child.Type() == "type_alias_declaration" ||
			child.Type() == "class_declaration" {
			continue
		}
		result := findTSFunction(child, funcName, content)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *tsDefUseVisitor) extractReferences(funcNode *sitter.Node) {
	if funcNode == nil {
		return
	}

	v.extractParameters(funcNode)

	blockNode := findBlockTS(funcNode)
	if blockNode == nil {
		blockNode = findChildByTypeTS(funcNode, "statement_block", v.content)
		if blockNode == nil {
			expressionBody := findChildByTypeTS(funcNode, "expression_statement", v.content)
			if expressionBody != nil {
				v.extractUses(expressionBody)
			}
			return
		}
	}

	v.walkBlock(blockNode)
}

func (v *tsDefUseVisitor) extractParameters(funcNode *sitter.Node) {
	paramsNode := findChildByTypeTS(funcNode, "formal_parameters", v.content)
	if paramsNode == nil {
		paramsNode = findChildByTypeTS(funcNode, "parameter_list", v.content)
	}
	if paramsNode == nil {
		return
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "required_parameter" ||
			child.Type() == "parameter" ||
			child.Type() == "optional_parameter" {
			v.extractParameterNames(child)
		}
	}

	if funcNode.Type() == "method_definition" {
		thisNode := findChildByTypeTS(funcNode, "this", v.content)
		if thisNode != nil {
			ref := VarRef{
				Name:    "this",
				RefType: RefTypeDefinition,
				Line:    int(thisNode.StartPoint().Row) + 1,
				Column:  int(thisNode.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
	}
}

func (v *tsDefUseVisitor) extractParameterNames(paramNode *sitter.Node) {
	if paramNode == nil {
		return
	}

	identifier := findChildByTypeTS(paramNode, "identifier", v.content)
	if identifier != nil {
		name := nodeTextTS(identifier, v.content)
		if name != "" && !isTSBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: RefTypeDefinition,
				Line:    int(identifier.StartPoint().Row) + 1,
				Column:  int(identifier.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
		return
	}

	rest := findChildByTypeTS(paramNode, "rest_parameter", v.content)
	if rest != nil {
		v.extractParameterNames(rest)
		return
	}

	objectPattern := findChildByTypeTS(paramNode, "object_pattern", v.content)
	if objectPattern == nil {
		objectPattern = findChildByTypeTS(paramNode, "array_pattern", v.content)
	}
	if objectPattern != nil {
		v.extractDestructuring(objectPattern, RefTypeDefinition)
	}
}

func (v *tsDefUseVisitor) walkBlock(blockNode *sitter.Node) {
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

func (v *tsDefUseVisitor) processNode(node *sitter.Node) {
	switch node.Type() {
	case "variable_declaration":
		v.processVariableDeclaration(node)
	case "lexical_declaration":
		v.processLexicalDeclaration(node)
	case "assignment_expression":
		v.processAssignmentExpression(node)
	case "update_expression":
		v.processUpdateExpression(node)
	case "for_statement":
		v.processForStatement(node)
	case "for_in_statement":
		v.processForInStatement(node)
	case "for_of_statement":
		v.processForOfStatement(node)
	case "return_statement":
		v.processReturnStatement(node)
	case "if_statement":
		v.processIfStatement(node)
	case "switch_statement":
		v.processSwitchStatement(node)
	case "try_statement":
		v.processTryStatement(node)
	case "catch_clause":
		v.processCatchClause(node)
	case "arrow_function":
		v.extractParameters(node)
	case "function_declaration":
		v.extractParameters(node)
	case "method_definition":
		v.extractParameters(node)
	case "block":
		v.walkBlock(node)
	default:
		v.extractUses(node)
	}
}

func (v *tsDefUseVisitor) processVariableDeclaration(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "variable_declarator" {
			v.processVariableDeclarator(child, RefTypeDefinition)
		}
	}
	v.extractUses(node)
}

func (v *tsDefUseVisitor) processLexicalDeclaration(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "variable_declarator" {
			v.processVariableDeclarator(child, RefTypeDefinition)
		}
	}
	v.extractUses(node)
}

func (v *tsDefUseVisitor) processVariableDeclarator(node *sitter.Node, refType RefType) {
	nameNode := findChildByTypeTS(node, "identifier", v.content)
	if nameNode != nil {
		name := nodeTextTS(nameNode, v.content)
		if name != "" && !isTSBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: refType,
				Line:    int(nameNode.StartPoint().Row) + 1,
				Column:  int(nameNode.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
	}

	objectPattern := findChildByTypeTS(node, "object_pattern", v.content)
	if objectPattern != nil {
		v.extractDestructuring(objectPattern, refType)
	}

	arrayPattern := findChildByTypeTS(node, "array_pattern", v.content)
	if arrayPattern != nil {
		v.extractDestructuring(arrayPattern, refType)
	}
}

func (v *tsDefUseVisitor) extractDestructuring(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "object_pattern":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Type() == "shorthand_property_identifier" ||
				child.Type() == "property_identifier" {
				name := nodeTextTS(child, v.content)
				if name != "" && !isTSBuiltin(name) {
					ref := VarRef{
						Name:    name,
						RefType: refType,
						Line:    int(child.StartPoint().Row) + 1,
						Column:  int(child.StartPoint().Column) + 1,
					}
					v.addRef(ref)
				}
			} else if child.Type() == "object_pattern" ||
				child.Type() == "array_pattern" ||
				child.Type() == "pair_pattern" {
				v.extractDestructuring(child, refType)
			} else if child.Type() == "rest_pattern" ||
				child.Type() == "rest_element" {
				restId := findChildByTypeTS(child, "identifier", v.content)
				if restId != nil {
					name := nodeTextTS(restId, v.content)
					if name != "" && !isTSBuiltin(name) {
						ref := VarRef{
							Name:    name,
							RefType: refType,
							Line:    int(restId.StartPoint().Row) + 1,
							Column:  int(restId.StartPoint().Column) + 1,
						}
						v.addRef(ref)
					}
				}
			}
		}

	case "array_pattern":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Type() == "identifier" {
				name := nodeTextTS(child, v.content)
				if name != "" && !isTSBuiltin(name) {
					ref := VarRef{
						Name:    name,
						RefType: refType,
						Line:    int(child.StartPoint().Row) + 1,
						Column:  int(child.StartPoint().Column) + 1,
					}
					v.addRef(ref)
				}
			} else if child.Type() == "array_pattern" ||
				child.Type() == "object_pattern" {
				v.extractDestructuring(child, refType)
			} else if child.Type() == "rest_pattern" ||
				child.Type() == "rest_element" {
				restId := findChildByTypeTS(child, "identifier", v.content)
				if restId != nil {
					name := nodeTextTS(restId, v.content)
					if name != "" && !isTSBuiltin(name) {
						ref := VarRef{
							Name:    name,
							RefType: refType,
							Line:    int(restId.StartPoint().Row) + 1,
							Column:  int(restId.StartPoint().Column) + 1,
						}
						v.addRef(ref)
					}
				}
			}
		}
	}
}

func (v *tsDefUseVisitor) processAssignmentExpression(node *sitter.Node) {
	left := findChildByTypeTS(node, "left", v.content)
	if left != nil {
		text := nodeTextTS(node, v.content)
		isCompound := len(text) > 0 && (text[0] == '+' || text[0] == '-' || text[0] == '*' || text[0] == '/')
		if isCompound {
			v.extractAssignmentTarget(left, RefTypeUpdate)
		} else {
			v.extractAssignmentTarget(left, RefTypeDefinition)
		}
	}
	v.extractUses(node)
}

func (v *tsDefUseVisitor) processUpdateExpression(node *sitter.Node) {
	argument := findChildByTypeTS(node, "argument", v.content)
	if argument != nil {
		v.extractAssignmentTarget(argument, RefTypeUpdate)
	}
}

func (v *tsDefUseVisitor) extractAssignmentTarget(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "identifier":
		name := nodeTextTS(node, v.content)
		if name != "" && !isTSBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: refType,
				Line:    int(node.StartPoint().Row) + 1,
				Column:  int(node.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
	case "member_expression":
		object := findChildByTypeTS(node, "object", v.content)
		if object != nil {
			v.extractAssignmentTarget(object, refType)
		}
	case "element_expression":
		v.extractIdentifiers(node, refType)
	case "object_pattern", "array_pattern":
		v.extractDestructuring(node, refType)
	default:
		v.extractIdentifiers(node, refType)
	}
}

func (v *tsDefUseVisitor) processForStatement(node *sitter.Node) {
	init := findChildByTypeTS(node, "init", v.content)
	if init != nil {
		v.processNode(init)
	}

	test := findChildByTypeTS(node, "test", v.content)
	if test != nil {
		v.extractUses(test)
	}

	update := findChildByTypeTS(node, "update", v.content)
	if update != nil {
		v.extractUses(update)
	}

	body := findChildByTypeTS(node, "body", v.content)
	if body != nil {
		if body.Type() == "block" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}
}

func (v *tsDefUseVisitor) processForInStatement(node *sitter.Node) {
	left := findChildByTypeTS(node, "left", v.content)
	if left != nil {
		v.extractAssignmentTarget(left, RefTypeDefinition)
	}

	right := findChildByTypeTS(node, "right", v.content)
	if right != nil {
		v.extractUses(right)
	}

	body := findChildByTypeTS(node, "body", v.content)
	if body != nil {
		if body.Type() == "block" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}
}

func (v *tsDefUseVisitor) processForOfStatement(node *sitter.Node) {
	left := findChildByTypeTS(node, "left", v.content)
	if left != nil {
		v.extractAssignmentTarget(left, RefTypeDefinition)
	}

	right := findChildByTypeTS(node, "right", v.content)
	if right != nil {
		v.extractUses(right)
	}

	body := findChildByTypeTS(node, "body", v.content)
	if body != nil {
		if body.Type() == "block" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}
}

func (v *tsDefUseVisitor) processReturnStatement(node *sitter.Node) {
	v.extractUses(node)
}

func (v *tsDefUseVisitor) processIfStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "condition":
			v.extractUses(child)
		case "consequence", "alternative":
			if child.Type() == "block" {
				v.walkBlock(child)
			} else {
				v.processNode(child)
			}
		}
	}
}

func (v *tsDefUseVisitor) processSwitchStatement(node *sitter.Node) {
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

func (v *tsDefUseVisitor) processCaseClause(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "block" {
			v.walkBlock(child)
		} else if child.Type() != "case" && child.Type() != "pattern" {
			v.processNode(child)
		}
	}
}

func (v *tsDefUseVisitor) processTryStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "block":
			v.walkBlock(child)
		case "catch_clause":
			v.processCatchClause(child)
		case "finally":
			finallyBlock := findChildByTypeTS(child, "block", v.content)
			if finallyBlock != nil {
				v.walkBlock(finallyBlock)
			}
		}
	}
}

func (v *tsDefUseVisitor) processCatchClause(node *sitter.Node) {
	parameter := findChildByTypeTS(node, "parameter", v.content)
	if parameter != nil {
		identifier := findChildByTypeTS(parameter, "identifier", v.content)
		if identifier != nil {
			name := nodeTextTS(identifier, v.content)
			if name != "" && !isTSBuiltin(name) {
				ref := VarRef{
					Name:    name,
					RefType: RefTypeDefinition,
					Line:    int(identifier.StartPoint().Row) + 1,
					Column:  int(identifier.StartPoint().Column) + 1,
				}
				v.addRef(ref)
			}
		}

		objectPattern := findChildByTypeTS(parameter, "object_pattern", v.content)
		if objectPattern != nil {
			v.extractDestructuring(objectPattern, RefTypeDefinition)
		}
	}

	body := findChildByTypeTS(node, "block", v.content)
	if body != nil {
		v.walkBlock(body)
	}
}

func (v *tsDefUseVisitor) extractUses(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextTS(node, v.content)
		if name != "" && !isTSBuiltin(name) {
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

func (v *tsDefUseVisitor) extractIdentifiers(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextTS(node, v.content)
		if name != "" && !isTSBuiltin(name) {
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
			v.extractIdentifiers(child, refType)
		}
	}
}

func (v *tsDefUseVisitor) isDefinitionAtPosition(node *sitter.Node) bool {
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

func (v *tsDefUseVisitor) addRef(ref VarRef) {
	v.refs = append(v.refs, ref)
	v.variables[ref.Name] = append(v.variables[ref.Name], ref)
}

func findBlockTS(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "block" {
		return node
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			result := findBlockTS(child)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

func findChildByTypeTS(node *sitter.Node, childType string, content []byte) *sitter.Node {
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

func nodeTextTS(node *sitter.Node, content []byte) string {
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

func isTSBuiltin(name string) bool {
	builtins := map[string]bool{
		"console": true, "Math": true, "JSON": true, "Object": true,
		"Array": true, "String": true, "Number": true, "Boolean": true,
		"Date": true, "RegExp": true, "Error": true, "Map": true,
		"Set": true, "WeakMap": true, "WeakSet": true, "Promise": true,
		"Symbol": true, "BigInt": true, "Proxy": true, "Reflect": true,

		"break": true, "case": true, "catch": true, "continue": true,
		"debugger": true, "default": true, "delete": true, "do": true,
		"else": true, "export": true, "extends": true, "finally": true,
		"for": true, "function": true, "if": true, "import": true,
		"in": true, "instanceof": true, "new": true, "return": true,
		"super": true, "switch": true, "this": true, "throw": true,
		"try": true, "typeof": true, "var": true, "void": true,
		"while": true, "with": true, "yield": true, "class": true,
		"const": true, "let": true, "enum": true, "implements": true,
		"interface": true, "package": true, "private": true,
		"protected": true, "public": true, "static": true,
		"abstract": true, "as": true, "async": true, "await": true,
		"declare": true, "from": true, "get": true, "is": true,
		"keyof": true, "module": true, "namespace": true, "never": true,
		"of": true, "readonly": true, "require": true, "set": true,
		"type": true, "undefined": true, "null": true, "true": true,
		"false": true, "Infinity": true, "NaN": true,

		"window": true, "document": true, "navigator": true, "localStorage": true,
		"sessionStorage": true, "location": true, "history": true,
	}

	if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
		return true
	}

	return builtins[name]
}
