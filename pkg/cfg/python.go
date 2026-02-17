package cfg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

// ExtractCFG extracts the Control Flow Graph from a file for the specified function.
// It dispatches to the appropriate language-specific extractor based on file extension.
func ExtractCFG(filePath string, functionName string) (*CFGInfo, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".py":
		return extractPythonCFG(filePath, functionName)
	case ".go":
		return ExtractGoCFG(filePath, functionName)
	case ".ts", ".tsx":
		return ExtractTSCFG(filePath, functionName)
	case ".rs":
		return ExtractRustCFG(filePath, functionName)
	case ".java":
		return ExtractJavaCFG(filePath, functionName)
	case ".c":
		return ExtractCCFG(filePath, functionName)
	case ".cpp", ".cc":
		return ExtractCppCFG(filePath, functionName)
	case ".rb":
		return ExtractRubyCFG(filePath, functionName)
	case ".php":
		return ExtractPhpCFG(filePath, functionName)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", filePath)
	}
}

// pythonCFGExtractor handles CFG extraction for Python source code.
type pythonCFGExtractor struct {
	content  []byte
	tree     *sitter.Tree
	blocks   map[string]*CFGBlock
	edges    []CFGEdge
	blockID  int
	funcName string
}

// newPythonCFGExtractor creates a new Python CFG extractor.
func newPythonCFGExtractor(content []byte, funcName string) *pythonCFGExtractor {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())
	tree := parser.Parse(nil, content)

	return &pythonCFGExtractor{
		content:  content,
		tree:     tree,
		blocks:   make(map[string]*CFGBlock),
		edges:    make([]CFGEdge, 0),
		blockID:  0,
		funcName: funcName,
	}
}

// extractPythonCFG extracts the Control Flow Graph from a Python file for the specified function.
// It parses the Python source using tree-sitter and builds a CFG representation.
func extractPythonCFG(filePath string, functionName string) (*CFGInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	extractor := newPythonCFGExtractor(content, functionName)
	defer extractor.tree.Close()

	// Find the function definition
	root := extractor.tree.RootNode()
	funcNode := extractor.findFunction(root, functionName)
	if funcNode == nil {
		return nil, fmt.Errorf("function %q not found in %s", functionName, filePath)
	}

	// Find the function body (block)
	blockNode := extractor.findBlock(funcNode)
	if blockNode == nil {
		return nil, fmt.Errorf("function body not found for %s", functionName)
	}

	// Create entry block
	entryBlock := extractor.newBlock(BlockTypeEntry, int(funcNode.StartPoint().Row)+1)
	entryBlock.Statements = []string{"entry"}
	extractor.addBlock(entryBlock)

	// Process the function body to build CFG
	currentBlock := entryBlock
	extractor.processBlock(blockNode, &currentBlock)

	// Create exit block and connect from last block
	exitBlock := extractor.newBlock(BlockTypeExit, int(funcNode.EndPoint().Row)+1)
	exitBlock.Statements = []string{"exit"}
	extractor.addBlock(exitBlock)
	extractor.addEdge(currentBlock.ID, exitBlock.ID, EdgeTypeUnconditional)

	// Calculate cyclomatic complexity
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

// findFunction searches for a function definition node by name.
func (e *pythonCFGExtractor) findFunction(node *sitter.Node, funcName string) *sitter.Node {
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

	// Search in children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		// Don't descend into other class definitions (we handle functions only)
		if child.Type() == "class_definition" {
			continue
		}
		result := e.findFunction(child, funcName)
		if result != nil {
			return result
		}
	}

	return nil
}

