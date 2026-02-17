// Package pdg provides functionality for building Program Dependence Graphs (PDGs)
// by merging Control Flow Graph (CFG) and Data Flow Graph (DFG) information.
package pdg

import (
	"github.com/l3aro/go-context-query/pkg/cfg"
	"github.com/l3aro/go-context-query/pkg/dfg"
)

// PDGBuilder builds a Program Dependence Graph by merging CFG and DFG information.
type PDGBuilder struct {
	cfg *cfg.CFGInfo
	dfg *dfg.DFGInfo

	// lineToNodeID maps source line numbers to PDG node IDs
	lineToNodeID map[int]string
}

// NewPDGBuilder creates a new PDGBuilder with the given CFG and DFG information.
func NewPDGBuilder(cfgInfo *cfg.CFGInfo, dfgInfo *dfg.DFGInfo) *PDGBuilder {
	return &PDGBuilder{
		cfg:          cfgInfo,
		dfg:          dfgInfo,
		lineToNodeID: make(map[int]string),
	}
}

// Build constructs the complete PDG by merging CFG and DFG information.
// It creates nodes from CFG blocks, adds control edges from CFG,
// and adds data edges from DFG (avoiding self-loops).
func (b *PDGBuilder) Build() *PDGInfo {
	if b.cfg == nil {
		return &PDGInfo{
			Nodes: make(map[string]PDGNode),
			Edges: make([]PDGEdge, 0),
		}
	}

	pdgInfo := &PDGInfo{
		FunctionName: b.cfg.FunctionName,
		CFG:          b.cfg,
		DFG:          b.dfg,
		Nodes:        make(map[string]PDGNode),
		Edges:        make([]PDGEdge, 0),
	}

	// Step 1: Create nodes from CFG blocks
	b.createNodesFromCFG(pdgInfo)

	// Step 2: Add control edges from CFG
	b.addControlEdges(pdgInfo)

	// Step 3: Add data edges from DFG (avoiding self-loops)
	b.addDataEdges(pdgInfo)

	return pdgInfo
}

// createNodesFromCFG creates PDG nodes from CFG blocks and builds line-to-node mapping.
func (b *PDGBuilder) createNodesFromCFG(pdgInfo *PDGInfo) {
	for _, block := range b.cfg.Blocks {
		nodeType := b.mapBlockTypeToNodeType(block.Type)

		node := PDGNode{
			ID:         block.ID,
			Type:       nodeType,
			StartLine:  block.StartLine,
			EndLine:    block.EndLine,
			CFGBlockID: block.ID,
		}

		// If we have DFG info, attach definitions and uses to the node
		if b.dfg != nil {
			node.Definitions = b.collectDefinitionsForBlock(block)
			node.Uses = b.collectUsesForBlock(block)
		}

		pdgInfo.Nodes[block.ID] = node

		// Build line-to-node mapping for each line in the block
		for line := block.StartLine; line <= block.EndLine; line++ {
			b.lineToNodeID[line] = block.ID
		}
	}
}

// mapBlockTypeToNodeType maps CFG BlockType to PDG NodeType.
func (b *PDGBuilder) mapBlockTypeToNodeType(blockType cfg.BlockType) NodeType {
	switch blockType {
	case cfg.BlockTypeEntry:
		return NodeTypeEntry
	case cfg.BlockTypeBranch:
		return NodeTypeBranch
	case cfg.BlockTypeLoopBody:
		return NodeTypeLoop
	case cfg.BlockTypeReturn:
		return NodeTypeStatement
	case cfg.BlockTypeExit:
		return NodeTypeExit
	default:
		return NodeTypeStatement
	}
}

// collectDefinitionsForBlock collects variable definitions within a CFG block.
func (b *PDGBuilder) collectDefinitionsForBlock(block cfg.CFGBlock) []dfg.VarRef {
	if b.dfg == nil {
		return nil
	}

	var defs []dfg.VarRef
	for _, vref := range b.dfg.VarRefs {
		if vref.Line >= block.StartLine && vref.Line <= block.EndLine {
			if vref.RefType == dfg.RefTypeDefinition || vref.RefType == dfg.RefTypeUpdate {
				defs = append(defs, vref)
			}
		}
	}
	return defs
}

// collectUsesForBlock collects variable uses within a CFG block.
func (b *PDGBuilder) collectUsesForBlock(block cfg.CFGBlock) []dfg.VarRef {
	if b.dfg == nil {
		return nil
	}

	var uses []dfg.VarRef
	for _, vref := range b.dfg.VarRefs {
		if vref.Line >= block.StartLine && vref.Line <= block.EndLine {
			if vref.RefType == dfg.RefTypeUse {
				uses = append(uses, vref)
			}
		}
	}
	return uses
}

// addControlEdges adds control dependence edges from CFG edges.
func (b *PDGBuilder) addControlEdges(pdgInfo *PDGInfo) {
	for _, edge := range b.cfg.Edges {
		// Only add edges where both source and target nodes exist
		if _, srcExists := pdgInfo.Nodes[edge.SourceID]; !srcExists {
			continue
		}
		if _, tgtExists := pdgInfo.Nodes[edge.TargetID]; !tgtExists {
			continue
		}

		pdgEdge := PDGEdge{
			SourceID: edge.SourceID,
			TargetID: edge.TargetID,
			DepType:  DepTypeControl,
			Label:    string(edge.EdgeType),
		}
		pdgInfo.Edges = append(pdgInfo.Edges, pdgEdge)
	}
}

// addDataEdges adds data dependence edges from DFG dataflow edges.
// It avoids self-loops where source and target nodes are the same.
func (b *PDGBuilder) addDataEdges(pdgInfo *PDGInfo) {
	if b.dfg == nil {
		return
	}

	for _, dataEdge := range b.dfg.DataflowEdges {
		// Get node IDs for the definition and use lines
		defNodeID := b.lineToNodeID[dataEdge.DefRef.Line]
		useNodeID := b.lineToNodeID[dataEdge.UseRef.Line]

		// Skip if we can't find the nodes
		if defNodeID == "" || useNodeID == "" {
			continue
		}

		// Skip self-loops: when the source and target nodes are the same
		if defNodeID == useNodeID {
			continue
		}

		// Only add edges where both source and target nodes exist in PDG
		if _, srcExists := pdgInfo.Nodes[defNodeID]; !srcExists {
			continue
		}
		if _, tgtExists := pdgInfo.Nodes[useNodeID]; !tgtExists {
			continue
		}

		pdgEdge := PDGEdge{
			SourceID: defNodeID,
			TargetID: useNodeID,
			DepType:  DepTypeData,
			Label:    dataEdge.VarName,
		}
		pdgInfo.Edges = append(pdgInfo.Edges, pdgEdge)
	}
}
