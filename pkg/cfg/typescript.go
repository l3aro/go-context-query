package cfg

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

type tsCFGExtractor struct {
	content  []byte
	tree     *sitter.Tree
	blocks   map[string]*CFGBlock
	edges    []CFGEdge
	blockID  int
	funcName string
}

func newTSCFGExtractor(content []byte, funcName string) *tsCFGExtractor {
	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())
	tree := parser.Parse(nil, content)

	return &tsCFGExtractor{
		content:  content,
		tree:     tree,
		blocks:   make(map[string]*CFGBlock),
		edges:    make([]CFGEdge, 0),
		blockID:  0,
		funcName: funcName,
	}
}

// ExtractTSCFG extracts the Control Flow Graph from a TypeScript/JavaScript file
// for the specified function name.
func ExtractTSCFG(filePath string, functionName string) (*CFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	extractor := newTSCFGExtractor(content, functionName)
	defer extractor.tree.Close()

	root := extractor.tree.RootNode()
	funcNode := extractor.findFunction(root, functionName)
	if funcNode == nil {
		return nil, fmt.Errorf("function %q not found in %s", functionName, filePath)
	}

	blockNode := extractor.findBlock(funcNode)
	if blockNode == nil {
		return nil, fmt.Errorf("function body not found for %s", functionName)
	}

	entryBlock := extractor.newBlock(BlockTypeEntry, int(funcNode.StartPoint().Row)+1)
	entryBlock.Statements = []string{"entry"}
	extractor.addBlock(entryBlock)

	currentBlock := entryBlock
	extractor.processBlock(blockNode, &currentBlock)

	exitBlock := extractor.newBlock(BlockTypeExit, int(funcNode.EndPoint().Row)+1)
	exitBlock.Statements = []string{"exit"}
	extractor.addBlock(exitBlock)

	// Connect the last block to exit if not already connected
	if currentBlock != nil && currentBlock.ID != exitBlock.ID {
		extractor.addEdge(currentBlock.ID, exitBlock.ID, EdgeTypeUnconditional)
	}

	complexity := extractor.calculateCyclomaticComplexity(blockNode)

	return &CFGInfo{
		FunctionName:         functionName,
		Blocks:               extractor.blocksToMap(),
		Edges:                extractor.edges,
		EntryBlockID:         entryBlock.ID,
		ExitBlockIDs:         []string{exitBlock.ID},
		CyclomaticComplexity: complexity,
	}, nil
}

