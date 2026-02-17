package dfg

import (
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

type javaDefUseVisitor struct {
	content   []byte
	funcName  string
	refs      []VarRef
	variables map[string][]VarRef
}

func newJavaDefUseVisitor(content []byte, funcName string) *javaDefUseVisitor {
	return &javaDefUseVisitor{
		content:   content,
		funcName:  funcName,
		refs:      make([]VarRef, 0),
		variables: make(map[string][]VarRef),
	}
}

func extractJavaDFG(filePath string, functionName string) (*DFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractJavaCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(java.GetLanguage())
	tree := parser.Parse(nil, content)
	defer tree.Close()

	root := tree.RootNode()
	funcNode := findJavaMethod(root, functionName, content)
	if funcNode == nil {
		return nil, fmt.Errorf("method %q not found in %s", functionName, filePath)
	}

	visitor := newJavaDefUseVisitor(content, functionName)
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

func findJavaMethod(node *sitter.Node, funcName string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "method_declaration" {
		funcNameNode := findChildByTypeJava(node, "identifier", content)
		if funcNameNode != nil {
			name := nodeTextJava(funcNameNode, content)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "constructor_declaration" {
		funcNameNode := findChildByTypeJava(node, "identifier", content)
		if funcNameNode != nil {
			name := nodeTextJava(funcNameNode, content)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "class_declaration" {
		classBody := findChildByTypeJava(node, "class_body", content)
		if classBody != nil {
			for i := 0; i < int(classBody.ChildCount()); i++ {
				child := classBody.Child(i)
				if child != nil {
					result := findJavaMethod(child, funcName, content)
					if result != nil {
						return result
					}
				}
			}
		}
	}

	if node.Type() == "interface_declaration" {
		interfaceBody := findChildByTypeJava(node, "interface_body", content)
		if interfaceBody != nil {
			for i := 0; i < int(interfaceBody.ChildCount()); i++ {
				child := interfaceBody.Child(i)
				if child != nil {
					result := findJavaMethod(child, funcName, content)
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
		if child.Type() == "import_declaration" ||
			child.Type() == "package_declaration" {
			continue
		}
		result := findJavaMethod(child, funcName, content)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *javaDefUseVisitor) extractReferences(funcNode *sitter.Node) {
	if funcNode == nil {
		return
	}

	v.extractParameters(funcNode)

	blockNode := findBlockJava(funcNode)
	if blockNode == nil {
		return
	}

	v.walkBlock(blockNode)
}

func (v *javaDefUseVisitor) extractParameters(funcNode *sitter.Node) {
	paramsNode := findChildByTypeJava(funcNode, "formal_parameters", v.content)
	if paramsNode == nil {
		return
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "formal_parameter" {
			v.extractParameterNames(child)
		}
	}

	if funcNode.Type() == "method_declaration" {
		receiverNode := findChildByTypeJava(funcNode, "receiver", v.content)
		if receiverNode != nil {
			v.extractIdentifiersFromNode(receiverNode, RefTypeDefinition)
		}
	}
}

func (v *javaDefUseVisitor) extractParameterNames(paramNode *sitter.Node) {
	if paramNode == nil {
		return
	}

	variableName := findChildByTypeJava(paramNode, "variable_declarator", v.content)
	if variableName != nil {
		nameNode := findChildByTypeJava(variableName, "identifier", v.content)
		if nameNode != nil {
			name := nodeTextJava(nameNode, v.content)
			if name != "" && !isJavaBuiltin(name) {
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

	v.extractIdentifiersFromNode(paramNode, RefTypeDefinition)
}

func (v *javaDefUseVisitor) walkBlock(blockNode *sitter.Node) {
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

func (v *javaDefUseVisitor) processNode(node *sitter.Node) {
	switch node.Type() {
	case "local_variable_declaration":
		v.processLocalVariableDeclaration(node)
	case "assignment_expression":
		v.processAssignment(node)
	case "update_expression":
		v.processUpdateExpression(node)
	case "for_statement":
		v.processForStatement(node)
	case "enhanced_for_statement":
		v.processEnhancedForStatement(node)
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
	case "block":
		v.walkBlock(node)
	default:
		v.extractUses(node)
	}
}

func (v *javaDefUseVisitor) processLocalVariableDeclaration(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "variable_declarator" {
			nameNode := findChildByTypeJava(child, "identifier", v.content)
			if nameNode != nil {
				name := nodeTextJava(nameNode, v.content)
				if name != "" && !isJavaBuiltin(name) {
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

func (v *javaDefUseVisitor) processAssignment(node *sitter.Node) {
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

func (v *javaDefUseVisitor) processUpdateExpression(node *sitter.Node) {
	argument := findChildByTypeJava(node, "argument", v.content)
	if argument != nil {
		v.extractIdentifiersFromNode(argument, RefTypeUpdate)
	}

	v.extractUses(node)
}

func (v *javaDefUseVisitor) processForStatement(node *sitter.Node) {
	init := findChildByTypeJava(node, "init", v.content)
	if init != nil {
		v.processNode(init)
	}

	condition := findChildByTypeJava(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	update := findChildByTypeJava(node, "update", v.content)
	if update != nil {
		v.extractUses(update)
	}

	body := findChildByTypeJava(node, "body", v.content)
	if body != nil {
		if body.Type() == "block" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}
}

func (v *javaDefUseVisitor) processEnhancedForStatement(node *sitter.Node) {
	variable := findChildByTypeJava(node, "variable_declarator", v.content)
	if variable != nil {
		nameNode := findChildByTypeJava(variable, "identifier", v.content)
		if nameNode != nil {
			name := nodeTextJava(nameNode, v.content)
			if name != "" && !isJavaBuiltin(name) {
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

	value := findChildByTypeJava(node, "value", v.content)
	if value != nil {
		v.extractUses(value)
	}

	body := findChildByTypeJava(node, "body", v.content)
	if body != nil {
		if body.Type() == "block" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}
}

func (v *javaDefUseVisitor) processWhileStatement(node *sitter.Node) {
	condition := findChildByTypeJava(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	body := findChildByTypeJava(node, "body", v.content)
	if body != nil {
		if body.Type() == "block" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}
}

func (v *javaDefUseVisitor) processDoStatement(node *sitter.Node) {
	body := findChildByTypeJava(node, "body", v.content)
	if body != nil {
		if body.Type() == "block" {
			v.walkBlock(body)
		} else {
			v.processNode(body)
		}
	}

	condition := findChildByTypeJava(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}
}

func (v *javaDefUseVisitor) processReturnStatement(node *sitter.Node) {
	v.extractUses(node)
}

func (v *javaDefUseVisitor) processIfStatement(node *sitter.Node) {
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

func (v *javaDefUseVisitor) processSwitchStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "condition":
			v.extractUses(child)
		case "switch_block":
			v.processSwitchBlock(child)
		}
	}
}

func (v *javaDefUseVisitor) processSwitchBlock(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "switch_case" || child.Type() == "switch_label" {
			v.extractUses(child)
		} else if child.Type() == "block" {
			v.walkBlock(child)
		} else {
			v.processNode(child)
		}
	}
}

func (v *javaDefUseVisitor) processTryStatement(node *sitter.Node) {
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
		case "finally_clause":
			finallyBlock := findChildByTypeJava(child, "block", v.content)
			if finallyBlock != nil {
				v.walkBlock(finallyBlock)
			}
		}
	}
}

func (v *javaDefUseVisitor) processCatchClause(node *sitter.Node) {
	parameter := findChildByTypeJava(node, "catch_formal_parameter", v.content)
	if parameter != nil {
		v.extractIdentifiersFromNode(parameter, RefTypeDefinition)
	}

	block := findChildByTypeJava(node, "block", v.content)
	if block != nil {
		v.walkBlock(block)
	}
}

func (v *javaDefUseVisitor) processThrowStatement(node *sitter.Node) {
	v.extractUses(node)
}

func (v *javaDefUseVisitor) extractUses(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextJava(node, v.content)
		if name != "" && !isJavaBuiltin(name) {
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

func (v *javaDefUseVisitor) isDefinitionAtPosition(node *sitter.Node) bool {
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

func (v *javaDefUseVisitor) extractIdentifiersFromNode(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextJava(node, v.content)
		if name != "" && !isJavaBuiltin(name) {
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

func (v *javaDefUseVisitor) addRef(ref VarRef) {
	v.refs = append(v.refs, ref)
	v.variables[ref.Name] = append(v.variables[ref.Name], ref)
}

func findBlockJava(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "block" {
		return node
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			result := findBlockJava(child)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

func findChildByTypeJava(node *sitter.Node, childType string, content []byte) *sitter.Node {
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

func nodeTextJava(node *sitter.Node, content []byte) string {
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

func isJavaBuiltin(name string) bool {
	builtins := map[string]bool{
		"abstract": true, "assert": true, "boolean": true, "break": true,
		"byte": true, "case": true, "catch": true, "char": true,
		"class": true, "const": true, "continue": true, "default": true,
		"do": true, "double": true, "else": true, "enum": true,
		"extends": true, "final": true, "finally": true, "float": true,
		"for": true, "goto": true, "if": true, "implements": true,
		"import": true, "instanceof": true, "int": true, "interface": true,
		"long": true, "native": true, "new": true, "package": true,
		"private": true, "protected": true, "public": true, "return": true,
		"short": true, "static": true, "strictfp": true, "super": true,
		"switch": true, "synchronized": true, "this": true, "throw": true,
		"throws": true, "transient": true, "try": true, "void": true,
		"volatile": true, "while": true,

		"true": true, "false": true, "null": true,

		"String": true, "Integer": true, "Boolean": true, "Double": true,
		"Float": true, "Long": true, "Byte": true, "Short": true,
		"Character": true, "Object": true, "Class": true, "System": true,
		"Math": true, "Thread": true, "Runnable": true, "Comparable": true,
		"Iterable": true, "Collection": true, "List": true, "Set": true,
		"Map": true, "ArrayList": true, "HashMap": true, "HashSet": true,
		"LinkedList": true, "Queue": true, "Stack": true, "Vector": true,
		"Arrays": true, "Collections": true, "Objects": true, "Optional": true,
		"Stream": true, "Scanner": true, "BufferedReader": true, "FileReader": true,
		"FileWriter": true, "BufferedWriter": true, "PrintWriter": true,
		"IOException": true, "FileNotFoundException": true, "RuntimeException": true,
		"Exception": true, "Error": true, "Throwable": true,
	}

	return builtins[name]
}
