package cfg

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

type goCFGExtractor struct {
	content  []byte
	tree     *sitter.Tree
	blocks   map[string]*CFGBlock
	edges    []CFGEdge
	blockID  int
	funcName string
}

func newGoCFGExtractor(content []byte, funcName string) *goCFGExtractor {
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	tree := parser.Parse(nil, content)

	return &goCFGExtractor{
		content:  content,
		tree:     tree,
		blocks:   make(map[string]*CFGBlock),
		edges:    make([]CFGEdge, 0),
		blockID:  0,
		funcName: funcName,
	}
}

func ExtractGoCFG(filePath string, functionName string) (*CFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	extractor := newGoCFGExtractor(content, functionName)
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
	extractor.addEdge(currentBlock.ID, exitBlock.ID, EdgeTypeUnconditional)

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

func (e *goCFGExtractor) findFunction(node *sitter.Node, funcName string) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_declaration" {
		funcNameNode := e.findChildByType(node, "identifier")
		if funcNameNode != nil {
			name := e.nodeText(funcNameNode)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "method_declaration" {
		funcNameNode := e.findChildByType(node, "field_identifier")
		if funcNameNode != nil {
			name := e.nodeText(funcNameNode)
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
		if child.Type() == "function_declaration" || child.Type() == "method_declaration" {
			if child.Type() == "function_declaration" {
				funcNameNode := e.findChildByType(child, "identifier")
				if funcNameNode != nil && e.nodeText(funcNameNode) == funcName {
					return child
				}
			} else if child.Type() == "method_declaration" {
				funcNameNode := e.findChildByType(child, "field_identifier")
				if funcNameNode != nil && e.nodeText(funcNameNode) == funcName {
					return child
				}
			}
			continue
		}
		result := e.findFunction(child, funcName)
		if result != nil {
			return result
		}
	}

	return nil
}

func (e *goCFGExtractor) findBlock(node *sitter.Node) *sitter.Node {
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

func (e *goCFGExtractor) processBlock(blockNode *sitter.Node, currentBlock **CFGBlock) {
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

		case "return_statement":
			e.processReturnStatement(child, currentBlock)

		case "break_statement":
			e.processBreakStatement(child, currentBlock)

		case "continue_statement":
			e.processContinueStatement(child, currentBlock)

		case "goto_statement":
			e.processGotoStatement(child, currentBlock)

		case "label_statement":
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		case "select_statement":
			e.processSelectStatement(child, currentBlock)

		case "defer_statement":
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		case "go_statement":
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		default:
			stmt := e.nodeText(child)
			stmt = strings.TrimSpace(stmt)
			if stmt != "" && !strings.HasPrefix(stmt, "//") && !strings.HasPrefix(stmt, "/*") {
				if *currentBlock != nil {
					(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
					(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
				}
			}
		}
	}
}

func (e *goCFGExtractor) processIfStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition string
	var initStmt string
	var consequent *sitter.Node
	var alternative *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "init":
			initStmt = e.nodeText(child)
		case "condition":
			condition = e.nodeText(child)
		case "consequence":
			consequent = child
		case "alternative":
			alternative = child
		}
	}

	branchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	if initStmt != "" {
		branchBlock.Statements = []string{initStmt + "; if " + condition}
	} else {
		branchBlock.Statements = []string{"if " + condition}
	}
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

		hasElseBody := false
		for i := 0; i < int(alternative.ChildCount()); i++ {
			child := alternative.Child(i)
			if child != nil && child.Type() == "block" {
				hasElseBody = true
				break
			}
		}

		if hasElseBody {
			e.addEdge(branchBlock.ID, elseBlock.ID, EdgeTypeFalse)
			e.processBlock(alternative, &elseBlock)
			beforeElseBlock = elseBlock
		} else {
			e.addEdge(branchBlock.ID, elseBlock.ID, EdgeTypeFalse)
			e.processElseIf(alternative, &elseBlock, &beforeElseBlock)
		}
	} else {
		*currentBlock = branchBlock
		return
	}

	*currentBlock = beforeElseBlock
}

func (e *goCFGExtractor) processElseIf(node *sitter.Node, currentBlock **CFGBlock, beforeElseBlock **CFGBlock) {
	if node == nil {
		return
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "if_statement" {
			elseIfBlock := e.newBlock(BlockTypePlain, int(child.StartPoint().Row)+1)
			e.addBlock(elseIfBlock)
			e.addEdge((*currentBlock).ID, elseIfBlock.ID, EdgeTypeFalse)

			e.processIfStatement(child, &elseIfBlock)
			*beforeElseBlock = elseIfBlock
			*currentBlock = elseIfBlock
		} else if child.Type() == "block" {
			e.processBlock(child, currentBlock)
			*beforeElseBlock = *currentBlock
		}
	}
}

