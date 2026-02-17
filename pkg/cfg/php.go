package cfg

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"
)

type phpCFGExtractor struct {
	content  []byte
	tree     *sitter.Tree
	blocks   map[string]*CFGBlock
	edges    []CFGEdge
	blockID  int
	funcName string
}

func newPhpCFGExtractor(content []byte, funcName string) *phpCFGExtractor {
	parser := sitter.NewParser()
	parser.SetLanguage(php.GetLanguage())
	tree := parser.Parse(nil, content)

	return &phpCFGExtractor{
		content:  content,
		tree:     tree,
		blocks:   make(map[string]*CFGBlock),
		edges:    make([]CFGEdge, 0),
		blockID:  0,
		funcName: funcName,
	}
}

func ExtractPhpCFG(filePath string, functionName string) (*CFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	extractor := newPhpCFGExtractor(content, functionName)
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

func (e *phpCFGExtractor) findFunction(node *sitter.Node, funcName string) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_definition" {
		funcNameNode := e.findChildByType(node, "name")
		if funcNameNode != nil {
			name := e.nodeText(funcNameNode)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "class_declaration" {
		classBody := e.findChildByType(node, "body")
		if classBody != nil {
			for i := 0; i < int(classBody.ChildCount()); i++ {
				child := classBody.Child(i)
				if child != nil {
					result := e.findFunction(child, funcName)
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
		result := e.findFunction(child, funcName)
		if result != nil {
			return result
		}
	}

	return nil
}

func (e *phpCFGExtractor) findBlock(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "compound_statement" || node.Type() == "body" {
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

func (e *phpCFGExtractor) processBlock(blockNode *sitter.Node, currentBlock **CFGBlock) {
	if blockNode == nil {
		return
	}

	for i := 0; i < int(blockNode.ChildCount()); i++ {
		child := blockNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "if":
			e.processIfStatement(child, currentBlock)

		case "switch":
			e.processSwitchStatement(child, currentBlock)

		case "for":
			e.processForStatement(child, currentBlock)

		case "foreach":
			e.processForeachStatement(child, currentBlock)

		case "while":
			e.processWhileStatement(child, currentBlock)

		case "do":
			e.processDoWhileStatement(child, currentBlock)

		case "return":
			e.processReturnStatement(child, currentBlock)

		case "break":
			e.processBreakStatement(child, currentBlock)

		case "continue":
			e.processContinueStatement(child, currentBlock)

		case "goto":
			e.processGotoStatement(child, currentBlock)

		case "try":
			e.processTryStatement(child, currentBlock)

		case "throw":
			e.processThrowStatement(child, currentBlock)

		case "case":
			e.processCaseStatement(child, currentBlock)

		default:
			stmt := e.nodeText(child)
			stmt = strings.TrimSpace(stmt)
			if stmt != "" && !strings.HasPrefix(stmt, "//") && !strings.HasPrefix(stmt, "/*") && !strings.HasPrefix(stmt, "#") {
				if *currentBlock != nil {
					(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
					(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
				}
			}
		}
	}
}

func (e *phpCFGExtractor) processIfStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition *sitter.Node
	var consequence *sitter.Node
	var alternative *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "condition":
			condition = child
		case "consequence":
			consequence = child
		case "alternative":
			alternative = child
		}
	}

	branchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	if condition != nil {
		branchBlock.Statements = []string{"if (" + e.nodeText(condition) + ")"}
	} else {
		branchBlock.Statements = []string{"if"}
	}
	e.addBlock(branchBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, branchBlock.ID, EdgeTypeUnconditional)
	}

	if consequence != nil {
		consequentBlock := e.newBlock(BlockTypePlain, int(node.StartPoint().Row)+1)
		e.addBlock(consequentBlock)
		e.addEdge(branchBlock.ID, consequentBlock.ID, EdgeTypeTrue)
		e.processBlock(consequence, &consequentBlock)
		*currentBlock = consequentBlock
	}

	if alternative != nil {
		elseBlock := e.newBlock(BlockTypePlain, int(alternative.StartPoint().Row)+1)
		e.addBlock(elseBlock)
		e.addEdge(branchBlock.ID, elseBlock.ID, EdgeTypeFalse)
		e.processBlock(alternative, &elseBlock)
		*currentBlock = elseBlock
	} else {
		*currentBlock = branchBlock
	}
}

func (e *phpCFGExtractor) processSwitchStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition *sitter.Node
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "condition":
			condition = child
		case "body":
			body = child
		}
	}

	switchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	if condition != nil {
		switchBlock.Statements = []string{"switch (" + e.nodeText(condition) + ")"}
	} else {
		switchBlock.Statements = []string{"switch"}
	}
	e.addBlock(switchBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, switchBlock.ID, EdgeTypeUnconditional)
	}

	if body != nil {
		e.processBlock(body, currentBlock)
	}

	*currentBlock = switchBlock
}