// findBlock finds the block node (function body) within a function definition.
func (e *pythonCFGExtractor) findBlock(node *sitter.Node) *sitter.Node {
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

// processBlock processes a block node and builds CFG blocks and edges.
func (e *pythonCFGExtractor) processBlock(blockNode *sitter.Node, currentBlock **CFGBlock) {
	if blockNode == nil {
		return
	}

	for i := 0; i < int(blockNode.ChildCount()); i++ {
		child := blockNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "expression_statement":
			// Add statement to current block
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		case "if_statement":
			e.processIfStatement(child, currentBlock)

		case "for_statement":
			e.processForStatement(child, currentBlock)

		case "while_statement":
			e.processWhileStatement(child, currentBlock)

		case "return_statement":
			e.processReturnStatement(child, currentBlock)

		case "break_statement":
			e.processBreakStatement(child, currentBlock)

		case "continue_statement":
			e.processContinueStatement(child, currentBlock)

		case "try_statement":
			e.processTryStatement(child, currentBlock)

		case "function_definition":
			// Nested function - create separate CFG (simplified: just record it)
			// Full nested function support would require recursive CFG extraction

		case "with_statement":
			// Handle with statement - treat as regular statement
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		case "raise_statement":
			// Handle raise statement
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		case "assert_statement":
			// Handle assert statement
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		case "match_statement":
			// Handle match statement (Python 3.10+)
			e.processMatchStatement(child, currentBlock)

		case "async_for_statement":
			// Handle async for loops
			e.processForStatement(child, currentBlock)

		case "async_with_statement":
			// Handle async with statements
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		case "yield_expression", "yield":
			// Handle yield statements (for generators)
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		case "named_expression":
			// Handle walrus operator (:=) in conditions
			stmt := e.nodeText(child)
			if stmt != "" && *currentBlock != nil {
				(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
				(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
			}

		default:
			// Handle other statements as regular statements
			stmt := e.nodeText(child)
			if stmt != "" && !strings.HasPrefix(strings.TrimSpace(stmt), "#") {
				if *currentBlock != nil {
					(*currentBlock).Statements = append((*currentBlock).Statements, stmt)
					(*currentBlock).EndLine = int(child.EndPoint().Row) + 1
				}
			}
		}
	}
}

// processIfStatement handles if/elif/else statements.
func (e *pythonCFGExtractor) processIfStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	// Get the condition
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

	// Create branch block for the condition
	branchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	branchBlock.Statements = []string{"if " + condition}
	e.addBlock(branchBlock)

	// Connect current block to branch block
	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, branchBlock.ID, EdgeTypeUnconditional)
	}

	// Create consequent (if body) block
	consequentBlock := e.newBlock(BlockTypePlain, int(node.StartPoint().Row)+1)
	e.addBlock(consequentBlock)
	e.addEdge(branchBlock.ID, consequentBlock.ID, EdgeTypeTrue)

	// Track the block before else/elif
	var beforeElseBlock *CFGBlock
	beforeElseBlock = consequentBlock

	// Process consequent
	if consequent != nil {
		e.processBlock(consequent, &consequentBlock)
	}

	// Process alternative (elif/else)
	if alternative != nil {
		// Check if it's an elif or else
		isElse := false
		elseBlock := e.newBlock(BlockTypePlain, int(alternative.StartPoint().Row)+1)
		e.addBlock(elseBlock)

		for i := 0; i < int(alternative.ChildCount()); i++ {
			child := alternative.Child(i)
			if child != nil && child.Type() == "else" {
				isElse = true
				break
			}
		}

		if isElse {
			// Else block - connect from branch with false edge
			e.addEdge(branchBlock.ID, elseBlock.ID, EdgeTypeFalse)
			e.processBlock(alternative, &elseBlock)
			beforeElseBlock = elseBlock
		} else {
			// Elif - need to process similarly to if
			e.addEdge(branchBlock.ID, elseBlock.ID, EdgeTypeFalse)
			e.processElif(alternative, &elseBlock, &beforeElseBlock)
		}
	} else {
		// No else/elif - branch block flows to whatever comes next
		// Update current block to be the last statement in consequent
		*currentBlock = consequentBlock
		return
	}

	// After processing if/else, the current block becomes the last block
	// that can fall through to the next statement
	*currentBlock = beforeElseBlock
}