func (e *goCFGExtractor) processSwitchStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var initStmt string
	var condition string
	var caseList []*sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "init":
			initStmt = e.nodeText(child)
		case "condition":
			condition = e.nodeText(child)
		case "case_clause":
			caseList = append(caseList, child)
		}
	}

	switchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	if initStmt != "" {
		switchBlock.Statements = []string{initStmt + "; switch " + condition}
	} else {
		switchBlock.Statements = []string{"switch " + condition}
	}
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

	hasDefault := false
	for _, sc := range caseList {
		for i := 0; i < int(sc.ChildCount()); i++ {
			child := sc.Child(i)
			if child != nil && child.Type() == "default" {
				hasDefault = true
				break
			}
		}
		if hasDefault {
			break
		}
	}

	if !hasDefault {
		*currentBlock = lastBlock
	} else {
		*currentBlock = lastBlock
	}
}

func (e *goCFGExtractor) processForStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var initStmt string
	var condition string
	var postStmt string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "init":
			initStmt = e.nodeText(child)
		case "condition":
			condition = e.nodeText(child)
		case "post":
			postStmt = e.nodeText(child)
		case "body":
			body = child
		}
	}

	isRange := false
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "range_clause" {
			isRange = true
			break
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	if isRange {
		loopHeader.Statements = []string{"for range ..."}
	} else if initStmt != "" || condition != "" || postStmt != "" {
		loopHeader.Statements = []string{"for " + initStmt + "; " + condition + "; " + postStmt}
	} else {
		loopHeader.Statements = []string{"for {}"}
	}
	e.addBlock(loopHeader)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, loopHeader.ID, EdgeTypeUnconditional)
	}

	loopBody := e.newBlock(BlockTypeLoopBody, int(node.StartPoint().Row)+1)
	e.addBlock(loopBody)
	e.addEdge(loopHeader.ID, loopBody.ID, EdgeTypeTrue)

	e.processBlock(body, &loopBody)

	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	*currentBlock = loopHeader
}

func (e *goCFGExtractor) processReturnStatement(node *sitter.Node, currentBlock **CFGBlock) {
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

func (e *goCFGExtractor) processBreakStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	breakStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, breakStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeBreak)
}

func (e *goCFGExtractor) processContinueStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	continueStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, continueStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeContinue)
}

func (e *goCFGExtractor) processGotoStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	gotoStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, gotoStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeUnconditional)
}

func (e *goCFGExtractor) processSelectStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	selectBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	selectBlock.Statements = []string{"select {"}
	e.addBlock(selectBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, selectBlock.ID, EdgeTypeUnconditional)
	}

	var caseList []*sitter.Node
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && (child.Type() == "case_clause" || child.Type() == "communication_case") {
			caseList = append(caseList, child)
		}
	}

	var lastBlock *CFGBlock
	lastBlock = selectBlock

	for _, sc := range caseList {
		caseBlock := e.newBlock(BlockTypeBranch, int(sc.StartPoint().Row)+1)
		caseBlock.Statements = []string{e.nodeText(sc)}
		e.addBlock(caseBlock)

		e.addEdge(selectBlock.ID, caseBlock.ID, EdgeTypeUnconditional)

		caseBody := e.findChildByType(sc, "block")
		if caseBody != nil {
			caseBodyBlock := e.newBlock(BlockTypePlain, int(caseBody.StartPoint().Row)+1)
			e.addBlock(caseBodyBlock)
			e.addEdge(caseBlock.ID, caseBodyBlock.ID, EdgeTypeUnconditional)

			e.processBlock(caseBody, &caseBodyBlock)
			lastBlock = caseBodyBlock
		}
	}

	*currentBlock = lastBlock
}

func (e *goCFGExtractor) newBlock(blockType BlockType, line int) *CFGBlock {
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

func (e *goCFGExtractor) addBlock(block *CFGBlock) {
	e.blocks[block.ID] = block
}

func (e *goCFGExtractor) addEdge(sourceID, targetID string, edgeType EdgeType) {
	edge := CFGEdge{
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: edgeType,
	}
	e.edges = append(e.edges, edge)
}

func (e *goCFGExtractor) blocksToMap() map[string]CFGBlock {
	result := make(map[string]CFGBlock)
	for id, block := range e.blocks {
		result[id] = *block
	}
	return result
}

func (e *goCFGExtractor) calculateCyclomaticComplexity(node *sitter.Node) int {
	if node == nil {
		return 1
	}

	decisionPoints := e.countDecisionPoints(node)
	return decisionPoints + 1
}

func (e *goCFGExtractor) countDecisionPoints(node *sitter.Node) int {
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

	case "range_clause":
		count++

	case "switch_statement":
		count += e.countSwitchCases(node)

	case "select_statement":
		count += e.countSelectCases(node)

	case "case_clause":
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

func (e *goCFGExtractor) countElseIfBranches(node *sitter.Node) int {
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

func (e *goCFGExtractor) countSwitchCases(node *sitter.Node) int {
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

func (e *goCFGExtractor) countSelectCases(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && (child.Type() == "case_clause" || child.Type() == "communication_case") {
			count++
		}
	}
	return count
}

func (e *goCFGExtractor) findChildByType(node *sitter.Node, childType string) *sitter.Node {
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

func (e *goCFGExtractor) nodeText(node *sitter.Node) string {
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
