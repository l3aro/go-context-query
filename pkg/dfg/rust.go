package dfg

import (
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

type rustDefUseVisitor struct {
	content   []byte
	funcName  string
	refs      []VarRef
	variables map[string][]VarRef
}

func newRustDefUseVisitor(content []byte, funcName string) *rustDefUseVisitor {
	return &rustDefUseVisitor{
		content:   content,
		funcName:  funcName,
		refs:      make([]VarRef, 0),
		variables: make(map[string][]VarRef),
	}
}

func extractRustDFG(filePath string, functionName string) (*DFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractRustCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(rust.GetLanguage())
	tree := parser.Parse(nil, content)
	defer tree.Close()

	root := tree.RootNode()
	funcNode := findRustFunction(root, functionName, content)
	if funcNode == nil {
		return nil, fmt.Errorf("function %q not found in %s", functionName, filePath)
	}

	visitor := newRustDefUseVisitor(content, functionName)
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

func findRustFunction(node *sitter.Node, funcName string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_item" {
		funcNameNode := findChildByTypeRust(node, "identifier", content)
		if funcNameNode != nil {
			name := nodeTextRust(funcNameNode, content)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "declaration" || node.Type() == "associated_item" {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "function_item" {
				result := findRustFunction(child, funcName, content)
				if result != nil {
					return result
				}
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "use_declaration" ||
			child.Type() == "attribute_item" ||
			child.Type() == "mod_item" ||
			child.Type() == "struct_item" ||
			child.Type() == "enum_item" ||
			child.Type() == "trait_item" ||
			child.Type() == "impl_item" {
			continue
		}
		result := findRustFunction(child, funcName, content)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *rustDefUseVisitor) extractReferences(funcNode *sitter.Node) {
	if funcNode == nil {
		return
	}

	v.extractParameters(funcNode)

	blockNode := findBlockRust(funcNode)
	if blockNode == nil {
		return
	}

	v.walkBlock(blockNode)
}

func (v *rustDefUseVisitor) extractParameters(funcNode *sitter.Node) {
	paramsNode := findChildByTypeRust(funcNode, "parameters", v.content)
	if paramsNode == nil {
		return
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "parameter" {
			v.extractParameterNames(child)
		}
	}

	selfNode := findChildByTypeRust(funcNode, "self_parameter", v.content)
	if selfNode != nil {
		ref := VarRef{
			Name:    "self",
			RefType: RefTypeDefinition,
			Line:    int(selfNode.StartPoint().Row) + 1,
			Column:  int(selfNode.StartPoint().Column) + 1,
		}
		v.addRef(ref)
	}
}

func (v *rustDefUseVisitor) extractParameterNames(paramNode *sitter.Node) {
	if paramNode == nil {
		return
	}

	identifier := findChildByTypeRust(paramNode, "identifier", v.content)
	if identifier != nil {
		name := nodeTextRust(identifier, v.content)
		if name != "" && !isRustBuiltin(name) {
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

	tuplePattern := findChildByTypeRust(paramNode, "tuple_pattern", v.content)
	if tuplePattern != nil {
		v.extractDestructuring(tuplePattern, RefTypeDefinition)
		return
	}

	refPattern := findChildByTypeRust(paramNode, "reference_pattern", v.content)
	if refPattern != nil {
		v.extractParameterNames(refPattern)
	}
}

func (v *rustDefUseVisitor) walkBlock(blockNode *sitter.Node) {
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

func (v *rustDefUseVisitor) processNode(node *sitter.Node) {
	switch node.Type() {
	case "let_declaration":
		v.processLetDeclaration(node)
	case "let_else_declaration":
		v.processLetElseDeclaration(node)
	case "assignment_expression":
		v.processAssignmentExpression(node)
	case "compound_assignment_expression":
		v.processCompoundAssignmentExpression(node)
	case "if_expression":
		v.processIfExpression(node)
	case "match_expression":
		v.processMatchExpression(node)
	case "loop_expression":
		v.processLoopExpression(node)
	case "while_expression":
		v.processWhileExpression(node)
	case "for_expression":
		v.processForExpression(node)
	case "return_expression":
		v.processReturnExpression(node)
	case "block":
		v.walkBlock(node)
	default:
		v.extractUses(node)
	}
}

func (v *rustDefUseVisitor) processLetDeclaration(node *sitter.Node) {
	pattern := findChildByTypeRust(node, "identifier", v.content)
	if pattern != nil {
		name := nodeTextRust(pattern, v.content)
		if name != "" && !isRustBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: RefTypeDefinition,
				Line:    int(pattern.StartPoint().Row) + 1,
				Column:  int(pattern.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
	}

	tuplePattern := findChildByTypeRust(node, "tuple_pattern", v.content)
	if tuplePattern != nil {
		v.extractDestructuring(tuplePattern, RefTypeDefinition)
	}

	structPattern := findChildByTypeRust(node, "struct_pattern", v.content)
	if structPattern != nil {
		v.extractStructDestructuring(structPattern, RefTypeDefinition)
	}

	v.extractUses(node)
}

func (v *rustDefUseVisitor) processLetElseDeclaration(node *sitter.Node) {
	pattern := findChildByTypeRust(node, "identifier", v.content)
	if pattern != nil {
		name := nodeTextRust(pattern, v.content)
		if name != "" && !isRustBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: RefTypeDefinition,
				Line:    int(pattern.StartPoint().Row) + 1,
				Column:  int(pattern.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
	}

	v.extractUses(node)
}

func (v *rustDefUseVisitor) processAssignmentExpression(node *sitter.Node) {
	left := findChildByTypeRust(node, "left", v.content)
	if left != nil {
		v.extractAssignmentTarget(left, RefTypeUpdate)
	}
	v.extractUses(node)
}

func (v *rustDefUseVisitor) processCompoundAssignmentExpression(node *sitter.Node) {
	v.extractAssignmentTarget(node, RefTypeUpdate)
	v.extractUses(node)
}

func (v *rustDefUseVisitor) extractAssignmentTarget(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "identifier":
		name := nodeTextRust(node, v.content)
		if name != "" && !isRustBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: refType,
				Line:    int(node.StartPoint().Row) + 1,
				Column:  int(node.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
	case "field_expression":
		object := findChildByTypeRust(node, "value", v.content)
		if object != nil {
			v.extractAssignmentTarget(object, refType)
		}
	case "index_expression":
		v.extractIdentifiers(node, refType)
	case "tuple_pattern", "tuple_struct_pattern":
		v.extractDestructuring(node, refType)
	case "struct_pattern":
		v.extractStructDestructuring(node, refType)
	default:
		v.extractIdentifiers(node, refType)
	}
}

func (v *rustDefUseVisitor) extractDestructuring(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "tuple_pattern":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Type() == "identifier" {
				name := nodeTextRust(child, v.content)
				if name != "" && !isRustBuiltin(name) {
					ref := VarRef{
						Name:    name,
						RefType: refType,
						Line:    int(child.StartPoint().Row) + 1,
						Column:  int(child.StartPoint().Column) + 1,
					}
					v.addRef(ref)
				}
			} else if child.Type() == "tuple_pattern" ||
				child.Type() == "tuple_struct_pattern" ||
				child.Type() == "wildcard_pattern" {
				v.extractDestructuring(child, refType)
			}
		}

	case "tuple_struct_pattern":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}

			if child.Type() == "identifier" {
				name := nodeTextRust(child, v.content)
				if name != "" && !isRustBuiltin(name) && name != "_" {
					ref := VarRef{
						Name:    name,
						RefType: refType,
						Line:    int(child.StartPoint().Row) + 1,
						Column:  int(child.StartPoint().Column) + 1,
					}
					v.addRef(ref)
				}
			} else if child.Type() == "tuple_pattern" ||
				child.Type() == "tuple_struct_pattern" ||
				child.Type() == "wildcard_pattern" {
				v.extractDestructuring(child, refType)
			}
		}
	}
}

func (v *rustDefUseVisitor) extractStructDestructuring(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "field_pattern" {
			identifier := findChildByTypeRust(child, "identifier", v.content)
			if identifier != nil {
				name := nodeTextRust(identifier, v.content)
				if name != "" && !isRustBuiltin(name) && name != "_" {
					ref := VarRef{
						Name:    name,
						RefType: refType,
						Line:    int(identifier.StartPoint().Row) + 1,
						Column:  int(identifier.StartPoint().Column) + 1,
					}
					v.addRef(ref)
				}
			}
		}
	}
}

func (v *rustDefUseVisitor) processIfExpression(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "condition":
			v.extractUses(child)
		case "block":
			v.walkBlock(child)
		case "else_clause":
			elseBlock := findChildByTypeRust(child, "block", v.content)
			if elseBlock != nil {
				v.walkBlock(elseBlock)
			} else {
				elseIf := findChildByTypeRust(child, "if_expression", v.content)
				if elseIf != nil {
					v.processIfExpression(elseIf)
				}
			}
		}
	}
}

func (v *rustDefUseVisitor) processMatchExpression(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "match_arm" {
			v.processMatchArm(child)
		} else if child.Type() != "match" && child.Type() != "{" && child.Type() != "}" {
			v.extractUses(child)
		}
	}
}

func (v *rustDefUseVisitor) processMatchArm(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "block" || child.Type() == "expression" {
			v.walkBlock(child)
		}
	}
}

func (v *rustDefUseVisitor) processLoopExpression(node *sitter.Node) {
	body := findChildByTypeRust(node, "block", v.content)
	if body != nil {
		v.walkBlock(body)
	}
}

func (v *rustDefUseVisitor) processWhileExpression(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "condition" {
			v.extractUses(child)
		} else if child.Type() == "block" {
			v.walkBlock(child)
		}
	}
}