// processElif handles elif branches.
func (e *pythonCFGExtractor) processElif(node *sitter.Node, currentBlock **CFGBlock, beforeElseBlock **CFGBlock) {
	if node == nil {
		return
	}

	// Similar to if_statement but for elif
	var consequent *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "condition":
			condText := e.nodeText(child)
			_ = condText
		case "consequence":
			consequent = child
		case "alternative":
			// Another elif/else
			elseBlock := e.newBlock(BlockTypePlain, int(child.StartPoint().Row)+1)
			e.addBlock(elseBlock)
			e.addEdge((*currentBlock).ID, elseBlock.ID, EdgeTypeFalse)

			// Check if this is elif or else
			isElse := false
			for j := 0; j < int(child.ChildCount()); j++ {
				grandChild := child.Child(j)
				if grandChild != nil && grandChild.Type() == "else" {
					isElse = true
					break
				}
			}

			if isElse {
				e.processBlock(child, &elseBlock)
				*beforeElseBlock = elseBlock
			} else {
				// Another elif
				newCurrentBlock := elseBlock
				e.processElif(child, &newCurrentBlock, beforeElseBlock)
			}
			return
		}
	}

	if consequent != nil {
		e.processBlock(consequent, currentBlock)
	}
}

// processForStatement handles for loops.
func (e *pythonCFGExtractor) processForStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	// Get for loop header info
	var iterators, source string
	var body *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "left":
			iterators = e.nodeText(child)
		case "right":
			source = e.nodeText(child)
		case "body":
			body = child
		}
	}

	// Create loop header block
	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"for " + iterators + " in " + source}
	e.addBlock(loopHeader)

	// Connect current block to loop header
	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, loopHeader.ID, EdgeTypeUnconditional)
	}

	// Create loop body block
	loopBody := e.newBlock(BlockTypeLoopBody, int(node.StartPoint().Row)+1)
	e.addBlock(loopBody)
	e.addEdge(loopHeader.ID, loopBody.ID, EdgeTypeTrue)

	// Process loop body
	e.processBlock(body, &loopBody)

	// Add back edge from loop body to loop header
	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	// After loop, continue from header (false branch)
	e.addEdge(loopHeader.ID, loopHeader.ID, EdgeTypeFalse) // Self-loop for exit condition

	*currentBlock = loopHeader
}

// processWhileStatement handles while loops.
func (e *pythonCFGExtractor) processWhileStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	// Get while condition and body
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

	// Create loop header block
	loopHeader := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	loopHeader.Statements = []string{"while " + condition}
	e.addBlock(loopHeader)

	// Connect current block to loop header
	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, loopHeader.ID, EdgeTypeUnconditional)
	}

	// Create loop body block
	loopBody := e.newBlock(BlockTypeLoopBody, int(node.StartPoint().Row)+1)
	e.addBlock(loopBody)
	e.addEdge(loopHeader.ID, loopBody.ID, EdgeTypeTrue)

	// Process loop body
	e.processBlock(body, &loopBody)

	// Add back edge from loop body to loop header
	e.addEdge(loopBody.ID, loopHeader.ID, EdgeTypeBackEdge)

	*currentBlock = loopHeader
}

// processReturnStatement handles return statements.
func (e *pythonCFGExtractor) processReturnStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	// Get return value
	var returnValue string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() != "return" {
			returnValue = e.nodeText(child)
			break
		}
	}

	// Create return block
	returnBlock := e.newBlock(BlockTypeReturn, int(node.StartPoint().Row)+1)
	if returnValue != "" {
		returnBlock.Statements = []string{"return " + returnValue}
	} else {
		returnBlock.Statements = []string{"return"}
	}
	e.addBlock(returnBlock)

	// Connect current block to return block
	e.addEdge((*currentBlock).ID, returnBlock.ID, EdgeTypeUnconditional)

	// Update current block to return block (so subsequent statements can still be added)
	// Note: In practice, return exits the function, but we keep track for CFG completeness
	*currentBlock = returnBlock
}

