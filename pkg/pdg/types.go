// Package pdg defines data structures for representing Program Dependence Graphs (PDGs).
// It provides types for nodes, edges, and the complete PDG information,
// integrating control flow (CFG) and data flow (DFG) information.
package pdg

import (
	"github.com/l3aro/go-context-query/pkg/cfg"
	"github.com/l3aro/go-context-query/pkg/dfg"
)

// NodeType represents the type of a PDG node.
type NodeType string

const (
	NodeTypeStatement NodeType = "statement" // Regular statement node
	NodeTypeBranch    NodeType = "branch"    // Conditional branch node
	NodeTypeLoop      NodeType = "loop"      // Loop node
	NodeTypeEntry     NodeType = "entry"     // Function entry point
	NodeTypeExit      NodeType = "exit"      // Function exit point
)

// DepType represents the type of dependence in a PDG edge.
type DepType string

const (
	DepTypeControl DepType = "control" // Control dependence
	DepTypeData    DepType = "data"    // Data dependence
)

// PDGNode represents a node in the Program Dependence Graph.
// It combines control flow information with data flow information.
type PDGNode struct {
	ID          string       `json:"id"`           // Unique identifier for the node
	Type        NodeType     `json:"type"`         // Type of node (statement, branch, loop, entry, exit)
	StartLine   int          `json:"start_line"`   // Starting line number in source
	EndLine     int          `json:"end_line"`     // Ending line number in source
	Definitions []dfg.VarRef `json:"definitions"`  // Variable definitions in this node
	Uses        []dfg.VarRef `json:"uses"`         // Variable uses in this node
	CFGBlockID  string       `json:"cfg_block_id"` // ID of the corresponding CFG block
}

// PDGEdge represents a directed edge between two PDG nodes.
// Edges represent either control or data dependencies.
type PDGEdge struct {
	SourceID string  `json:"source_id"` // ID of the source node
	TargetID string  `json:"target_id"` // ID of the target node
	DepType  DepType `json:"dep_type"`  // Type of dependence (control or data)
	Label    string  `json:"label"`     // Optional label for the edge
}

// PDGInfo represents the complete Program Dependence Graph for a function.
// It integrates CFG and DFG information into a unified dependence graph.
type PDGInfo struct {
	FunctionName string             `json:"function_name"` // Name of the function
	CFG          *cfg.CFGInfo       `json:"cfg"`           // Control Flow Graph information
	DFG          *dfg.DFGInfo       `json:"dfg"`           // Data Flow Graph information
	Nodes        map[string]PDGNode `json:"nodes"`         // Map of node ID to node
	Edges        []PDGEdge          `json:"edges"`         // List of edges in the graph

	// Cached edge maps for efficient traversal
	incomingCache map[string][]PDGEdge // cached incoming edges
	outgoingCache map[string][]PDGEdge // cached outgoing edges
	cacheValid    bool                 // whether cache is valid
}
