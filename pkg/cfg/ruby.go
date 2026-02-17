package cfg

import (
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"
)

type rubyCFGExtractor struct {
	content  []byte
	tree     *sitter.Tree
	blocks   map[string]*CFGBlock
	edges    []CFGEdge
	blockID  int
	funcName string
}

func newRubyCFGExtractor(content []byte, funcName string) *rubyCFGExtractor {
	parser := sitter.NewParser()
	parser.SetLanguage(ruby.GetLanguage())
	tree := parser.Parse(nil, content)

	return &rubyCFGExtractor{
		content:  content,
		tree:     tree,
		blocks:   make(map[string]*CFGBlock),
		edges:    make([]CFGEdge, 0),
		blockID:  0,
		funcName: funcName,
	}
}

func ExtractRubyCFG(filePath string, functionName string) (*CFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	extractor := newRubyCFGExtractor(content, functionName)
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

func (e *rubyCFGExtractor) findMethod(node *sitter.Node, funcName string) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "method" {
		nameNode := e.findChildByType(node, "identifier")
		if nameNode != nil {
			name := e.nodeText(nameNode)
			if name == funcName {
				return node
			}
		}
	}

	if node.Type() == "class" {
		classBody := e.findChildByType(node, "body")
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

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "comment" {
			continue
		}
		result := e.findMethod(child, funcName)
		if result != nil {
			return result
		}
	}

	return nil
}

func (e *rubyCFGExtractor) findBlock(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "body" || node.Type() == "do_block" || node.Type() == "compound_statement" {
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

func (e *rubyCFGExtractor) processBlock(blockNode *sitter.Node, currentBlock **CFGBlock) {
	if blockNode == nil {
		return
	}

	for i := 0; i < int(blockNode.ChildCount()); i++ {
		child := blockNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "if", "elsif", "unless":
			e.processIfStatement(child, currentBlock)

		case "case":
			e.processCaseStatement(child, currentBlock)

		case "while", "until":
			e.processLoopStatement(child, currentBlock)

		case "for":
			e.processForStatement(child, currentBlock)

		case "begin", "rescue", "ensure", "else":
			e.processExceptionHandling(child, currentBlock)

		case "return", "break", "next", "redo", "retry":
			e.processJumpStatement(child, currentBlock)

		default:
			stmt := e.nodeText(child)
			stmt = strings.TrimSpace(stmt)
			if stmt != "" && !strings.HasPrefix(stmt, "#") {
				if *currentBlock != nil {
					(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
					(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
				}
			}
		}
	}
}

func (e *rubyCFGExtractor) processIfStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var consequence *sitter.Node
	var alternative *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "consequence":
			consequence = child
		case "alternative":
			alternative = child
		}
	}

	branchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	branchBlock.Statements = []string{e.nodeText(node)}
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

func (e *rubyCFGExtractor) processCaseStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var condition *sitter.Node
	var whenClauses []*sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "condition":
			condition = child
		case "when":
			whenClauses = append(whenClauses, child)
		}
	}

	caseBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	if condition != nil {
		caseBlock.Statements = []string{"case " + e.nodeText(condition)}
	} else {
		caseBlock.Statements = []string{"case"}
	}
	e.addBlock(caseBlock)

	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, caseBlock.ID, EdgeTypeUnconditional)
	}

	var lastBlock *CFGBlock
	lastBlock = caseBlock

	for _, whenClause := range whenClauses {
		whenBlock := e.newBlock(BlockTypeBranch, int(whenClause.StartPoint().Row)+1)
		whenBlock.Statements = []string{e.nodeText(whenClause)}
		e.addBlock(whenBlock)

		e.addEdge(caseBlock.ID, whenBlock.ID, EdgeTypeUnconditional)

		for i := 0; i < int(whenClause.ChildCount()); i++ {
			child := whenClause.Child(i)
			if child != nil && (child.Type() == "body" || child.Type() == "do_block") {
				whenBodyBlock := e.newBlock(BlockTypePlain, int(child.StartPoint().Row)+1)
				e.addBlock(whenBodyBlock)
				e.addEdge(whenBlock.ID, whenBodyBlock.ID, EdgeTypeUnconditional)
				e.processBlock(child, &whenBodyBlock)
				lastBlock = whenBodyBlock
				break
			}
		}
	}

	*currentBlock = lastBlock
}

func (e *rubyCFGExtractor) processLoopStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "body":
			body = child
		}
	}

	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{e.nodeText(node)}
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

func (e *rubyCFGExtractor) processForStatement(node *sitter.Node, currentBlock **CFGBlock) {
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
		case "iterator":
			iterator = e.nodeText(child)
		case "body":
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

func (e *rubyCFGExtractor) processExceptionHandling(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "begin":
		e.processBlock(node, currentBlock)
	case "rescue":
		e.processBlock(node, currentBlock)
	case "ensure":
		e.processBlock(node, currentBlock)
	case "else":
		e.processBlock(node, currentBlock)
	}
}

func (e *rubyCFGExtractor) processJumpStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	jumpBlock := e.newBlock(BlockTypeReturn, int(node.StartPoint().Row)+1)
	jumpBlock.Statements = []string{e.nodeText(node)}
	e.addBlock(jumpBlock)

	e.addEdge((*currentBlock).ID, jumpBlock.ID, EdgeTypeUnconditional)

	*currentBlock = jumpBlock
}

func (e *rubyCFGExtractor) newBlock(blockType BlockType, line int) *CFGBlock {
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

func (e *rubyCFGExtractor) addBlock(block *CFGBlock) {
	e.blocks[block.ID] = block
}

func (e *rubyCFGExtractor) addEdge(sourceID, targetID string, edgeType EdgeType) {
	edge := CFGEdge{
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: edgeType,
	}
	e.edges = append(e.edges, edge)
}

func (e *rubyCFGExtractor) blocksToMap() map[string]CFGBlock {
	result := make(map[string]CFGBlock)
	for id, block := range e.blocks {
		result[id] = *block
	}
	return result
}

func (e *rubyCFGExtractor) calculateCyclomaticComplexity(node *sitter.Node) int {
	if node == nil {
		return 1
	}

	decisionPoints := e.countDecisionPoints(node)
	return decisionPoints + 1
}

func (e *rubyCFGExtractor) countDecisionPoints(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0

	switch node.Type() {
	case "if", "elsif", "unless":
		count++

	case "while", "until":
		count++

	case "for":
		count++

	case "case":
		count += e.countWhenClauses(node)

	case "and", "or", "&&", "||":
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

func (e *rubyCFGExtractor) countWhenClauses(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "when" {
			count++
		}
	}
	return count
}

func (e *rubyCFGExtractor) findChildByType(node *sitter.Node, childType string) *sitter.Node {
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

func (e *rubyCFGExtractor) nodeText(node *sitter.Node) string {
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