// processBreakStatement handles break statements.
func (e *pythonCFGExtractor) processBreakStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	// Add break statement to current block
	breakStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, breakStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	// Mark as break for later edge resolution (would need loop tracking in full implementation)
	// For now, we add a placeholder edge
	e.addEdge((*currentBlock).ID, "", EdgeTypeBreak)
}

// processContinueStatement handles continue statements.
func (e *pythonCFGExtractor) processContinueStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil || *currentBlock == nil {
		return
	}

	// Add continue statement to current block
	continueStmt := e.nodeText(node)
	(*currentBlock).Statements = append((*currentBlock).Statements, continueStmt)
	(*currentBlock).EndLine = int(node.EndPoint().Row) + 1

	// Mark as continue for later edge resolution
	e.addEdge((*currentBlock).ID, "", EdgeTypeContinue)
}

// processTryStatement handles try/except/finally statements.
func (e *pythonCFGExtractor) processTryStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	// Find all parts of the try statement
	var tryBlock, finallyBlock *sitter.Node
	var exceptHandlers []*sitter.Node

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
		case "exception_handler":
			// Regular except handler
			exceptHandlers = append(exceptHandlers, child)
		case "except_group_clause":
			// Exception group handler (except*) - Python 3.11+
			exceptHandlers = append(exceptHandlers, child)
		case "finally":
			finallyBlock = child
		}
	}

	// Create try block
	tryBody := e.newBlock(BlockTypePlain, int(node.StartPoint().Row)+1)
	tryBody.Statements = []string{"try"}
	e.addBlock(tryBody)

	// Connect current to try
	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, tryBody.ID, EdgeTypeUnconditional)
	}

	// Process try body
	e.processBlock(tryBlock, &tryBody)

	// Handle except handlers
	var lastExceptBlock *CFGBlock
	for _, handler := range exceptHandlers {
		exceptBlock := e.newBlock(BlockTypeBranch, int(handler.StartPoint().Row)+1)
		exceptBlock.Statements = []string{"except"}
		e.addBlock(exceptBlock)

		// Connect from try block (or previous except) to this except
		if lastExceptBlock != nil {
			e.addEdge(lastExceptBlock.ID, exceptBlock.ID, EdgeTypeUnconditional)
		} else {
			e.addEdge(tryBody.ID, exceptBlock.ID, EdgeTypeUnconditional)
		}

		// Process except body
		e.processBlock(handler, &exceptBlock)
		lastExceptBlock = exceptBlock
	}

	// Handle finally
	if finallyBlock != nil {
		finallyBody := e.newBlock(BlockTypePlain, int(finallyBlock.StartPoint().Row)+1)
		finallyBody.Statements = []string{"finally"}
		e.addBlock(finallyBody)

		// Connect from try, excepts, or current to finally
		if lastExceptBlock != nil {
			e.addEdge(lastExceptBlock.ID, finallyBody.ID, EdgeTypeUnconditional)
		} else if tryBody != nil {
			e.addEdge(tryBody.ID, finallyBody.ID, EdgeTypeUnconditional)
		} else if *currentBlock != nil {
			e.addEdge((*currentBlock).ID, finallyBody.ID, EdgeTypeUnconditional)
		}

		e.processBlock(finallyBlock, &finallyBody)
		*currentBlock = finallyBody
	} else if lastExceptBlock != nil {
		*currentBlock = lastExceptBlock
	} else {
		*currentBlock = tryBody
	}
}

