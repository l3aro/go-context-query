package dfg

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

// scopeStack tracks variable definitions at each scope level.
// Each element is a set of variable names defined in that scope.
type scopeStack []map[string]bool

// isDefined checks if a variable is defined in any scope (local first, then outer).
func (s scopeStack) isDefined(name string) bool {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i][name] {
			return true
		}
	}
	return false
}

// addDefinition adds a variable definition to the current (innermost) scope.
func (s *scopeStack) addDefinition(name string) {
	if len(*s) > 0 {
		(*s)[len(*s)-1][name] = true
	}
}

// push creates a new scope level.
func (s *scopeStack) push() {
	*s = append(*s, make(map[string]bool))
}

// pop removes the current scope level.
func (s *scopeStack) pop() {
	if len(*s) > 0 {
		*s = (*s)[:len(*s)-1]
	}
}

type pythonDefUseVisitor struct {
	content    []byte
	funcName   string
	refs       []VarRef
	variables  map[string][]VarRef
	scopeStack scopeStack
	imports    []string
}

func newPythonDefUseVisitor(content []byte, funcName string) *pythonDefUseVisitor {
	return &pythonDefUseVisitor{
		content:    content,
		funcName:   funcName,
		refs:       make([]VarRef, 0),
		variables:  make(map[string][]VarRef),
		scopeStack: scopeStack{make(map[string]bool)},
		imports:    make([]string, 0),
	}
}

func extractPythonDFG(filePath string, functionName string) (*DFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		// Fall back to simple analysis if CFG extraction fails
		return extractPythonDFGSimple(content, functionName)
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
	visitor.extractImports(root)
	visitor.extractReferences(funcNode)

	analyzer := NewReachingDefsAnalyzer()
	edges := analyzer.ComputeDefUseChains(cfgInfo, visitor.refs)

	return &DFGInfo{
		FunctionName:  functionName,
		VarRefs:       visitor.refs,
		DataflowEdges: edges,
		Variables:     visitor.variables,
		Imports:       visitor.imports,
	}, nil
}

// extractPythonDFGSimple provides a fallback DFG analysis without CFG.
func extractPythonDFGSimple(content []byte, functionName string) (*DFGInfo, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())
	tree := parser.Parse(nil, content)
	defer tree.Close()

	root := tree.RootNode()
	funcNode := findPythonFunction(root, functionName, content)
	if funcNode == nil {
		return nil, fmt.Errorf("function %q not found", functionName)
	}

	visitor := newPythonDefUseVisitor(content, functionName)
	visitor.extractImports(root)
	visitor.extractReferences(funcNode)

	analyzer := NewSimpleFallbackAnalyzer()
	edges := analyzer.ComputeDefUseChains(visitor.refs)

	return &DFGInfo{
		FunctionName:  functionName,
		VarRefs:       visitor.refs,
		DataflowEdges: edges,
		Variables:     visitor.variables,
		Imports:       visitor.imports,
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

func (v *pythonDefUseVisitor) extractImports(root *sitter.Node) {
	if root == nil {
		return
	}

	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "import_statement":
			v.processImportStatement(child)
		case "import_from_statement":
			v.processImportFromStatement(child)
		}
	}
}

func (v *pythonDefUseVisitor) processImportStatement(node *sitter.Node) {
	if node == nil {
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "dotted_name":
			v.imports = append(v.imports, nodeText(child, v.content))
		case "aliased_import":
			alias := nodeText(child, v.content)
			if alias != "" {
				v.imports = append(v.imports, alias)
			}
		}
	}
}

func (v *pythonDefUseVisitor) processImportFromStatement(node *sitter.Node) {
	if node == nil {
		return
	}

	var module string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "dotted_name":
			module = nodeText(child, v.content)
		case "relative_import":
			module = nodeText(child, v.content)
		case "wildcard_import":
			v.imports = append(v.imports, module+".*")
		default:
			name := nodeText(child, v.content)
			if name != "" && name != "from" && name != "import" && name != "," {
				if module != "" {
					v.imports = append(v.imports, module+"."+name)
				} else {
					v.imports = append(v.imports, name)
				}
			}
		}
	}
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

