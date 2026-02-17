// Package cfg defines data structures for representing Control Flow Graphs (CFGs).
// It provides types for blocks, edges, and the complete CFG information.
package cfg

// BlockType represents the type of a CFG block.
type BlockType string

const (
	BlockTypeEntry    BlockType = "entry"     // Function entry point
	BlockTypeBranch   BlockType = "branch"    // Conditional branch (if/elif/else)
	BlockTypeLoopBody BlockType = "loop_body" // Loop body (for/while)
	BlockTypeReturn   BlockType = "return"    // Return statement
	BlockTypeExit     BlockType = "exit"      // Function exit point
	BlockTypePlain    BlockType = "plain"     // Regular statements
)

// EdgeType represents the type of a CFG edge.
type EdgeType string

const (
	EdgeTypeUnconditional EdgeType = "unconditional" // Unconditional jump
	EdgeTypeTrue          EdgeType = "true"          // True branch of conditional
	EdgeTypeFalse         EdgeType = "false"         // False branch of conditional
	EdgeTypeBackEdge      EdgeType = "back_edge"     // Back edge (loop continuation)
	EdgeTypeBreak         EdgeType = "break"         // Break from loop/switch
	EdgeTypeContinue      EdgeType = "continue"      // Continue to next iteration
)

// CFGBlock represents a basic block in the Control Flow Graph.
// A block is a sequence of statements with a single entry and exit point.
type CFGBlock struct {
	ID           string    `json:"id"`           // Unique identifier for the block
	Type         BlockType `json:"type"`         // Type of block (entry, branch, loop_body, return, exit)
	StartLine    int       `json:"start_line"`   // Starting line number in source
	EndLine      int       `json:"end_line"`     // Ending line number in source
	Statements   []string  `json:"statements"`   // List of statements in this block
	Predecessors []string  `json:"predecessors"` // IDs of blocks that can precede this block
}

// CFGEdge represents a directed edge between two CFG blocks.
type CFGEdge struct {
	SourceID  string   `json:"source_id"`           // ID of the source block
	TargetID  string   `json:"target_id"`           // ID of the target block
	EdgeType  EdgeType `json:"edge_type"`           // Type of edge (true, false, unconditional, etc.)
	Condition string   `json:"condition,omitempty"` // Condition expression for conditional edges
}

// CFGInfo represents the complete Control Flow Graph for a function.
type CFGInfo struct {
	FunctionName         string              `json:"function_name"`         // Name of the function
	Blocks               map[string]CFGBlock `json:"blocks"`                // Map of block ID to block
	Edges                []CFGEdge           `json:"edges"`                 // List of edges in the graph
	EntryBlockID         string              `json:"entry_block_id"`        // ID of the entry block
	ExitBlockIDs         []string            `json:"exit_block_ids"`        // IDs of exit blocks
	CyclomaticComplexity int                 `json:"cyclomatic_complexity"` // Cyclomatic complexity of the function
}
