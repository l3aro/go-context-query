package cfg

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"
)

type rustCFGExtractor struct {
	content  []byte
	tree     *sitter.Tree
	blocks   map[string]*CFGBlock
	edges    []CFGEdge
	blockID  int
	funcName string
}

func newRustCFGExtractor(content []byte, funcName string) *rustCFGExtractor {
	parser := sitter.NewParser()
	parser.SetLanguage(rust.GetLanguage())
	tree := parser.Parse(nil, content)

	return &rustCFGExtractor{
		content:  content,
		tree:     tree,
		blocks:   make(map[string]*CFGBlock),
		edges:    make([]CFGEdge, 0),
		blockID:  0,
		funcName: funcName,
	}
}

func ExtractRustCFG(filePath string, functionName string) (*CFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	extractor := newRustCFGExtractor(content, functionName)
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

func (e *rustCFGExtractor) findFunction(node *sitter.Node, funcName string) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_item" {
		funcNameNode := e.findChildByType(node, "identifier")
		if funcNameNode != nil {
			name := e.nodeText(funcNameNode)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "declaration" || node.Type() == "associated_item" {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "function_item" {
				return e.findFunction(child, funcName)
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

func (e *rustCFGExtractor) findBlock(node *sitter.Node) *sitter.Node {
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

func (e *rustCFGExtractor) processBlock(blockNode *sitter.Node, currentBlock **CFGBlock) {
	if blockNode == nil {
		return
	}

	for i := 0; i < int(blockNode.ChildCount()); i++ {
		child := blockNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "if_expression":
			e.processIfExpression(child, currentBlock)

		case "match_expression":
			e.processMatchExpression(child, currentBlock)

		case "loop_expression":
			e.processLoopExpression(child, currentBlock)

		case "while_expression":
			e.processWhileExpression(child, currentBlock)

		case "for_expression":
			e.processForExpression(child, currentBlock)

		case "return_expression":
			e.processReturnExpression(child, currentBlock)

		case "break_expression":
			e.processBreakExpression(child, currentBlock)

		case "continue_expression":
			e.processContinueExpression(child, currentBlock)

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

func (e *rustCFGExtractor) processIfExpression(node *sitter.Node, currentBlock **CFGBlock) {
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
		case "block":
			if consequent == nil {
				consequent = child
			} else {
				alternative = child
			}
		case "else_clause":
			alternative = child
		}
	}

	branchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	branchBlock.Statements = []string{"if " + condition}
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
		elseIfNode := e.findChildByType(alternative, "if_expression")
		if elseIfNode != nil {
			elseBlock := e.newBlock(BlockTypePlain, int(alternative.StartPoint().Row)+1)
			e.addBlock(elseBlock)
			e.addEdge(branchBlock.ID, elseBlock.ID, EdgeTypeFalse)
			e.processIfExpression(elseIfNode, &elseBlock)
			beforeElseBlock = elseBlock
		} else {
			elseBlock := e.newBlock(BlockTypePlain, int(alternative.StartPoint().Row)+1)
			e.addBlock(elseBlock)
			e.addEdge(branchBlock.ID, elseBlock.ID, EdgeTypeFalse)
			e.processBlock(alternative, &elseBlock)
			beforeElseBlock = elseBlock
		}
	} else {
		*currentBlock = branchBlock
		return
	}

	*currentBlock = beforeElseBlock
}

func (e *rustCFGExtractor) processMatchExpression(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition string
	var matchArms []*sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "scoped_identifier", "identifier", "call_expression":
			if condition == "" {
				condition = e.nodeText(child)
			}
		case "match_arm":
			matchArms = append(matchArms, child)
		case "match_pattern":
			if condition == "" {
				condition = e.nodeText(child)
			}
		}
	}

	matchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	matchBlock.Statements = []string{"match " + condition}
	e.addBlock(matchBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, matchBlock.ID, EdgeTypeUnconditional)
	}

	var lastBlock *CFGBlock
	lastBlock = matchBlock

	for _, arm := range matchArms {
		armBlock := e.newBlock(BlockTypeBranch, int(arm.StartPoint().Row)+1)
		armBlock.Statements = []string{e.nodeText(arm)}
		e.addBlock(armBlock)

		e.addEdge(matchBlock.ID, armBlock.ID, EdgeTypeUnconditional)

		for i := 0; i < int(arm.ChildCount()); i++ {
			child := arm.Child(i)
			if child != nil && (child.Type() == "block" || child.Type() == "expression") {
				armBodyBlock := e.newBlock(BlockTypePlain, int(child.StartPoint().Row)+1)
				e.addBlock(armBodyBlock)
				e.addEdge(armBlock.ID, armBodyBlock.ID, EdgeTypeUnconditional)
				e.processBlock(child, &armBodyBlock)
				lastBlock = armBodyBlock
				break
			}
		}
	}

	*currentBlock = lastBlock
}

func (e *rustCFGExtractor) processLoopExpression(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "block" {
			body = child
			break
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"loop {"}
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

func (e *rustCFGExtractor) processWhileExpression(node *sitter.Node, currentBlock **CFGBlock) {
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
		case "block":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"while " + condition}
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

func (e *rustCFGExtractor) processForExpression(node *sitter.Node, currentBlock **CFGBlock) {
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
		case "for_iterator":
			iterator = e.nodeText(child)
		case "block":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"for " + iterator}
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

func (e *rustCFGExtractor) processReturnExpression(node *sitter.Node, currentBlock **CFGBlock) {
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

func (e *rustCFGExtractor) processBreakExpression(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	breakStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, breakStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeBreak)
}

func (e *rustCFGExtractor) processContinueExpression(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	continueStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, continueStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeContinue)
}

func (e *rustCFGExtractor) newBlock(blockType BlockType, line int) *CFGBlock {
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

func (e *rustCFGExtractor) addBlock(block *CFGBlock) {
	e.blocks[block.ID] = block
}

func (e *rustCFGExtractor) addEdge(sourceID, targetID string, edgeType EdgeType) {
	edge := CFGEdge{
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: edgeType,
	}
	e.edges = append(e.edges, edge)
}

func (e *rustCFGExtractor) blocksToMap() map[string]CFGBlock {
	result := make(map[string]CFGBlock)
	for id, block := range e.blocks {
		result[id] = *block
	}
	return result
}

func (e *rustCFGExtractor) calculateCyclomaticComplexity(node *sitter.Node) int {
	if node == nil {
		return 1
	}

	decisionPoints := e.countDecisionPoints(node)
	return decisionPoints + 1
}

func (e *rustCFGExtractor) countDecisionPoints(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0

	switch node.Type() {
	case "if_expression":
		count++
		alt := e.findChildByType(node, "else_clause")
		if alt != nil {
			count += e.countElseIfBranches(alt)
		}

	case "loop_expression":
		count++

	case "while_expression":
		count++

	case "for_expression":
		count++

	case "match_expression":
		count += e.countMatchArms(node)

	case "match_arm":
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

func (e *rustCFGExtractor) countElseIfBranches(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "if_expression" {
			count++
			alt := e.findChildByType(child, "else_clause")
			if alt != nil {
				count += e.countElseIfBranches(alt)
			}
		}
	}
	return count
}

func (e *rustCFGExtractor) countMatchArms(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "match_arm" {
			count++
		}
	}
	return count
}

func (e *rustCFGExtractor) findChildByType(node *sitter.Node, childType string) *sitter.Node {
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

func (e *rustCFGExtractor) nodeText(node *sitter.Node) string {
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