func (e *tsCFGExtractor) findFunction(node *sitter.Node, funcName string) *sitter.Node {
	if node == nil {
		return nil
	}

	// Check for function declaration: function foo() {}
	if node.Type() == "function_declaration" {
		funcNameNode := e.findChildByType(node, "identifier")
		if funcNameNode != nil {
			name := e.nodeText(funcNameNode)
			if name == funcName {
				return node
			}
		}
	}

	// Check for method definition in class: foo() {}
	if node.Type() == "method_definition" {
		propName := e.findChildByType(node, "property_name")
		if propName != nil {
			name := e.nodeText(propName)
			if name == funcName {
				return node
			}
		}
	}

	// Check for arrow function: const foo = () => {}
	if node.Type() == "variable_declarator" {
		nameNode := e.findChildByType(node, "variable_name")
		if nameNode != nil {
			name := e.nodeText(nameNode)
			if name == funcName {
				// Check if it's an arrow function
				init := e.findChildByType(node, "arrow_function")
				if init != nil {
					return init
				}
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		result := e.findFunction(child, funcName)
		if result != nil {
			return result
		}
	}

	return nil
}

func (e *tsCFGExtractor) findBlock(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	// Check for statement_block (TypeScript uses statement_block, not just block)
	if node.Type() == "statement_block" || node.Type() == "block" {
		return node
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			result := e.findBlock(child)
			if result != nil {
				return result
			}
		}
	}

	return nil
}

func (e *tsCFGExtractor) processBlock(blockNode *sitter.Node, currentBlock **CFGBlock) {
	if blockNode == nil {
		return
	}

	for i := 0; i < int(blockNode.ChildCount()); i++ {
		child := blockNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "if_statement":
			e.processIfStatement(child, currentBlock)

		case "switch_statement":
			e.processSwitchStatement(child, currentBlock)

		case "for_statement":
			e.processForStatement(child, currentBlock)

		case "for_in_statement":
			e.processForInStatement(child, currentBlock)

		case "for_of_statement":
			e.processForOfStatement(child, currentBlock)

		case "while_statement":
			e.processWhileStatement(child, currentBlock)

		case "do_statement":
			e.processDoWhileStatement(child, currentBlock)

		case "return_statement":
			e.processReturnStatement(child, currentBlock)

		case "break_statement":
			e.processBreakStatement(child, currentBlock)

		case "continue_statement":
			e.processContinueStatement(child, currentBlock)

		case "try_statement":
			e.processTryStatement(child, currentBlock)

		case "throw_statement":
			e.processThrowStatement(child, currentBlock)

		case "labeled_statement":
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		default:
			stmt := e.nodeText(child)
			stmt = strings.TrimSpace(stmt)
			if stmt != "" && !strings.HasPrefix(stmt, "//") && !strings.HasPrefix(stmt, "/*") && !strings.HasPrefix(stmt, "*") {
				if *currentBlock != nil {
					(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
					(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
				}
			}
		}
	}
}

func (e *tsCFGExtractor) processIfStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition string
	var consequent *sitter.Node
	var alternative *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "condition":
			condition = e.nodeText(child)
		case "consequence":
			consequent = child
		case "alternative":
			alternative = child
		}
	}

	branchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	branchBlock.Statements = []string{"if (" + condition + ")"}
	e.addBlock(branchBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, branchBlock.ID, EdgeTypeUnconditional)
	}

	consequentBlock := e.newBlock(BlockTypePlain, int(node.StartPoint().Row)+1)
	e.addBlock(consequentBlock)
	e.addEdge(branchBlock.ID, consequentBlock.ID, EdgeTypeTrue)

	var beforeElseBlock *CFGBlock
	beforeElseBlock = consequentBlock

	if consequent != nil {
		e.processBlock(consequent, &consequentBlock)
		beforeElseBlock = consequentBlock
	}

	if alternative != nil {
		elseBlock := e.newBlock(BlockTypePlain, int(alternative.StartPoint().Row)+1)
		e.addBlock(elseBlock)
		e.addEdge(branchBlock.ID, elseBlock.ID, EdgeTypeFalse)

		// Check if it's an else-if chain
		elseIfNode := e.findChildByType(alternative, "if_statement")
		if elseIfNode != nil {
			e.processIfStatement(elseIfNode, &elseBlock)
			beforeElseBlock = elseBlock
		} else {
			e.processBlock(alternative, &elseBlock)
			beforeElseBlock = elseBlock
		}
	} else {
		*currentBlock = branchBlock
		return
	}

	*currentBlock = beforeElseBlock
}

func (e *tsCFGExtractor) processSwitchStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition string
	var caseList []*sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "switch_expression":
			condition = e.nodeText(child)
		case "case_clause":
			caseList = append(caseList, child)
		}
	}

	switchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	switchBlock.Statements = []string{"switch (" + condition + ")"}
	e.addBlock(switchBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, switchBlock.ID, EdgeTypeUnconditional)
	}

	var lastBlock *CFGBlock
	lastBlock = switchBlock

	for _, sc := range caseList {
		caseBlock := e.newBlock(BlockTypeBranch, int(sc.StartPoint().Row)+1)
		caseBlock.Statements = []string{e.nodeText(sc)}
		e.addBlock(caseBlock)

		e.addEdge(switchBlock.ID, caseBlock.ID, EdgeTypeUnconditional)

		caseBody := e.findChildByType(sc, "statement_block")
		if caseBody != nil {
			caseBodyBlock := e.newBlock(BlockTypePlain, int(caseBody.StartPoint().Row)+1)
			e.addBlock(caseBodyBlock)
			e.addEdge(caseBlock.ID, caseBodyBlock.ID, EdgeTypeUnconditional)

			e.processBlock(caseBody, &caseBodyBlock)
			lastBlock = caseBodyBlock
		} else {
			lastBlock = caseBlock
		}
	}

	*currentBlock = lastBlock
}

func (e *tsCFGExtractor) processForStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var initializer string
	var condition string
	var increment string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "for_initializer":
			initializer = e.nodeText(child)
		case "for_condition":
			condition = e.nodeText(child)
		case "for_increment":
			increment = e.nodeText(child)
		case "body":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"for (" + initializer + "; " + condition + "; " + increment + ")"}
	e.addBlock(loopHeader)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, loopHeader.ID, EdgeTypeUnconditional)
	}

	loopBody := e.newBlock(BlockTypeLoopBody, int(node.StartPoint().Row)+1)
	e.addBlock(loopBody)
	e.addEdge(loopHeader.ID, loopBody.ID, EdgeTypeTrue)

	if body != nil {
		e.processBlock(body, &loopBody)
	}

	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	*currentBlock = loopHeader
}

