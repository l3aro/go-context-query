package dfg

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

type pythonDefUseVisitor struct {
	content   []byte
	funcName  string
	refs      []VarRef
	variables map[string][]VarRef
}

func newPythonDefUseVisitor(content []byte, funcName string) *pythonDefUseVisitor {
	return &pythonDefUseVisitor{
		content:   content,
		funcName:  funcName,
		refs:      make([]VarRef, 0),
		variables: make(map[string][]VarRef),
	}
}

func extractPythonDFG(filePath string, functionName string) (*DFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())
	tree := parser.Parse(nil, content)
	defer tree.Close()

	root := tree.RootNode()
	funcNode := findPythonFunction(root, functionName, content)
	if funcNode == nil {
		return nil, fmt.Errorf("function %q not found in %s", functionName, filePath)
	}

	visitor := newPythonDefUseVisitor(content, functionName)
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

func findPythonFunction(node *sitter.Node, funcName string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_definition" {
		funcNameNode := findChildByType(node, "identifier", content)
		if funcNameNode != nil {
			name := nodeText(funcNameNode, content)
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
		if child.Type() == "class_definition" {
			continue
		}
		result := findPythonFunction(child, funcName, content)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *pythonDefUseVisitor) extractReferences(funcNode *sitter.Node) {
	if funcNode == nil {
		return
	}

	blockNode := findBlock(funcNode)
	if blockNode == nil {
		return
	}

	v.extractParameters(funcNode)
	v.walkBlock(blockNode)
}

func (v *pythonDefUseVisitor) extractParameters(funcNode *sitter.Node) {
	paramsNode := findChildByType(funcNode, "parameters", v.content)
	if paramsNode == nil {
		return
	}

	paramTypes := []string{
		"positional_or_keyword_parameter",
		"keyword_parameter",
		"optional_parameter",
		"variadic_parameter",
		"dictionary_variadic_parameter",
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		for _, paramType := range paramTypes {
			if child.Type() == paramType {
				identifier := findChildByType(child, "identifier", v.content)
				if identifier != nil {
					name := nodeText(identifier, v.content)
					if name != "" {
						ref := VarRef{
							Name:    name,
							RefType: RefTypeDefinition,
							Line:    int(identifier.StartPoint().Row) + 1,
							Column:  int(identifier.StartPoint().Column) + 1,
						}
						v.addRef(ref)
					}
				}
			}
		}
	}
}

func (v *pythonDefUseVisitor) walkBlock(blockNode *sitter.Node) {
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

func (v *pythonDefUseVisitor) processNode(node *sitter.Node) {
	switch node.Type() {
	case "assignment":
		v.processAssignment(node)
	case "augmented_assignment":
		v.processAugmentedAssignment(node)
	case "for_statement":
		v.processForStatement(node)
	case "named_expression":
		v.processNamedExpression(node)
	case "return_statement":
		v.processReturnStatement(node)
	case "with_statement":
		v.processWithStatement(node)
	case "try_statement":
		v.processTryStatement(node)
	case "function_definition":
		v.extractParameters(node)
	case "block":
		v.walkBlock(node)
	default:
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil {
				v.processNode(child)
			}
		}
	}
}

func (v *pythonDefUseVisitor) processAssignment(node *sitter.Node) {
	left := findChildByType(node, "left", v.content)
	if left == nil {
		v.walkChildren(node)
		return
	}

	v.extractAssignmentTarget(left, RefTypeDefinition)
	v.walkChildren(node)
}

func (v *pythonDefUseVisitor) processAugmentedAssignment(node *sitter.Node) {
	left := findChildByType(node, "left", v.content)
	if left != nil {
		v.extractAssignmentTarget(left, RefTypeUpdate)
	}
	v.walkChildren(node)
}

func (v *pythonDefUseVisitor) processForStatement(node *sitter.Node) {
	left := findChildByType(node, "left", v.content)
	if left != nil {
		v.extractAssignmentTarget(left, RefTypeDefinition)
	}

	right := findChildByType(node, "right", v.content)
	if right != nil {
		v.extractUses(right)
	}

	body := findChildByType(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}

	elseClause := findChildByType(node, "else", v.content)
	if elseClause != nil {
		v.walkBlock(elseClause)
	}
}

func (v *pythonDefUseVisitor) processNamedExpression(node *sitter.Node) {
	left := findChildByType(node, "left", v.content)
	if left != nil {
		v.extractAssignmentTarget(left, RefTypeDefinition)
	}
	v.walkChildren(node)
}

func (v *pythonDefUseVisitor) processReturnStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() != "return" {
			v.extractUses(child)
		}
	}
}

