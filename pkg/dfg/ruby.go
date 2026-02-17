package dfg

import (
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

type rubyDefUseVisitor struct {
	content   []byte
	funcName  string
	refs      []VarRef
	variables map[string][]VarRef
}

func newRubyDefUseVisitor(content []byte, funcName string) *rubyDefUseVisitor {
	return &rubyDefUseVisitor{
		content:   content,
		funcName:  funcName,
		refs:      make([]VarRef, 0),
		variables: make(map[string][]VarRef),
	}
}

func extractRubyDFG(filePath string, functionName string) (*DFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractRubyCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(ruby.GetLanguage())
	tree := parser.Parse(nil, content)
	defer tree.Close()

	root := tree.RootNode()
	funcNode := findRubyMethod(root, functionName, content)
	if funcNode == nil {
		return nil, fmt.Errorf("method %q not found in %s", functionName, filePath)
	}

	visitor := newRubyDefUseVisitor(content, functionName)
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

func findRubyMethod(node *sitter.Node, funcName string, content []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "method" {
		funcNameNode := findChildByTypeRuby(node, "identifier", content)
		if funcNameNode != nil {
			name := nodeTextRuby(funcNameNode, content)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "class" {
		classBody := findChildByTypeRuby(node, "body", content)
		if classBody != nil {
			for i := 0; i < int(classBody.ChildCount()); i++ {
				child := classBody.Child(i)
				if child != nil {
					result := findRubyMethod(child, funcName, content)
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
		result := findRubyMethod(child, funcName, content)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *rubyDefUseVisitor) extractReferences(funcNode *sitter.Node) {
	if funcNode == nil {
		return
	}

	v.extractParameters(funcNode)

	blockNode := findBlockRuby(funcNode)
	if blockNode == nil {
		return
	}

	v.walkBlock(blockNode)
}

func (v *rubyDefUseVisitor) extractParameters(funcNode *sitter.Node) {
	paramsNode := findChildByTypeRuby(funcNode, "parameters", v.content)
	if paramsNode == nil {
		return
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "identifier" {
			name := nodeTextRuby(child, v.content)
			if name != "" && !isRubyBuiltin(name) {
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

func (v *rubyDefUseVisitor) walkBlock(blockNode *sitter.Node) {
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

func (v *rubyDefUseVisitor) processNode(node *sitter.Node) {
	switch node.Type() {
	case "local_variable_assignment", "instance_variable_assignment", "class_variable_assignment":
		v.processAssignment(node)
	case "for":
		v.processForStatement(node)
	case "while", "until":
		v.processLoopStatement(node)
	case "if", "elsif", "unless":
		v.processIfStatement(node)
	case "case":
		v.processCaseStatement(node)
	case "return", "break", "next":
		v.processJumpStatement(node)
	case "begin", "rescue", "ensure", "else":
		v.processExceptionHandling(node)
	case "body", "do_block", "compound_statement":
		v.walkBlock(node)
	default:
		v.extractUses(node)
	}
}

func (v *rubyDefUseVisitor) processAssignment(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "variable" || child.Type() == "identifier" {
			name := nodeTextRuby(child, v.content)
			if name != "" && !isRubyBuiltin(name) {
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

func (v *rubyDefUseVisitor) processForStatement(node *sitter.Node) {
	variable := findChildByTypeRuby(node, "variable", v.content)
	if variable != nil {
		name := nodeTextRuby(variable, v.content)
		if name != "" && !isRubyBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: RefTypeDefinition,
				Line:    int(variable.StartPoint().Row) + 1,
				Column:  int(variable.StartPoint().Column) + 1,
			}
			v.addRef(ref)
		}
	}

	iterator := findChildByTypeRuby(node, "iterator", v.content)
	if iterator != nil {
		v.extractUses(iterator)
	}

	body := findChildByTypeRuby(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}
}

func (v *rubyDefUseVisitor) processLoopStatement(node *sitter.Node) {
	condition := findChildByTypeRuby(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	body := findChildByTypeRuby(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}
}

func (v *rubyDefUseVisitor) processIfStatement(node *sitter.Node) {
	condition := findChildByTypeRuby(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	consequence := findChildByTypeRuby(node, "consequence", v.content)
	if consequence != nil {
		v.walkBlock(consequence)
	}

	alternative := findChildByTypeRuby(node, "alternative", v.content)
	if alternative != nil {
		v.walkBlock(alternative)
	}
}

func (v *rubyDefUseVisitor) processCaseStatement(node *sitter.Node) {
	condition := findChildByTypeRuby(node, "condition", v.content)
	if condition != nil {
		v.extractUses(condition)
	}

	whenClauses := findChildByTypeRuby(node, "when", v.content)
	if whenClauses != nil {
		v.extractUses(whenClauses)
	}

	body := findChildByTypeRuby(node, "body", v.content)
	if body != nil {
		v.walkBlock(body)
	}
}

func (v *rubyDefUseVisitor) processJumpStatement(node *sitter.Node) {
	v.extractUses(node)
}

func (v *rubyDefUseVisitor) processExceptionHandling(node *sitter.Node) {
	v.walkBlock(node)
}

func (v *rubyDefUseVisitor) extractUses(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeTextRuby(node, v.content)
		if name != "" && !isRubyBuiltin(name) {
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

func (v *rubyDefUseVisitor) isDefinitionAtPosition(node *sitter.Node) bool {
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

func (v *rubyDefUseVisitor) addRef(ref VarRef) {
	v.refs = append(v.refs, ref)
	v.variables[ref.Name] = append(v.variables[ref.Name], ref)
}

func findBlockRuby(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "body" || node.Type() == "do_block" || node.Type() == "compound_statement" {
		return node
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			result := findBlockRuby(child)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

func findChildByTypeRuby(node *sitter.Node, childType string, content []byte) *sitter.Node {
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

func nodeTextRuby(node *sitter.Node, content []byte) string {
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

func isRubyBuiltin(name string) bool {
	builtins := map[string]bool{
		"def": true, "class": true, "module": true, "end": true,
		"if": true, "elsif": true, "else": true, "unless": true,
		"case": true, "when": true, "while": true, "until": true,
		"for": true, "in": true, "do": true, "begin": true,
		"rescue": true, "ensure": true, "raise": true, "throw": true,
		"catch": true, "return": true, "break": true, "next": true,
		"redo": true, "retry": true, "self": true, "super": true,
		"true": true, "false": true, "nil": true, "and": true,
		"or": true, "not": true, "then": true, "yield": true,
		"proc": true, "lambda": true, "block_given": true,

		"puts": true, "print": true, "gets": true, "chomp": true,
		"each": true, "map": true, "select": true, "reject": true,
		"find": true, "reduce": true, "inject": true, "sort": true,
		"reverse": true, "length": true, "size": true, "empty": true,
		"nil?": true, "is_a?": true, "kind_of?": true,
		"to_s": true, "to_i": true, "to_f": true, "to_a": true,
		"to_h": true, "upcase": true, "downcase": true, "strip": true,
		"split": true, "join": true, "push": true, "pop": true,
		"shift": true, "unshift": true, "include?": true, "freeze": true,

		"Array": true, "Hash": true, "String": true, "Integer": true,
		"Float": true, "Symbol": true, "Range": true, "Regexp": true,
		"Time": true, "File": true, "Dir": true, "IO": true,
		"Thread": true, "Process": true, "ENV": true, "ARGV": true,
	}

	return builtins[name]
}
