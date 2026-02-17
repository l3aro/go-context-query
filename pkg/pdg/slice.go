// Package pdg provides slicing algorithms for Program Dependence Graphs.
// These algorithms enable backward and forward slice analysis to find
// data and control dependencies between lines of code.
package pdg

import (
	"container/list"
	"sort"

	"github.com/l3aro/go-context-query/pkg/dfg"
)

// DependencyInfo contains the control and data dependencies for a specific line.
// It separates incoming and outgoing edges for both control and data dependence types.
type DependencyInfo struct {
	ControlIn  []PDGEdge // Edges representing control dependence into this line
	ControlOut []PDGEdge // Edges representing control dependence from this line
	DataIn     []PDGEdge // Edges representing data dependence into this line
	DataOut    []PDGEdge // Edges representing data dependence from this line
}

// buildEdgeMaps creates incoming and outgoing edge maps for efficient traversal.
// incoming[nodeID] contains all edges pointing TO that node
// outgoing[nodeID] contains all edges originating FROM that node
func buildEdgeMaps(pdg *PDGInfo) (incoming map[string][]PDGEdge, outgoing map[string][]PDGEdge) {
	incoming = make(map[string][]PDGEdge)
	outgoing = make(map[string][]PDGEdge)

	for _, edge := range pdg.Edges {
		outgoing[edge.SourceID] = append(outgoing[edge.SourceID], edge)
		incoming[edge.TargetID] = append(incoming[edge.TargetID], edge)
	}
	return
}

// findNodesAtLine finds all PDG node IDs that contain the given line number.
// A line can belong to multiple nodes if they share the same line range.
func findNodesAtLine(pdg *PDGInfo, line int) []string {
	var nodeIDs []string
	for id, node := range pdg.Nodes {
		if line >= node.StartLine && line <= node.EndLine {
			nodeIDs = append(nodeIDs, id)
		}
	}
	return nodeIDs
}

// getLineNumbers extracts unique line numbers from a set of node IDs.
func getLineNumbers(pdg *PDGInfo, nodeIDs []string) []int {
	lineSet := make(map[int]struct{})
	for _, id := range nodeIDs {
		if node, ok := pdg.Nodes[id]; ok {
			// Add all lines in the node's range
			for line := node.StartLine; line <= node.EndLine; line++ {
				lineSet[line] = struct{}{}
			}
		}
	}

	// Convert to sorted slice
	lines := make([]int, 0, len(lineSet))
	for line := range lineSet {
		lines = append(lines, line)
	}
	sort.Ints(lines)
	return lines
}

// BackwardSlice performs a backward slice analysis starting from a given line.
// It finds all lines that may affect the value at the target line.
// If a variable filter is provided, only data edges with matching labels are followed.
func BackwardSlice(pdg *PDGInfo, line int, variable *string) []int {
	if pdg == nil {
		return nil
	}

	// Build edge maps for efficient traversal
	incoming, _ := buildEdgeMaps(pdg)

	// Find starting nodes at the target line
	startNodes := findNodesAtLine(pdg, line)
	if len(startNodes) == 0 {
		return nil
	}

	// BFS with visited set to avoid infinite loops
	visited := make(map[string]bool)
	queue := list.New()
	for _, nodeID := range startNodes {
		queue.PushBack(nodeID)
		visited[nodeID] = true
	}

	resultNodes := make([]string, 0)

	for queue.Len() > 0 {
		elem := queue.Remove(queue.Front())
		currentNodeID := elem.(string)
		resultNodes = append(resultNodes, currentNodeID)

		// Process all incoming edges
		for _, edge := range incoming[currentNodeID] {
			// If variable filter is specified, only follow data edges with matching label
			if variable != nil && edge.DepType == DepTypeData {
				if edge.Label != *variable {
					continue
				}
			}

			// Skip if already visited
			if visited[edge.SourceID] {
				continue
			}
			visited[edge.SourceID] = true
			queue.PushBack(edge.SourceID)
		}
	}

	return getLineNumbers(pdg, resultNodes)
}

// ForwardSlice performs a forward slice analysis starting from a given line.
// It finds all lines that may be affected by the value at the source line.
// If a variable filter is provided, only data edges with matching labels are followed.
func ForwardSlice(pdg *PDGInfo, line int, variable *string) []int {
	if pdg == nil {
		return nil
	}

	// Build edge maps for efficient traversal
	_, outgoing := buildEdgeMaps(pdg)

	// Find starting nodes at the source line
	startNodes := findNodesAtLine(pdg, line)
	if len(startNodes) == 0 {
		return nil
	}

	// BFS with visited set to avoid infinite loops
	visited := make(map[string]bool)
	queue := list.New()
	for _, nodeID := range startNodes {
		queue.PushBack(nodeID)
		visited[nodeID] = true
	}

	resultNodes := make([]string, 0)

	for queue.Len() > 0 {
		elem := queue.Remove(queue.Front())
		currentNodeID := elem.(string)
		resultNodes = append(resultNodes, currentNodeID)

		// Process all outgoing edges
		for _, edge := range outgoing[currentNodeID] {
			// If variable filter is specified, only follow data edges with matching label
			if variable != nil && edge.DepType == DepTypeData {
				if edge.Label != *variable {
					continue
				}
			}

			// Skip if already visited
			if visited[edge.TargetID] {
				continue
			}
			visited[edge.TargetID] = true
			queue.PushBack(edge.TargetID)
		}
	}

	return getLineNumbers(pdg, resultNodes)
}