func (v *pythonDefUseVisitor) walkStatement(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "expression_statement" {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil {
				v.processNode(child)
			}
		}
	} else {
		v.walkChildren(node)
	}
}

func (v *pythonDefUseVisitor) extractParameters(funcNode *sitter.Node) {
	paramsNode := findChildByType(funcNode, "parameters", v.content)
	if paramsNode == nil {
		return
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "identifier" {
			name := nodeText(child, v.content)
			if name != "" && !isBuiltin(name) {
				ref := VarRef{
					Name:    name,
					RefType: RefTypeDefinition,
					Line:    int(child.StartPoint().Row) + 1,
					Column:  int(child.StartPoint().Column) + 1,
				}
				v.addRef(ref)
				v.scopeStack.addDefinition(name)
			}
			continue
		}

		paramTypes := []string{
			"positional_or_keyword_parameter",
			"keyword_parameter",
			"optional_parameter",
			"variadic_parameter",
			"dictionary_variadic_parameter",
		}
		for _, paramType := range paramTypes {
			if child.Type() == paramType {
				identifier := findChildByType(child, "identifier", v.content)
				if identifier != nil {
					name := nodeText(identifier, v.content)
					if name != "" && !isBuiltin(name) {
						ref := VarRef{
							Name:    name,
							RefType: RefTypeDefinition,
							Line:    int(identifier.StartPoint().Row) + 1,
							Column:  int(identifier.StartPoint().Column) + 1,
						}
						v.addRef(ref)
						v.scopeStack.addDefinition(name)
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

		if child.Type() == "expression_statement" {
			for j := 0; j < int(child.ChildCount()); j++ {
				grandchild := child.Child(j)
				if grandchild != nil {
					v.processNode(grandchild)
				}
			}
		} else {
			v.processNode(child)
		}
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
		v.processNestedFunction(node)
	case "block":
		v.walkBlock(node)
	case "lambda":
		v.processLambda(node)
	case "list_comprehension", "set_comprehension", "generator_expression":
		v.processComprehension(node)
	case "dictionary_comprehension":
		v.processDictComprehension(node)
	case "attribute":
		// Handle attribute access (e.g., self.x)
		v.processAttributeAccess(node)
	case "identifier":
		// When we encounter an identifier not in an assignment context, it's a use
		v.extractUses(node)
	default:
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil {
				v.processNode(child)
			}
		}
	}
}

func (v *pythonDefUseVisitor) processNestedFunction(node *sitter.Node) {
	if node == nil {
		return
	}

	v.scopeStack.push()
	defer v.scopeStack.pop()

	// Extract parameters as definitions in the nested scope
	v.extractParameters(node)

	// Process function body to track any captured variables
	blockNode := findBlock(node)
	if blockNode != nil {
		v.walkBlock(blockNode)
	}
}

func (v *pythonDefUseVisitor) processAssignment(node *sitter.Node) {
	var left *sitter.Node

	if node.ChildCount() > 0 {
		firstChild := node.Child(0)
		if firstChild != nil && firstChild.Type() == "identifier" {
			left = firstChild
		}
	}

	if left == nil {
		left = findChildByType(node, "left", v.content)
	}

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
					v.scopeStack.addDefinition(name)
				}
			}
		}
	}
}

func (v *pythonDefUseVisitor) processLambda(node *sitter.Node) {
	if node == nil {
		return
	}

	v.scopeStack.push()
	defer v.scopeStack.pop()

	// Process children to extract parameters and body
	// Lambda structure: 'lambda' [parameters] ':' body
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		childType := child.Type()
		switch childType {
		case "lambda":
			// Skip the lambda keyword
			continue
		case ":":
			// Skip the colon separator
			continue
		case "lambda_parameters":
			// Extract lambda parameters as definitions
			v.extractLambdaParameters(child)
		default:
			// Everything else is the body expression
			v.processNode(child)
		}
	}
}

