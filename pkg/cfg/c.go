package cfg

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
)

type cCFGExtractor struct {
	content  []byte
	tree     *sitter.Tree
	blocks   map[string]*CFGBlock
	edges    []CFGEdge
	blockID  int
	funcName string
}

func newCCFGExtractor(content []byte, funcName string) *cCFGExtractor {
	parser := sitter.NewParser()
	parser.SetLanguage(c.GetLanguage())
	tree := parser.Parse(nil, content)

	return &cCFGExtractor{
		content:  content,
		tree:     tree,
		blocks:   make(map[string]*CFGBlock),
		edges:    make([]CFGEdge, 0),
		blockID:  0,
		funcName: funcName,
	}
}

// ExtractCCFG extracts the Control Flow Graph from a C function.
func ExtractCCFG(filePath string, functionName string) (*CFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	extractor := newCCFGExtractor(content, functionName)
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

func (e *cCFGExtractor) findFunction(node *sitter.Node, funcName string) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "function_definition" {
		funcNameNode := e.findChildByType(node, "identifier")
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
		if child.Type() == "function_definition" {
			funcNameNode := e.findChildByType(child, "identifier")
			if funcNameNode != nil && e.nodeText(funcNameNode) == funcName {
				return child
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

func (e *cCFGExtractor) findBlock(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "compound_statement" {
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

func (e *cCFGExtractor) processBlock(blockNode *sitter.Node, currentBlock **CFGBlock) {
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

		case "goto_statement":
			e.processGotoStatement(child, currentBlock)

		case "label":
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		case "case_statement":
			e.processCaseStatement(child, currentBlock)

		case "attribute":

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

func (e *cCFGExtractor) processIfStatement(node *sitter.Node, currentBlock **CFGBlock) {
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

		hasElseBody := false
		for i := 0; i < int(alternative.ChildCount()); i++ {
			child := alternative.Child(i)
			if child != nil && (child.Type() == "compound_statement" || child.Type() == "if_statement") {
				hasElseBody = true
				break
			}
		}

		if hasElseBody {
			e.processBlock(alternative, &elseBlock)
			beforeElseBlock = elseBlock
		} else {
			// else if case
			elseIfBlock := e.newBlock(BlockTypePlain, int(alternative.StartPoint().Row)+1)
			e.addBlock(elseIfBlock)
			e.addEdge(branchBlock.ID, elseIfBlock.ID, EdgeTypeFalse)

			for i := 0; i < int(alternative.ChildCount()); i++ {
				child := alternative.Child(i)
				if child != nil && child.Type() == "if_statement" {
					e.processIfStatement(child, &elseIfBlock)
					beforeElseBlock = elseIfBlock
					break
				}
			}
		}
	} else {
		*currentBlock = branchBlock
		return
	}

	*currentBlock = beforeElseBlock
}

func (e *cCFGExtractor) processSwitchStatement(node *sitter.Node, currentBlock **CFGBlock) {
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

	switchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	switchBlock.Statements = []string{"switch (" + condition + ")"}
	e.addBlock(switchBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, switchBlock.ID, EdgeTypeUnconditional)
	}

	var lastBlock *CFGBlock
	lastBlock = switchBlock

	if body != nil {
		for i := 0; i < int(body.ChildCount()); i++ {
			child := body.Child(i)
			if child == nil {
				continue
			}

			if child.Type() == "case_statement" || child.Type() == "labeled_statement" {
				caseBlock := e.newBlock(BlockTypeBranch, int(child.StartPoint().Row)+1)
				caseBlock.Statements = []string{e.nodeText(child)}
				e.addBlock(caseBlock)

				e.addEdge(switchBlock.ID, caseBlock.ID, EdgeTypeUnconditional)

				// Find the statement after case label
				for j := 0; j < int(child.ChildCount()); j++ {
					stmt := child.Child(j)
					if stmt != nil && stmt.Type() != "case" && stmt.Type() != "default" {
						caseBodyBlock := e.newBlock(BlockTypePlain, int(stmt.StartPoint().Row)+1)
						e.addBlock(caseBodyBlock)
						e.addEdge(caseBlock.ID, caseBodyBlock.ID, EdgeTypeUnconditional)

						if stmt.Type() == "compound_statement" {
							e.processBlock(stmt, &caseBodyBlock)
						} else {
							stmtText := e.nodeText(stmt)
							stmtText = strings.TrimSpace(stmtText)
							if stmtText != "" {
								caseBodyBlock.Statements = []string{stmtText}
								caseBodyBlock.EndLine = int(stmt.EndPoint().Row) + 1
							}
						}
						lastBlock = caseBodyBlock
						break
					}
				}
			}
		}
	}

	*currentBlock = lastBlock
}

func (e *cCFGExtractor) processForStatement(node *sitter.Node, currentBlock **CFGBlock) {
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

	e.processBlock(body, &loopBody)

	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	*currentBlock = loopHeader
}

func (e *cCFGExtractor) processWhileStatement(node *sitter.Node, currentBlock **CFGBlock) {
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

	e.processBlock(body, &loopBody)

	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	*currentBlock = loopHeader
}

func (e *cCFGExtractor) processDoWhileStatement(node *sitter.Node, currentBlock **CFGBlock) {
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
	e.addBlock(loopBody)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, loopBody.ID, EdgeTypeUnconditional)
	}

	e.processBlock(body, &loopBody)

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"do-while (" + condition + ")"}
	e.addBlock(loopHeader)

	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	*currentBlock = loopHeader
}

func (e *cCFGExtractor) processReturnStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	returnBlock := e.newBlock(BlockTypeReturn, int(node.StartPoint().Row)+1)
	returnBlock.Statements = []string{e.nodeText(node)}
	e.addBlock(returnBlock)

	e.addEdge((*currentBlock).ID, returnBlock.ID, EdgeTypeUnconditional)

	*currentBlock = returnBlock
}

func (e *cCFGExtractor) processBreakStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	breakStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, breakStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeBreak)
}

