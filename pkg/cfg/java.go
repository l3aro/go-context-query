package cfg

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
)

type javaCFGExtractor struct {
	content  []byte
	tree     *sitter.Tree
	blocks   map[string]*CFGBlock
	edges    []CFGEdge
	blockID  int
	funcName string
}

func newJavaCFGExtractor(content []byte, funcName string) *javaCFGExtractor {
	parser := sitter.NewParser()
	parser.SetLanguage(java.GetLanguage())
	tree := parser.Parse(nil, content)

	return &javaCFGExtractor{
		content:  content,
		tree:     tree,
		blocks:   make(map[string]*CFGBlock),
		edges:    make([]CFGEdge, 0),
		blockID:  0,
		funcName: funcName,
	}
}

func ExtractJavaCFG(filePath string, functionName string) (*CFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	extractor := newJavaCFGExtractor(content, functionName)
	defer extractor.tree.Close()

	root := extractor.tree.RootNode()
	funcNode := extractor.findMethod(root, functionName)
	if funcNode == nil {
		return nil, fmt.Errorf("method %q not found in %s", functionName, filePath)
	}

	blockNode := extractor.findBlock(funcNode)
	if blockNode == nil {
		return nil, fmt.Errorf("method body not found for %s", functionName)
	}

	entryBlock := extractor.newBlock(BlockTypeEntry, int(funcNode.StartPoint().Row)+1)
	entryBlock.Statements = []string{"entry"}
	extractor.addBlock(entryBlock)

	currentBlock := entryBlock
	extractor.processBlock(blockNode, &currentBlock)

	exitBlock := extractor.newBlock(BlockTypeExit, int(funcNode.EndPoint().Row)+1)
	exitBlock.Statements = []string{"exit"}
	extractor.addBlock(exitBlock)

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

func (e *javaCFGExtractor) findMethod(node *sitter.Node, funcName string) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "method_declaration" {
		funcNameNode := e.findChildByType(node, "identifier")
		if funcNameNode != nil {
			name := e.nodeText(funcNameNode)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "constructor_declaration" {
		funcNameNode := e.findChildByType(node, "identifier")
		if funcNameNode != nil {
			name := e.nodeText(funcNameNode)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "class_declaration" {
		classBody := e.findChildByType(node, "class_body")
		if classBody != nil {
			for i := 0; i < int(classBody.ChildCount()); i++ {
				child := classBody.Child(i)
				if child != nil {
					result := e.findMethod(child, funcName)
					if result != nil {
						return result
					}
				}
			}
		}
	}

	if node.Type() == "interface_declaration" {
		interfaceBody := e.findChildByType(node, "interface_body")
		if interfaceBody != nil {
			for i := 0; i < int(interfaceBody.ChildCount()); i++ {
				child := interfaceBody.Child(i)
				if child != nil {
					result := e.findMethod(child, funcName)
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
		result := e.findMethod(child, funcName)
		if result != nil {
			return result
		}
	}

	return nil
}

func (e *javaCFGExtractor) findBlock(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "block" {
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

func (e *javaCFGExtractor) processBlock(blockNode *sitter.Node, currentBlock **CFGBlock) {
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

		case "enhanced_for_statement":
			e.processEnhancedForStatement(child, currentBlock)

		case "while_statement":
			e.processWhileStatement(child, currentBlock)

		case "do_statement":
			e.processDoStatement(child, currentBlock)

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

		case "synchronized_statement":
			e.processSynchronizedStatement(child, currentBlock)

		case "labeled_statement":
			e.processLabeledStatement(child, currentBlock)

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

func (e *javaCFGExtractor) processIfStatement(node *sitter.Node, currentBlock **CFGBlock) {
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
		e.processBlock(alternative, &elseBlock)
		beforeElseBlock = elseBlock
	} else {
		*currentBlock = branchBlock
		return
	}

	*currentBlock = beforeElseBlock
}

func (e *javaCFGExtractor) processSwitchStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition string
	var cases []*sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "condition":
			condition = e.nodeText(child)
		case "switch_block":
			e.extractSwitchCases(child, &cases)
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

	for _, sc := range cases {
		caseBlock := e.newBlock(BlockTypeBranch, int(sc.StartPoint().Row)+1)
		caseBlock.Statements = []string{e.nodeText(sc)}
		e.addBlock(caseBlock)

		e.addEdge(switchBlock.ID, caseBlock.ID, EdgeTypeUnconditional)

		caseBody := e.findChildByType(sc, "block")
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

func (e *javaCFGExtractor) extractSwitchCases(node *sitter.Node, cases *[]*sitter.Node) {
	if node == nil {
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "switch_case" || child.Type() == "switch_label" {
			*cases = append(*cases, child)
		} else {
			e.extractSwitchCases(child, cases)
		}
	}
}

func (e *javaCFGExtractor) processForStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var init string
	var condition string
	var update string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "init":
			init = e.nodeText(child)
		case "condition":
			condition = e.nodeText(child)
		case "update":
			update = e.nodeText(child)
		case "body":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"for (" + init + "; " + condition + "; " + update + ")"}
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

func (e *javaCFGExtractor) processEnhancedForStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var iterator string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "enhanced_for":
			iterator = e.nodeText(child)
		case "body":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"for (" + iterator + ")"}
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

func (e *javaCFGExtractor) processWhileStatement(node *sitter.Node, currentBlock **CFGBlock) {
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

func (e *javaCFGExtractor) processDoStatement(node *sitter.Node, currentBlock **CFGBlock) {
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

	loopBody := e.newBlock(BlockTypeLoopBody, int(node.StartPoint().Row)+1)
	loopBody.Statements = []string{"do {"}
	e.addBlock(loopBody)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, loopBody.ID, EdgeTypeUnconditional)
	}

	if body != nil {
		e.processBlock(body, &loopBody)
	}

	loopCondition := e.newBlock(BlockTypeBranch, int(node.EndPoint().Row)+1)
	loopCondition.Statements = []string{"while (" + condition + ")"}
	e.addBlock(loopCondition)

	e.addEdge(loopBody.ID, loopCondition.ID, EdgeTypeTrue)
	e.addEdge(loopCondition.ID, loopBody.ID, EdgeTypeBackEdge)

	*currentBlock = loopCondition
}

func (e *javaCFGExtractor) processReturnStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	var returnValue string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() != "return" {
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

func (e *javaCFGExtractor) processBreakStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	breakStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, breakStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeBreak)
}

func (e *javaCFGExtractor) processContinueStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	continueStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, continueStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeContinue)
}

func (e *javaCFGExtractor) processTryStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var tryBlock *sitter.Node
	var catchClauses []*sitter.Node
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
			catchClauses = append(catchClauses, child)
		case "finally_clause":
			finallyBlock = child
		}
	}

	tryStartBlock := e.newBlock(BlockTypePlain, int(node.StartPoint().Row)+1)
	tryStartBlock.Statements = []string{"try {"}
	e.addBlock(tryStartBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, tryStartBlock.ID, EdgeTypeUnconditional)
	}

	var lastBlock *CFGBlock
	lastBlock = tryStartBlock

	if tryBlock != nil {
		e.processBlock(tryBlock, &lastBlock)
	}

	for _, catchClause := range catchClauses {
		catchBlock := e.newBlock(BlockTypeBranch, int(catchClause.StartPoint().Row)+1)
		catchBlock.Statements = []string{e.nodeText(catchClause)}
		e.addBlock(catchBlock)

		e.addEdge(lastBlock.ID, catchBlock.ID, EdgeTypeTrue)

		catchBody := e.findChildByType(catchClause, "block")
		if catchBody != nil {
			e.processBlock(catchBody, &catchBlock)
			lastBlock = catchBlock
		}
	}

	if finallyBlock != nil {
		finallyBody := e.findChildByType(finallyBlock, "block")
		if finallyBody != nil {
			finalBlock := e.newBlock(BlockTypePlain, int(finallyBlock.StartPoint().Row)+1)
			finalBlock.Statements = []string{"finally {"}
			e.addBlock(finalBlock)

			e.addEdge(lastBlock.ID, finalBlock.ID, EdgeTypeUnconditional)
			e.processBlock(finallyBody, &finalBlock)
			lastBlock = finalBlock
		}
	}

	*currentBlock = lastBlock
}