func (v *pythonDefUseVisitor) extractLambdaParameters(paramsNode *sitter.Node) {
	if paramsNode == nil {
		return
	}

	// Recursively find all identifiers in the parameters and mark as definitions
	v.extractLambdaParamIdentifiers(paramsNode)
}

func (v *pythonDefUseVisitor) extractLambdaParamIdentifiers(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "identifier" {
		name := nodeText(node, v.content)
		if name != "" && !isBuiltin(name) {
			ref := VarRef{
				Name:    name,
				RefType: RefTypeDefinition,
				Line:    int(node.StartPoint().Row) + 1,
				Column:  int(node.StartPoint().Column) + 1,
			}
			v.addRef(ref)
			v.scopeStack.addDefinition(name)
		}
		return
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			v.extractLambdaParamIdentifiers(child)
		}
	}
}

func (v *pythonDefUseVisitor) processComprehension(node *sitter.Node) {
	if node == nil {
		return
	}

	v.scopeStack.push()
	defer v.scopeStack.pop()

	// Process generators (for clauses) - targets are definitions in inner scope
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "for_in_clause":
			// Extract uses from iterable first
			iterable := child.ChildByFieldName("right")
			if iterable != nil {
				v.extractUses(iterable)
			}
			// Extract definitions from target
			target := child.ChildByFieldName("left")
			if target != nil {
				v.extractAssignmentTarget(target, RefTypeDefinition)
			}
			// Process conditions
			for j := 0; j < int(child.ChildCount()); j++ {
				condChild := child.Child(j)
				if condChild != nil && condChild.Type() == "if_clause" {
					v.extractUses(condChild)
				}
			}
		case "if_clause":
			v.extractUses(child)
		default:
			if child.Type() != "[" && child.Type() != "]" &&
				child.Type() != "{" && child.Type() != "}" &&
				child.Type() != "(" && child.Type() != ")" {
				v.processNode(child)
			}
		}
	}
}

func (v *pythonDefUseVisitor) processDictComprehension(node *sitter.Node) {
	if node == nil {
		return
	}

	v.scopeStack.push()
	defer v.scopeStack.pop()

	// Process generators (for clauses) - targets are definitions in inner scope
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "for_in_clause":
			// Extract uses from iterable first
			iterable := child.ChildByFieldName("right")
			if iterable != nil {
				v.extractUses(iterable)
			}
			// Extract definitions from target
			target := child.ChildByFieldName("left")
			if target != nil {
				v.extractAssignmentTarget(target, RefTypeDefinition)
			}
			// Process conditions
			for j := 0; j < int(child.ChildCount()); j++ {
				condChild := child.Child(j)
				if condChild != nil && condChild.Type() == "if_clause" {
					v.extractUses(condChild)
				}
			}
		case "if_clause":
			v.extractUses(child)
		default:
			if child.Type() != "[" && child.Type() != "]" &&
				child.Type() != "{" && child.Type() != "}" &&
				child.Type() != "(" && child.Type() != ")" {
				v.processNode(child)
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
			// Track definition in current scope
			if refType == RefTypeDefinition {
				v.scopeStack.addDefinition(name)
			}
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

// processAttributeAccess handles attribute access like self.x or obj.attribute
func (v *pythonDefUseVisitor) processAttributeAccess(node *sitter.Node) {
	if node == nil || node.Type() != "attribute" {
		return
	}

	// Extract the full attribute reference (e.g., "self.x")
	attrName := nodeText(node, v.content)

	// Get line number from the attribute node
	line := int(node.StartPoint().Row) + 1
	col := int(node.StartPoint().Column)

	// Add as a use of the attribute
	v.addRef(VarRef{
		Name:    attrName,
		Line:    line,
		Column:  col,
		RefType: RefTypeUse,
	})

	// Continue walking children to find nested uses
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			v.processNode(child)
		}
	}
}

func findBlock(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "block" || node.Type() == "body" {
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