// GetDependencies returns all dependencies for a specific line.
// It separates control and data dependencies into incoming and outgoing categories.
func GetDependencies(pdg *PDGInfo, line int) DependencyInfo {
	if pdg == nil {
		return DependencyInfo{}
	}

	// Find nodes at the target line
	nodeIDs := findNodesAtLine(pdg, line)
	if len(nodeIDs) == 0 {
		return DependencyInfo{}
	}

	// Build edge maps
	incoming, outgoing := buildEdgeMaps(pdg)

	// Collect all unique edges for all nodes at this line
	var controlIn, controlOut, dataIn, dataOut []PDGEdge

	// Use a set to avoid duplicate edges
	seenControlIn := make(map[string]bool)
	seenControlOut := make(map[string]bool)
	seenDataIn := make(map[string]bool)
	seenDataOut := make(map[string]bool)

	addEdgeIfUnique := func(edge PDGEdge, depType DepType, isIncoming bool) {
		// Create a unique key for the edge
		key := edge.SourceID + "->" + edge.TargetID + ":" + edge.Label

		if depType == DepTypeControl {
			if isIncoming {
				if !seenControlIn[key] {
					seenControlIn[key] = true
					controlIn = append(controlIn, edge)
				}
			} else {
				if !seenControlOut[key] {
					seenControlOut[key] = true
					controlOut = append(controlOut, edge)
				}
			}
		} else {
			if isIncoming {
				if !seenDataIn[key] {
					seenDataIn[key] = true
					dataIn = append(dataIn, edge)
				}
			} else {
				if !seenDataOut[key] {
					seenDataOut[key] = true
					dataOut = append(dataOut, edge)
				}
			}
		}
	}

	// Collect edges for all nodes at this line
	for _, nodeID := range nodeIDs {
		// Incoming edges
		for _, edge := range incoming[nodeID] {
			addEdgeIfUnique(edge, edge.DepType, true)
		}

		// Outgoing edges
		for _, edge := range outgoing[nodeID] {
			addEdgeIfUnique(edge, edge.DepType, false)
		}
	}

	// Sort edges by source and target for consistent ordering
	sortEdges := func(edges []PDGEdge) {
		sort.Slice(edges, func(i, j int) bool {
			if edges[i].SourceID != edges[j].SourceID {
				return edges[i].SourceID < edges[j].SourceID
			}
			return edges[i].TargetID < edges[j].TargetID
		})
	}

	sortEdges(controlIn)
	sortEdges(controlOut)
	sortEdges(dataIn)
	sortEdges(dataOut)

	return DependencyInfo{
		ControlIn:  controlIn,
		ControlOut: controlOut,
		DataIn:     dataIn,
		DataOut:    dataOut,
	}
}

// GetVariableNames returns all unique variable names used in data edges
// for the given PDG. This is useful for building variable filters.
func GetVariableNames(pdg *PDGInfo) []string {
	if pdg == nil {
		return nil
	}

	varSet := make(map[string]bool)
	for _, edge := range pdg.Edges {
		if edge.DepType == DepTypeData && edge.Label != "" {
			varSet[edge.Label] = true
		}
	}

	variables := make([]string, 0, len(varSet))
	for v := range varSet {
		variables = append(variables, v)
	}
	sort.Strings(variables)
	return variables
}

// FindNodesByVariable filters PDG nodes to find those that reference
// a specific variable in their definitions or uses.
func FindNodesByVariable(pdg *PDGInfo, varName string) []string {
	if pdg == nil {
		return nil
	}

	var nodeIDs []string
	for _, node := range pdg.Nodes {
		for _, def := range node.Definitions {
			if def.Name == varName {
				nodeIDs = append(nodeIDs, node.ID)
				break
			}
		}
		for _, use := range node.Uses {
			if use.Name == varName {
				nodeIDs = append(nodeIDs, node.ID)
				break
			}
		}
	}
	return nodeIDs
}

// GetNodeAtLine returns the PDGNode at a specific line, if exactly one exists.
// Returns nil if no node or multiple nodes exist at that line.
func GetNodeAtLine(pdg *PDGInfo, line int) *PDGNode {
	if pdg == nil {
		return nil
	}

	nodes := findNodesAtLine(pdg, line)
	if len(nodes) != 1 {
		return nil
	}

	node, ok := pdg.Nodes[nodes[0]]
	if !ok {
		return nil
	}
	return &node
}

// GetAllNodesAtLine returns all PDGNodes that contain a specific line.
func GetAllNodesAtLine(pdg *PDGInfo, line int) []PDGNode {
	if pdg == nil {
		return nil
	}

	nodeIDs := findNodesAtLine(pdg, line)
	nodes := make([]PDGNode, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		if node, ok := pdg.Nodes[id]; ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// RefType is a type alias for dfg.RefType to avoid import cycle
type RefType = dfg.RefType
