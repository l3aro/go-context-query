package dfg

import (
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

type phpDefUseVisitor struct {
	content   []byte
	funcName  string
	refs      []VarRef
	variables map[string][]VarRef
}

func newPhpDefUseVisitor(content []byte, funcName string) *phpDefUseVisitor {
	return &phpDefUseVisitor{
		content:   content,
		funcName:  funcName,
		refs:      make([]VarRef, 0),
		variables: make(map[string][]VarRef),
	}
}

func extractPhpDFG(filePath string, functionName string) (*DFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractPhpCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(php.GetLanguage())
	tree := parser.Parse(nil, content)
	defer tree.Close()

	root := tree.RootNode()
	funcNode := findPhpFunction(root, functionName, content)
	if funcNode == nil {
		return nil, fmt.Errorf("function %q not found in %s", functionName, filePath)
	}

	visitor := newPhpDefUseVisitor(content, functionName)
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

func findPhpFunction(node *sitter.Node, funcName string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_definition" {
		funcNameNode := findChildByTypePhp(node, "name", content)
		if funcNameNode != nil {
			name := nodeTextPhp(funcNameNode, content)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "class_declaration" {
		classBody := findChildByTypePhp(node, "body", content)
		if classBody != nil {
			for i := 0; i < int(classBody.ChildCount()); i++ {
				child := classBody.Child(i)
				if child != nil {
					result := findPhpFunction(child, funcName, content)
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
		if child.Type() == "comment" {
			continue
		}
		result := findPhpFunction(child, funcName, content)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *phpDefUseVisitor) extractReferences(funcNode *sitter.Node) {
	if funcNode == nil {
		return
	}

	v.extractParameters(funcNode)

	blockNode := findBlockPhp(funcNode)
	if blockNode == nil {
		return
	}

	v.walkBlock(blockNode)
}

func (v *phpDefUseVisitor) extractParameters(funcNode *sitter.Node) {
	paramsNode := findChildByTypePhp(funcNode, "parameters", v.content)
	if paramsNode == nil {
		return
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "optional_parameter" || child.Type() == "variadic_parameter" {
			v.extractParameterFromNode(child)
		} else if child.Type() == "parameter" {
			v.extractParameterFromNode(child)
		}
	}
}

func (v *phpDefUseVisitor) extractParameterFromNode(paramNode *sitter.Node) {
	if paramNode == nil {
		return
	}

	for i := 0; i < int(paramNode.ChildCount()); i++ {
		child := paramNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "variable_name" || child.Type() == "variable" {
			name := nodeTextPhp(child, v.content)
			if name != "" && !isPhpBuiltin(name) {
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

func (v *phpDefUseVisitor) walkBlock(blockNode *sitter.Node) {
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

func (v *phpDefUseVisitor) processNode(node *sitter.Node) {
	switch node.Type() {
	case "assignment_expression":
		v.processAssignment(node)
	case "augmented_assignment_expression":
		v.processAssignment(node)
	case "update_expression":
		v.processUpdateExpression(node)
	case "for":
		v.processForStatement(node)
	case "foreach":
		v.processForeachStatement(node)
	case "while":
		v.processWhileStatement(node)
	case "do":
		v.processDoStatement(node)
	case "if":
		v.processIfStatement(node)
	case "switch":
		v.processSwitchStatement(node)
	case "try":
		v.processTryStatement(node)
	case "return", "break", "continue", "goto":
		v.processJumpStatement(node)
	case "throw":
		v.processThrowStatement(node)
	case "compound_statement", "body":
		v.walkBlock(node)
	default:
		v.extractUses(node)
	}
}

func (v *phpDefUseVisitor) processAssignment(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "variable_name" || child.Type() == "variable" {
			name := nodeTextPhp(child, v.content)
			if name != "" && !isPhpBuiltin(name) {
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

func (v *phpDefUseVisitor) processUpdateExpression(node *sitter.Node) {
	v.extractUses(node)
}

func (v *phpDefUseVisitor) processForStatement(node *sitter.Node) {
	initializer := findChildByTypePhp(node, "initializer", v.content)
	if initializer != nil {
		v.extractUses(initializer)
	}

	condition := findChildByTypePhp(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	update := findChildByTypePhp(node, "update", v.content)
	if update != nil {
		v.extractUses(update)
	}

	body := findChildByTypePhp(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}
}

func (v *phpDefUseVisitor) processForeachStatement(node *sitter.Node) {
	iterable := findChildByTypePhp(node, "iterable", v.content)
	if iterable != nil {
		v.extractUses(iterable)
	}

	value := findChildByTypePhp(node, "value", v.content)
	if value != nil {
		v.extractIdentifiersFromNode(value, RefTypeDefinition)
	}

	key := findChildByTypePhp(node, "key", v.content)
	if key != nil {
		v.extractIdentifiersFromNode(key, RefTypeDefinition)
	}

	body := findChildByTypePhp(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}
}

func (v *phpDefUseVisitor) processWhileStatement(node *sitter.Node) {
	condition := findChildByTypePhp(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	body := findChildByTypePhp(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}
}

func (v *phpDefUseVisitor) processDoStatement(node *sitter.Node) {
	body := findChildByTypePhp(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}

	condition := findChildByTypePhp(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}
}

func (v *phpDefUseVisitor) processIfStatement(node *sitter.Node) {
	condition := findChildByTypePhp(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	consequence := findChildByTypePhp(node, "consequence", v.content)
	if consequence != nil {
		v.walkBlock(consequence)
	}

	alternative := findChildByTypePhp(node, "alternative", v.content)
	if alternative != nil {
		v.walkBlock(alternative)
	}
}

func (v *phpDefUseVisitor) processSwitchStatement(node *sitter.Node) {
	condition := findChildByTypePhp(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	body := findChildByTypePhp(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}
}

func (v *phpDefUseVisitor) processTryStatement(node *sitter.Node) {
	body := findChildByTypePhp(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}

	catchClauses := findChildByTypePhp(node, "catch_clause", v.content)
	if catchClauses != nil {
		v.processCatchClause(catchClauses)
	}
}

func (v *phpDefUseVisitor) processCatchClause(node *sitter.Node) {
	v.walkBlock(node)
}

func (v *phpDefUseVisitor) processJumpStatement(node *sitter.Node) {
	v.extractUses(node)
}

func (v *phpDefUseVisitor) processThrowStatement(node *sitter.Node) {
	v.extractUses(node)
}

func (v *phpDefUseVisitor) extractUses(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "variable_name" || node.Type() == "variable" {
		name := nodeTextPhp(node, v.content)
		if name != "" && !isPhpBuiltin(name) {
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

func (v *phpDefUseVisitor) isDefinitionAtPosition(node *sitter.Node) bool {
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

func (v *phpDefUseVisitor) extractIdentifiersFromNode(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	if node.Type() == "variable_name" || node.Type() == "variable" {
		name := nodeTextPhp(node, v.content)
		if name != "" && !isPhpBuiltin(name) {
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

func (v *phpDefUseVisitor) addRef(ref VarRef) {
	v.refs = append(v.refs, ref)
	v.variables[ref.Name] = append(v.variables[ref.Name], ref)
}

func findBlockPhp(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "compound_statement" || node.Type() == "body" {
		return node
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			result := findBlockPhp(child)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

func findChildByTypePhp(node *sitter.Node, childType string, content []byte) *sitter.Node {
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

func nodeTextPhp(node *sitter.Node, content []byte) string {
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

func isPhpBuiltin(name string) bool {
	builtins := map[string]bool{
		"array": true, "callable": true, "class": true, "false": true,
		"float": true, "int": true, "iterable": true, "mixed": true,
		"never": true, "null": true, "object": true, "parent": true,
		"self": true, "static": true, "string": true, "true": true,
		"void": true,

		"echo": true, "print": true, "die": true, "exit": true,
		"include": true, "include_once": true, "require": true, "require_once": true,
		"function": true, "interface": true, "trait": true,
		"extends": true, "implements": true, "abstract": true, "final": true,
		"public": true, "private": true, "protected": true,
		"const": true, "var": true, "new": true, "clone": true,
		"instanceof": true, "return": true, "break": true, "continue": true,
		"goto": true, "throw": true, "try": true, "catch": true,
		"finally": true, "if": true, "else": true, "elseif": true,
		"switch": true, "case": true, "default": true, "for": true,
		"foreach": true, "while": true, "do": true, "as": true,
		"yield": true, "yield from": true, "use": true, "namespace": true,
		"global": true, "list": true, "empty": true, "isset": true,
		"unset": true,

		"count": true, "sizeof": true, "strlen": true, "strpos": true,
		"str_replace": true, "substr": true, "explode": true, "implode": true,
		"array_push": true, "array_pop": true, "array_shift": true, "array_unshift": true,
		"array_merge": true, "array_keys": true, "array_values": true, "in_array": true,
		"file_get_contents": true, "file_put_contents": true, "fopen": true, "fclose": true,
		"fread": true, "fwrite": true, "print_r": true, "var_dump": true,
		"json_encode": true, "json_decode": true, "date": true, "time": true,
		"mktime": true, "strtotime": true, "header": true, "setcookie": true,
		"session_start": true, "md5": true, "sha1": true, "password_hash": true,
		"password_verify": true, "htmlspecialchars": true, "strip_tags": true,
		"trim": true, "ltrim": true, "rtrim": true, "ucfirst": true, "lcfirst": true,
		"strtolower": true, "strtoupper": true, "sprintf": true, "printf": true,
	}

	return builtins[name]
}