func (e *javaCFGExtractor) processThrowStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	throwStmt := e.nodeText(node)
	throwBlock := e.newBlock(BlockTypeReturn, int(node.StartPoint().Row)+1)
	throwBlock.Statements = []string{throwStmt}
	e.addBlock(throwBlock)

	e.addEdge((*currentBlock).ID, throwBlock.ID, EdgeTypeUnconditional)

	*currentBlock = throwBlock
}

func (e *javaCFGExtractor) processSynchronizedStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var lock string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "argument_list":
			lock = e.nodeText(child)
		case "block":
			body = child
		}
	}

	syncBlock := e.newBlock(BlockTypePlain, int(node.StartPoint().Row)+1)
	syncBlock.Statements = []string{"synchronized (" + lock + ")"}
	e.addBlock(syncBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, syncBlock.ID, EdgeTypeUnconditional)
	}

	if body != nil {
		e.processBlock(body, &syncBlock)
	}

	*currentBlock = syncBlock
}

func (e *javaCFGExtractor) processLabeledStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var label string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "identifier":
			label = e.nodeText(child)
		default:
			if body == nil {
				body = child
			}
		}
	}

	labelBlock := e.newBlock(BlockTypePlain, int(node.StartPoint().Row)+1)
	labelBlock.Statements = []string{label + ":"}
	e.addBlock(labelBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, labelBlock.ID, EdgeTypeUnconditional)
	}

	if body != nil {
		e.processBlock(body, &labelBlock)
	}

	*currentBlock = labelBlock
}