func (e *tsCFGExtractor) processForInStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var left string
	var right string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "for_in_initializer", "for_in_left":
			left = e.nodeText(child)
		case "for_in_source":
			right = e.nodeText(child)
		case "body":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"for (" + left + " in " + right + ")"}
	e.addBlock(loopHeader)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, loopHeader.ID, EdgeTypeUnconditional)
	}

	loopBody := e.newBlock(BlockTypeLoopBody, int(node.StartPoint().Row)+1)
	e.addBlock(loopBody)
	e.addEdge(loopHeader.ID, loopBody.ID, EdgeTypeTrue)

	if body != nil {
		e.processBlock(body, &loopBody)
	}

	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	*currentBlock = loopHeader
}

func (e *tsCFGExtractor) processForOfStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var left string
	var right string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "for_of_initializer", "for_of_left":
			left = e.nodeText(child)
		case "for_of_iterator":
			right = e.nodeText(child)
		case "body":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"for (" + left + " of " + right + ")"}
	e.addBlock(loopHeader)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, loopHeader.ID, EdgeTypeUnconditional)
	}

	loopBody := e.newBlock(BlockTypeLoopBody, int(node.StartPoint().Row)+1)
	e.addBlock(loopBody)
	e.addEdge(loopHeader.ID, loopBody.ID, EdgeTypeTrue)

	if body != nil {
		e.processBlock(body, &loopBody)
	}

	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	*currentBlock = loopHeader
}

func (e *tsCFGExtractor) processWhileStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "condition":
			condition = e.nodeText(child)
		case "body":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"while (" + condition + ")"}
	e.addBlock(loopHeader)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, loopHeader.ID, EdgeTypeUnconditional)
	}

	loopBody := e.newBlock(BlockTypeLoopBody, int(node.StartPoint().Row)+1)
	e.addBlock(loopBody)
	e.addEdge(loopHeader.ID, loopBody.ID, EdgeTypeTrue)

	if body != nil {
		e.processBlock(body, &loopBody)
	}

	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	*currentBlock = loopHeader
}

func (e *tsCFGExtractor) processDoWhileStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "condition":
			condition = e.nodeText(child)
		case "body":
			body = child
		}
	}

	// For do-while, we create a body block first, then the condition check
	bodyBlock := e.newBlock(BlockTypeLoopBody, int(node.StartPoint().Row)+1)
	e.addBlock(bodyBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, bodyBlock.ID, EdgeTypeUnconditional)
	}

	if body != nil {
		e.processBlock(body, &bodyBlock)
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.EndPoint().Row)+1)
	loopHeader.Statements = []string{"while (" + condition + ")"}
	e.addBlock(loopHeader)

	e.addEdge(bodyBlock.ID, loopHeader.ID, EdgeTypeUnconditional)

	*currentBlock = loopHeader
}

func (e *tsCFGExtractor) processReturnStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	var returnValue string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() != "return" && child.Type() != "expression_list" {
			returnValue = e.nodeText(child)
			break
		}
		if child != nil && child.Type() == "expression_list" {
			returnValue = e.nodeText(child)
			break
		}
	}

	returnBlock := e.newBlock(BlockTypeReturn, int(node.StartPoint().Row)+1)
	if returnValue != "" {
		returnBlock.Statements = []string{"return " + returnValue}
	} else {
		returnBlock.Statements = []string{"return"}
	}
	e.addBlock(returnBlock)

	e.addEdge((*currentBlock).ID, returnBlock.ID, EdgeTypeUnconditional)

	*currentBlock = returnBlock
}

func (e *tsCFGExtractor) processBreakStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	breakStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, breakStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeBreak)
}

func (e *tsCFGExtractor) processContinueStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	continueStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, continueStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeContinue)
}