// processMatchStatement handles match statements (Python 3.10+).
func (e *pythonCFGExtractor) processMatchStatement(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	// Get the subject (the value being matched)
	var subject string
	var cases *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "subject":
			subject = e.nodeText(child)
		case "cases":
			cases = child
		}
	}

	// Create branch block for the match
	matchBlock := e.newBlock(BlockTypeBranch, int(node.StartPoint().Row)+1)
	matchBlock.Statements = []string{"match " + subject}
	e.addBlock(matchBlock)

	// Connect current block to match block
	if *currentBlock != nil {
		e.addEdge((*currentBlock).ID, matchBlock.ID, EdgeTypeUnconditional)
	}

	// Process each case
	if cases != nil {
		var lastCaseBlock *CFGBlock

		for i := 0; i < int(cases.ChildCount()); i++ {
			caseChild := cases.Child(i)
			if caseChild == nil || caseChild.Type() != "case_clause" {
				continue
			}

			// Create block for this case
			caseBlock := e.newBlock(BlockTypePlain, int(caseChild.StartPoint().Row)+1)
			e.addBlock(caseBlock)

			// Connect from match block (each case is a branch)
			e.addEdge(matchBlock.ID, caseBlock.ID, EdgeTypeTrue)

			// Process case body
			prevBlock := caseBlock
			e.processCaseBlock(caseChild, &prevBlock)

			lastCaseBlock = prevBlock
		}

		// After all cases, continue from the last case block
		if lastCaseBlock != nil {
			*currentBlock = lastCaseBlock
		}
	}
}

// processCaseBlock processes the body of a match case clause.
func (e *pythonCFGExtractor) processCaseBlock(node *sitter.Node, currentBlock **CFGBlock) {
	if node == nil {
		return
	}

	// Find the block containing the case body
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "block" {
			e.processBlock(child, currentBlock)
			break
		}
	}
}

// newBlock creates a new CFG block with a unique ID.
func (e *pythonCFGExtractor) newBlock(blockType BlockType, line int) *CFGBlock {
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

// addBlock adds a block to the CFG.
func (e *pythonCFGExtractor) addBlock(block *CFGBlock) {
	e.blocks[block.ID] = block
}

// addEdge adds an edge between two blocks.
func (e *pythonCFGExtractor) addEdge(sourceID, targetID string, edgeType EdgeType) {
	edge := CFGEdge{
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: edgeType,
	}
	e.edges = append(e.edges, edge)
}

// blocksToMap converts the blocks map to the required format.
func (e *pythonCFGExtractor) blocksToMap() map[string]CFGBlock {
	result := make(map[string]CFGBlock)
	for id, block := range e.blocks {
		result[id] = *block
	}
	return result
}

// calculateCyclomaticComplexity calculates the cyclomatic complexity.
// Formula: decision_points + 1
func (e *pythonCFGExtractor) calculateCyclomaticComplexity(node *sitter.Node) int {
	if node == nil {
		return 1
	}

	decisionPoints := e.countDecisionPoints(node)
	return decisionPoints + 1
}

// countDecisionPoints counts the number of decision points in the code.
func (e *pythonCFGExtractor) countDecisionPoints(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0

	// Count conditional statements as decision points
	switch node.Type() {
	case "if_statement":
		count++

		// Count elif as additional decision points
		alt := e.findChildByType(node, "alternative")
		if alt != nil {
			count += e.countElifBranches(alt)
		}

	case "for_statement":
		count++

	case "while_statement":
		count++

	case "and_operator", "or_operator":
		count++

	case "try_statement":
		// Each except handler adds a decision point
		count += e.countExceptHandlers(node)
	}

	// Recursively count in children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			count += e.countDecisionPoints(child)
		}
	}

	return count
}

// countElifBranches counts elif branches.
func (e *pythonCFGExtractor) countElifBranches(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		// Each elif adds a decision point
		if child.Type() == "elif" {
			count++
		}
		// Recursively count nested elif
		count += e.countElifBranches(child)
	}
	return count
}

// countExceptHandlers counts exception handlers.
func (e *pythonCFGExtractor) countExceptHandlers(node *sitter.Node) int {
	if node == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "exception_handler" {
			count++
		}
	}
	return count
}

// findChildByType finds a child node of a specific type.
func (e *pythonCFGExtractor) findChildByType(node *sitter.Node, childType string) *sitter.Node {
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

// nodeText extracts the text content of a node from the source.
func (e *pythonCFGExtractor) nodeText(node *sitter.Node) string {
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