func (e *phpCFGExtractor) processForStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var init *sitter.Node
	var condition *sitter.Node
	var update *sitter.Node
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "initializer":
			init = child
		case "condition":
			condition = child
		case "update":
			update = child
		case "body":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	var initStr, condStr, updaStr string
	if init != nil {
		initStr = e.nodeText(init)
	}
	if condition != nil {
		condStr = e.nodeText(condition)
	}
	if update != nil {
		updaStr = e.nodeText(update)
	}
	loopHeader.Statements = []string{"for (" + initStr + "; " + condStr + "; " + updaStr + ")"}
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

func (e *phpCFGExtractor) processForeachStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var iterable *sitter.Node
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "iterable":
			iterable = child
		case "body":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	if iterable != nil {
		loopHeader.Statements = []string{"foreach (" + e.nodeText(iterable) + ")"}
	} else {
		loopHeader.Statements = []string{"foreach"}
	}
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

func (e *phpCFGExtractor) processWhileStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition *sitter.Node
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "condition":
			condition = child
		case "body":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	if condition != nil {
		loopHeader.Statements = []string{"while (" + e.nodeText(condition) + ")"}
	} else {
		loopHeader.Statements = []string{"while"}
	}
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

func (e *phpCFGExtractor) processDoWhileStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition *sitter.Node
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "condition":
			condition = child
		case "body":
			body = child
		}
	}

	loopBody := e.newBlock(BlockTypeLoopBody, int(node.StartPoint().Row)+1)
	e.addBlock(loopBody)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, loopBody.ID, EdgeTypeUnconditional)
	}

	if body != nil {
		e.processBlock(body, &loopBody)
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	if condition != nil {
		loopHeader.Statements = []string{"do-while (" + e.nodeText(condition) + ")"}
	} else {
		loopHeader.Statements = []string{"do-while"}
	}
	e.addBlock(loopHeader)

	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	*currentBlock = loopHeader
}

func (e *phpCFGExtractor) processReturnStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	returnBlock := e.newBlock(BlockTypeReturn, int(node.StartPoint().Row)+1)
	returnBlock.Statements = []string{e.nodeText(node)}
	e.addBlock(returnBlock)

	e.addEdge((*currentBlock).ID, returnBlock.ID, EdgeTypeUnconditional)

	*currentBlock = returnBlock
}

func (e *phpCFGExtractor) processBreakStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	breakStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, breakStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeBreak)
}

func (e *phpCFGExtractor) processContinueStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	continueStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, continueStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeContinue)
}

func (e *phpCFGExtractor) processGotoStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	gotoStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, gotoStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeUnconditional)
}

func (e *phpCFGExtractor) processTryStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var tryBlock *sitter.Node
	var catchClauses []*sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "body":
			if tryBlock == nil {
				tryBlock = child
			}
		case "catch_clause":
			catchClauses = append(catchClauses, child)
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

		catchBody := e.findChildByType(catchClause, "body")
		if catchBody != nil {
			e.processBlock(catchBody, &catchBlock)
			lastBlock = catchBlock
		}
	}

	*currentBlock = lastBlock
}

func (e *phpCFGExtractor) processThrowStatement(node *sitter.Node, currentBlock **CFGBlock) {
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

func (e *phpCFGExtractor) processCaseStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	caseStmt := e.nodeText(node)
	if *currentBlock != nil {
		(*currentBlock).Statements = append((*currentBlock).Statements, caseStmt)
		(*currentBlock).EndLine = int(node.EndPoint().Row) + 1
	}
}

func (e *phpCFGExtractor) newBlock(blockType BlockType, line int) *CFGBlock {
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

func (e *phpCFGExtractor) addBlock(block *CFGBlock) {
	e.blocks[block.ID] = block
}

func (e *phpCFGExtractor) addEdge(sourceID, targetID string, edgeType EdgeType) {
	edge := CFGEdge{
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: edgeType,
	}
	e.edges = append(e.edges, edge)
}

func (e *phpCFGExtractor) blocksToMap() map[string]CFGBlock {
	result := make(map[string]CFGBlock)
	for id, block := range e.blocks {
		result[id] = *block
	}
	return result
}

func (e *phpCFGExtractor) calculateCyclomaticComplexity(node *sitter.Node) int {
	if node == nil {
		return 1
	}

	decisionPoints := e.countDecisionPoints(node)
	return decisionPoints + 1
}

func (e *phpCFGExtractor) countDecisionPoints(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0

	switch node.Type() {
	case "if":
		count++

	case "for", "foreach":
		count++

	case "while":
		count++

	case "do":
		count++

	case "switch":
		count += e.countSwitchCases(node)

	case "&&", "||", "and", "or":
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

func (e *phpCFGExtractor) countSwitchCases(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "case" {
			count++
		}
	}
	return count
}

func (e *phpCFGExtractor) findChildByType(node *sitter.Node, childType string) *sitter.Node {
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

func (e *phpCFGExtractor) nodeText(node *sitter.Node) string {
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