func (v *pythonDefUseVisitor) processWithStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "with_item" {
			v.extractUses(child)
		}
	}

	body := findChildByType(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}
}

func (v *pythonDefUseVisitor) processTryStatement(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "block":
			v.walkBlock(child)
		case "exception_handler":
			v.extractExceptionVariable(child)
			v.walkBlock(child)
		case "finally":
			v.walkBlock(child)
		}
	}
}

func (v *pythonDefUseVisitor) extractExceptionVariable(handlerNode *sitter.Node) {
	if handlerNode == nil {
		return
	}

	for i := 0; i < int(handlerNode.ChildCount()); i++ {
		child := handlerNode.Child(i)
		if child != nil && child.Type() == "as_pattern" {
			identifier := findChildByType(child, "identifier", v.content)
			if identifier != nil {
				name := nodeText(identifier, v.content)
				if name != "" {
					ref := VarRef{
						Name:    name,
						RefType: RefTypeDefinition,
						Line:    int(identifier.StartPoint().Row) + 1,
						Column:  int(identifier.StartPoint().Column) + 1,
					}
					v.addRef(ref)
				}
			}
		}
	}
}

func (v *pythonDefUseVisitor) extractAssignmentTarget(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "identifier":
		name := nodeText(node, v.content)
		if name != "" && !isBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: refType,
				Line:    int(node.StartPoint().Row) + 1,
				Column:  int(node.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
	case "attribute":
		v.extractUses(node)
	case "subscript":
		v.extractUses(node)
	case "tuple", "list", "set":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil {
				v.extractAssignmentTarget(child, refType)
			}
		}
	case "star_expression":
		child := findChildByType(node, "identifier", v.content)
		if child != nil {
			v.extractAssignmentTarget(child, refType)
		}
	default:
		v.extractIdentifiers(node, refType)
	}
}

func (v *pythonDefUseVisitor) extractUses(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeText(node, v.content)
		if name != "" && !isBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: RefTypeUse,
				Line:    int(node.StartPoint().Row) + 1,
				Column:  int(node.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
		return
	}

	v.walkChildren(node)
}

func (v *pythonDefUseVisitor) extractIdentifiers(node *sitter.Node, refType RefType) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeText(node, v.content)
		if name != "" && !isBuiltin(name) {
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

func (v *pythonDefUseVisitor) walkChildren(node *sitter.Node) {
	if node == nil {
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			v.processNode(child)
		}
	}
}

func (v *pythonDefUseVisitor) addRef(ref VarRef) {
	v.refs = append(v.refs, ref)
	v.variables[ref.Name] = append(v.variables[ref.Name], ref)
}

func findBlock(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "block" {
		return node
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			result := findBlock(child)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

func findChildByType(node *sitter.Node, childType string, content []byte) *sitter.Node {
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

func nodeText(node *sitter.Node, content []byte) string {
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

func isBuiltin(name string) bool {
	builtins := map[string]bool{
		"abs": true, "all": true, "any": true, "ascii": true,
		"bin": true, "bool": true, "breakpoint": true, "bytearray": true,
		"bytes": true, "callable": true, "chr": true, "classmethod": true,
		"compile": true, "complex": true, "delattr": true, "dict": true,
		"dir": true, "divmod": true, "enumerate": true, "eval": true,
		"exec": true, "filter": true, "float": true, "format": true,
		"frozenset": true, "getattr": true, "globals": true, "hasattr": true,
		"hash": true, "help": true, "hex": true, "id": true,
		"input": true, "int": true, "isinstance": true, "issubclass": true,
		"iter": true, "len": true, "list": true, "locals": true,
		"map": true, "max": true, "memoryview": true, "min": true,
		"next": true, "object": true, "oct": true, "open": true,
		"ord": true, "pow": true, "print": true, "property": true,
		"range": true, "repr": true, "reversed": true, "round": true,
		"set": true, "setattr": true, "slice": true, "sorted": true,
		"staticmethod": true, "str": true, "sum": true, "super": true,
		"tuple": true, "type": true, "vars": true, "zip": true,
		"True": true, "False": true, "None": true, "NotImplemented": true,
		"self": true, "cls": true,
	}

	if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
		return true
	}

	return builtins[name]
}