func (e *javaCFGExtractor) newBlock(blockType BlockType, line int) *CFGBlock {
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

func (e *javaCFGExtractor) addBlock(block *CFGBlock) {
	e.blocks[block.ID] = block
}

func (e *javaCFGExtractor) addEdge(sourceID, targetID string, edgeType EdgeType) {
	edge := CFGEdge{
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: edgeType,
	}
	e.edges = append(e.edges, edge)
}

func (e *javaCFGExtractor) blocksToMap() map[string]CFGBlock {
	result := make(map[string]CFGBlock)
	for id, block := range e.blocks {
		result[id] = *block
	}
	return result
}

func (e *javaCFGExtractor) calculateCyclomaticComplexity(node *sitter.Node) int {
	if node == nil {
		return 1
	}

	decisionPoints := e.countDecisionPoints(node)
	return decisionPoints + 1
}

func (e *javaCFGExtractor) countDecisionPoints(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0

	switch node.Type() {
	case "if_statement":
		count++
		alt := e.findChildByType(node, "alternative")
		if alt != nil {
			count++
		}

	case "for_statement":
		count++

	case "enhanced_for_statement":
		count++

	case "while_statement":
		count++

	case "do_statement":
		count++

	case "switch_statement":
		count += e.countSwitchCases(node)

	case "catch_clause":
		count++

	case "&&", "||":
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

func (e *javaCFGExtractor) countSwitchCases(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && (child.Type() == "switch_case" || child.Type() == "switch_label") {
			count++
		}
	}
	return count
}

func (e *javaCFGExtractor) findChildByType(node *sitter.Node, childType string) *sitter.Node {
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

func (e *javaCFGExtractor) nodeText(node *sitter.Node) string {
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