func (e *tsCFGExtractor) processTryStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var tryBlock *sitter.Node
	var catchBlock *sitter.Node
	var finallyBlock *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "block":
			if tryBlock == nil {
				tryBlock = child
			}
		case "catch_clause":
			catchBlock = child
		case "finally":
			finallyBlock = child
		}
	}

	// Create try block
	if tryBlock != nil {
		tryBodyBlock := e.newBlock(BlockTypePlain, int(tryBlock.StartPoint().Row)+1)
		e.addBlock(tryBodyBlock)

		if *currentBlock != nil {
			e.addEdge((*currentBlock).ID, tryBodyBlock.ID, EdgeTypeUnconditional)
		}

		e.processBlock(tryBlock, &tryBodyBlock)
		*currentBlock = tryBodyBlock
	}

	// Create catch block
	if catchBlock != nil {
		// Find the catch body (block inside catch)
		var catchBody *sitter.Node
		for i := 0; i < int(catchBlock.ChildCount()); i++ {
			child := catchBlock.Child(i)
			if child != nil && child.Type() == "block" {
				catchBody = child
				break
			}
		}

		if *currentBlock != nil {
			e.addEdge((*currentBlock).ID, "", EdgeTypeFalse)
		}

		catchBodyBlock := e.newBlock(BlockTypeBranch, int(catchBlock.StartPoint().Row)+1)
		catchBodyBlock.Statements = []string{"catch"}
		e.addBlock(catchBodyBlock)

		if *currentBlock != nil {
			e.addEdge((*currentBlock).ID, catchBodyBlock.ID, EdgeTypeUnconditional)
		}

		if catchBody != nil {
			e.processBlock(catchBody, &catchBodyBlock)
		}
		*currentBlock = catchBodyBlock
	}

	// Create finally block
	if finallyBlock != nil {
		var finallyBody *sitter.Node
		for i := 0; i < int(finallyBlock.ChildCount()); i++ {
			child := finallyBlock.Child(i)
			if child != nil && child.Type() == "block" {
				finallyBody = child
				break
			}
		}

		if *currentBlock != nil {
			e.addEdge((*currentBlock).ID, "", EdgeTypeUnconditional)
		}

		finallyBodyBlock := e.newBlock(BlockTypePlain, int(finallyBlock.StartPoint().Row)+1)
		finallyBodyBlock.Statements = []string{"finally"}
		e.addBlock(finallyBodyBlock)

		if *currentBlock != nil {
			e.addEdge((*currentBlock).ID, finallyBodyBlock.ID, EdgeTypeUnconditional)
		}

		if finallyBody != nil {
			e.processBlock(finallyBody, &finallyBodyBlock)
		}
		*currentBlock = finallyBodyBlock
	}
}

func (e *tsCFGExtractor) processThrowStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	throwValue := ""
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() != "throw" {
			throwValue = e.nodeText(child)
			break
		}
	}

	throwBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	if throwValue != "" {
		throwBlock.Statements = []string{"throw " + throwValue}
	} else {
		throwBlock.Statements = []string{"throw"}
	}
	e.addBlock(throwBlock)

	e.addEdge((*currentBlock).ID, throwBlock.ID, EdgeTypeUnconditional)

	*currentBlock = throwBlock
}

func (e *tsCFGExtractor) newBlock(blockType BlockType, line int) *CFGBlock {
	e.blockID++
	block := &CFGBlock{
		ID:           fmt.Sprintf("block_%d", e.blockID),
		Type:         blockType,
		StartLine:    line,
		EndLine:      line,
		Statements:   make([]string, 0),
		Predecessors: make([]string, 0),
	}
	return block
}

func (e *tsCFGExtractor) addBlock(block *CFGBlock) {
	e.blocks[block.ID] = block
}

func (e *tsCFGExtractor) addEdge(sourceID, targetID string, edgeType EdgeType) {
	edge := CFGEdge{
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: edgeType,
	}
	e.edges = append(e.edges, edge)
}

func (e *tsCFGExtractor) blocksToMap() map[string]CFGBlock {
	result := make(map[string]CFGBlock)
	for id, block := range e.blocks {
		result[id] = *block
	}
	return result
}

func (e *tsCFGExtractor) calculateCyclomaticComplexity(node *sitter.Node) int {
	if node == nil {
		return 1
	}

	decisionPoints := e.countDecisionPoints(node)
	return decisionPoints + 1
}

func (e *tsCFGExtractor) countDecisionPoints(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0

	switch node.Type() {
	case "if_statement":
		count++
		alt := e.findChildByType(node, "alternative")
		if alt != nil {
			count += e.countElseIfBranches(alt)
		}

	case "for_statement":
		count++

	case "for_in_statement":
		count++

	case "for_of_statement":
		count++

	case "while_statement":
		count++

	case "do_statement":
		count++

	case "switch_statement":
		count += e.countSwitchCases(node)

	case "case_clause":
		count++

	case "&&", "||":
		count++

	case "ternary_expression":
		count++
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			count += e.countDecisionPoints(child)
		}
	}

	return count
}

func (e *tsCFGExtractor) countElseIfBranches(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "if_statement" {
			count++
			alt := e.findChildByType(child, "alternative")
			if alt != nil {
				count += e.countElseIfBranches(alt)
			}
		}
	}
	return count
}

func (e *tsCFGExtractor) countSwitchCases(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "case_clause" {
			count++
		}
	}
	return count
}

func (e *tsCFGExtractor) findChildByType(node *sitter.Node, childType string) *sitter.Node {
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

func (e *tsCFGExtractor) nodeText(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	start := node.StartByte()
	end := node.EndByte()
	if start >= uint32(len(e.content)) || end > uint32(len(e.content)) {
		return ""
	}
	return string(e.content[start:end])
}
