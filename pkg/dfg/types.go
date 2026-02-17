// Package dfg defines data structures for representing Data Flow Graphs (DFGs).
// It provides types for variable references, data flow edges, and DFG information.
package dfg

// RefType represents the type of variable reference in data flow analysis.
type RefType string

const (
	RefTypeDefinition RefType = "definition" // Variable definition (assignment)
	RefTypeUpdate     RefType = "update"     // Variable update (reassignment)
	RefTypeUse        RefType = "use"        // Variable use (read)
)

// VarRef represents a variable reference in the source code.
// It captures the variable name, its reference type, and position.
type VarRef struct {
	Name    string  `json:"name"`     // Variable name
	RefType RefType `json:"ref_type"` // Type of reference (definition, update, use)
	Line    int     `json:"line"`     // Line number in source
	Column  int     `json:"column"`   // Column number in source
}

// DataflowEdge represents a data flow edge between two variable references.
// It connects a definition/update to a use of a variable.
type DataflowEdge struct {
	DefRef  VarRef `json:"def_ref"`  // Definition or update reference
	UseRef  VarRef `json:"use_ref"`  // Use reference
	VarName string `json:"var_name"` // Name of the variable being tracked
}

// DFGInfo represents the complete Data Flow Graph for a function.
// It contains all variable references, data flow edges, and grouped variables.
type DFGInfo struct {
	FunctionName  string              `json:"function_name"`  // Name of the function
	VarRefs       []VarRef            `json:"var_refs"`       // All variable references in order
	DataflowEdges []DataflowEdge      `json:"dataflow_edges"` // Data flow edges between references
	Variables     map[string][]VarRef `json:"variables"`      // Variables grouped by name
	Imports       []string            `json:"imports"`        // Imported modules/functions
}