func (v *rustDefUseVisitor) processForExpression(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "for_iterator" {
			identifier := findChildByTypeRust(child, "identifier", v.content)
			if identifier != nil {
				name := nodeTextRust(identifier, v.content)
				if name != "" && !isRustBuiltin(name) {
					ref := VarRef{
						Name:    name,
						RefType: RefTypeDefinition,
						Line:    int(identifier.StartPoint().Row) + 1,
						Column:  int(identifier.StartPoint().Column) + 1,
					}
					v.addRef(ref)
				}
			}
			v.extractUses(child)
		} else if child.Type() == "block" {
			v.walkBlock(child)
		}
	}
}

func (v *rustDefUseVisitor) processReturnExpression(node *sitter.Node) {
	v.extractUses(node)
}

func (v *rustDefUseVisitor) extractUses(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextRust(node, v.content)
		if name != "" && !isRustBuiltin(name) {
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

func (v *rustDefUseVisitor) isDefinitionAtPosition(node *sitter.Node) bool {
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

func (v *rustDefUseVisitor) extractIdentifiers(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextRust(node, v.content)
		if name != "" && !isRustBuiltin(name) {
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

func (v *rustDefUseVisitor) addRef(ref VarRef) {
	v.refs = append(v.refs, ref)
	v.variables[ref.Name] = append(v.variables[ref.Name], ref)
}

func findBlockRust(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "block" {
		return node
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			result := findBlockRust(child)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

func findChildByTypeRust(node *sitter.Node, childType string, content []byte) *sitter.Node {
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

func nodeTextRust(node *sitter.Node, content []byte) string {
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

func isRustBuiltin(name string) bool {
	builtins := map[string]bool{
		"as": true, "async": true, "await": true, "break": true,
		"const": true, "continue": true, "crate": true, "dyn": true,
		"else": true, "enum": true, "extern": true, "false": true,
		"fn": true, "for": true, "if": true, "impl": true,
		"in": true, "let": true, "loop": true, "match": true,
		"mod": true, "move": true, "mut": true, "pub": true,
		"ref": true, "return": true, "self": true, "Self": true,
		"static": true, "struct": true, "super": true, "trait": true,
		"true": true, "type": true, "unsafe": true, "use": true,
		"where": true, "while": true,
		"bool": true, "char": true, "f32": true, "f64": true,
		"i8": true, "i16": true, "i32": true, "i64": true, "i128": true,
		"isize": true, "u8": true, "u16": true, "u32": true, "u64": true,
		"u128": true, "usize": true, "str": true, "String": true,
		"Vec": true, "Option": true, "Result": true, "Box": true,
		"Rc": true, "Arc": true, "Cell": true, "RefCell": true,
		"println": true, "print": true, "eprintln": true, "eprint": true,
		"format": true, "panic": true, "assert": true, "assert_eq": true,
		"assert_ne": true, "vec": true, "Some": true, "None": true,
		"Ok": true, "Err": true, "macro_rules": true,
	}

	if len(name) > 2 && name[0] == '_' && name[1] == '_' {
		return true
	}

	return builtins[name]
}