func (e *cCFGExtractor) processContinueStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	continueStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, continueStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeContinue)
}

func (e *cCFGExtractor) processGotoStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	gotoStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, gotoStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	e.addEdge((*currentBlock).ID, "", EdgeTypeUnconditional)
}

func (e *cCFGExtractor) processCaseStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	caseStmt := e.nodeText(node)
	if *currentBlock != nil {
		(*currentBlock).Statements = append((*currentBlock).Statements, caseStmt)
		(*currentBlock).EndLine = int(node.EndPoint().Row) + 1
	}
}

func (e *cCFGExtractor) newBlock(blockType BlockType, line int) *CFGBlock {
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

func (e *cCFGExtractor) addBlock(block *CFGBlock) {
	e.blocks[block.ID] = block
}

func (e *cCFGExtractor) addEdge(sourceID, targetID string, edgeType EdgeType) {
	edge := CFGEdge{
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: edgeType,
	}
	e.edges = append(e.edges, edge)
}

func (e *cCFGExtractor) blocksToMap() map[string]CFGBlock {
	result := make(map[string]CFGBlock)
	for id, block := range e.blocks {
		result[id] = *block
	}
	return result
}

func (e *cCFGExtractor) calculateCyclomaticComplexity(node *sitter.Node) int {
	if node == nil {
		return 1
	}

	decisionPoints := e.countDecisionPoints(node)
	return decisionPoints + 1
}

func (e *cCFGExtractor) countDecisionPoints(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0

	switch node.Type() {
	case "if_statement":
		count++

	case "for_statement":
		count++

	case "while_statement":
		count++

	case "do_statement":
		count++

	case "switch_statement":
		count += e.countSwitchCases(node)

	case "case_statement":
		count++

	case "&&", "||":
		count++

	case "conditional_expression":
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

func (e *cCFGExtractor) countSwitchCases(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && (child.Type() == "case_statement" || child.Type() == "labeled_statement") {
			count++
		}
	}
	return count
}

func (e *cCFGExtractor) findChildByType(node *sitter.Node, childType string) *sitter.Node {
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

func (e *cCFGExtractor) nodeText(node *sitter.Node) string {
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
