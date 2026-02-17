// Package dfg provides data flow analysis including reaching definitions.
package dfg

import (
	"container/list"
	"fmt"

	"github.com/l3aro/go-context-query/pkg/cfg"
)

// ReachingDefsAnalyzer performs reaching definitions analysis on a control flow graph.
// It uses a worklist-based algorithm to compute which definitions reach each point
// in the CFG, then builds def-use chains from these results.
type ReachingDefsAnalyzer struct {
	// blockGen maps block ID to set of definition IDs generated in that block
	blockGen map[string]map[int]struct{}
	// blockKill maps block ID to set of variable names killed in that block
	blockKill map[string]map[string]struct{}
	// defBlock maps definition ID to the block ID where it's defined
	defBlock map[int]string
	// defVar maps definition ID to the variable name
	defVar map[int]string
}

// NewReachingDefsAnalyzer creates a new ReachingDefsAnalyzer.
func NewReachingDefsAnalyzer() *ReachingDefsAnalyzer {
	return &ReachingDefsAnalyzer{
		blockGen:  make(map[string]map[int]struct{}),
		blockKill: make(map[string]map[string]struct{}),
		defBlock:  make(map[int]string),
		defVar:    make(map[int]string),
	}
}

// ComputeDefUseChains computes def-use chains using reaching definitions analysis.
// It takes a CFG and a list of variable references, then returns data flow edges
// connecting definitions to their uses.
func (r *ReachingDefsAnalyzer) ComputeDefUseChains(cfgInfo *cfg.CFGInfo, refs []VarRef) []DataflowEdge {
	if cfgInfo == nil || len(cfgInfo.Blocks) == 0 {
		return nil
	}

	// Initialize: build gen/kill sets and track definition locations
	r.initialize(cfgInfo, refs)

	// Build predecessor map for each block
	preds := r.buildPredecessors(cfgInfo)

	// Initialize in-sets for each block (entry block gets empty set)
	in := make(map[string]map[int]struct{})
	out := make(map[string]map[int]struct{})

	// Initialize all blocks
	for blockID := range cfgInfo.Blocks {
		in[blockID] = make(map[int]struct{})
		out[blockID] = make(map[int]struct{})
	}

	// Worklist algorithm
	worklist := list.New()
	for blockID := range cfgInfo.Blocks {
		worklist.PushBack(blockID)
	}

	for worklist.Len() > 0 {
		elem := worklist.Remove(worklist.Front()).(string)
		blockID := elem

		// Save old out set
		oldOut := r.copySet(out[blockID])

		// Compute in[block] = union of out[pred] for all predecessors
		in[blockID] = r.unionPreds(out, preds[blockID])

		// Compute out[block] = gen[block] U (in[block] - kill[block])
		out[blockID] = r.computeOut(in[blockID], blockID)

		// If out changed, add successors to worklist
		if !r.setsEqual(oldOut, out[blockID]) {
			for _, edge := range cfgInfo.Edges {
				if edge.SourceID == blockID {
					worklist.PushBack(edge.TargetID)
				}
			}
		}
	}

	// Build def-use chains from reaching definitions
	return r.buildDefUseChains(cfgInfo, refs, in, out)
}

// initialize builds gen/kill sets and tracks definition locations.
func (r *ReachingDefsAnalyzer) initialize(cfgInfo *cfg.CFGInfo, refs []VarRef) {
	r.blockGen = make(map[string]map[int]struct{})
	r.blockKill = make(map[string]map[string]struct{})
	r.defBlock = make(map[int]string)
	r.defVar = make(map[int]string)

	defID := 0

	// Group refs by block
	blockRefs := make(map[string][]VarRef)
	for _, ref := range refs {
		blockID := r.findBlockForLine(cfgInfo, ref.Line)
		if blockID != "" {
			blockRefs[blockID] = append(blockRefs[blockID], ref)
			if ref.RefType == RefTypeDefinition || ref.RefType == RefTypeUpdate {
				r.defBlock[defID] = blockID
				r.defVar[defID] = ref.Name
				defID++
			}
		}
	}

	// Build gen and kill sets for each block
	for blockID, block := range cfgInfo.Blocks {
		gen := make(map[int]struct{})
		kill := make(map[string]struct{})

		// Find definitions in this block
		for i, ref := range blockRefs[blockID] {
			if ref.RefType == RefTypeDefinition || ref.RefType == RefTypeUpdate {
				// Find the def ID for this reference
				for dID, dBlock := range r.defBlock {
					if dBlock == blockID && r.defVar[dID] == ref.Name {
						// Check if this is the same reference (by line/column)
						found := false
						for j, r := range refs {
							if r.Name == ref.Name && r.Line == ref.Line && r.Column == ref.Column {
								if j == i {
									gen[dID] = struct{}{}
									found = true
									break
								}
							}
						}
						if !found {
							gen[dID] = struct{}{}
						}
					}
				}
				// Kill all other definitions of the same variable in this block
				kill[ref.Name] = struct{}{}
			}
		}

		// Also kill based on statements in block (assignments)
		for _, stmt := range block.Statements {
			if varName := r.extractAssignedVar(stmt); varName != "" {
				kill[varName] = struct{}{}
			}
		}

		r.blockGen[blockID] = gen
		r.blockKill[blockID] = kill
	}
}

// findBlockForLine finds the block containing the given line number.
func (r *ReachingDefsAnalyzer) findBlockForLine(cfgInfo *cfg.CFGInfo, line int) string {
	for blockID, block := range cfgInfo.Blocks {
		if line >= block.StartLine && line <= block.EndLine {
			return blockID
		}
	}
	return ""
}

// extractAssignedVar extracts variable name from an assignment statement.
func (r *ReachingDefsAnalyzer) extractAssignedVar(stmt string) string {
	// Simple extraction: look for pattern "var = ..." or "var :="
	for i, ch := range stmt {
		if ch == '=' && i > 0 {
			// Look back for valid identifier start
			start := i - 1
			for start > 0 && (stmt[start-1] == ' ' || stmt[start-1] == '\t') {
				start--
			}
			if start > 0 {
				varName := ""
				for j := start; j > 0; j-- {
					c := stmt[j-1]
					if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
						varName = string(stmt[j-1:j]) + varName
					} else {
						break
					}
				}
				if varName != "" {
					return varName
				}
			}
		}
	}
	return ""
}

// buildPredecessors builds a map of block ID to its predecessors.
func (r *ReachingDefsAnalyzer) buildPredecessors(cfgInfo *cfg.CFGInfo) map[string][]string {
	preds := make(map[string][]string)
	for blockID := range cfgInfo.Blocks {
		preds[blockID] = nil
	}

	for _, edge := range cfgInfo.Edges {
		preds[edge.TargetID] = append(preds[edge.TargetID], edge.SourceID)
	}

	return preds
}

// copySet creates a copy of a definition set.
func (r *ReachingDefsAnalyzer) copySet(src map[int]struct{}) map[int]struct{} {
	dst := make(map[int]struct{})
	for k := range src {
		dst[k] = struct{}{}
	}
	return dst
}

// unionPreds computes the union of out sets for all predecessors.
func (r *ReachingDefsAnalyzer) unionPreds(out map[string]map[int]struct{}, preds []string) map[int]struct{} {
	result := make(map[int]struct{})

	for _, predID := range preds {
		for defID := range out[predID] {
			result[defID] = struct{}{}
		}
	}

	return result
}

// computeOut computes out[block] = gen[block] U (in[block] - kill[block]).
func (r *ReachingDefsAnalyzer) computeOut(inSet map[int]struct{}, blockID string) map[int]struct{} {
	outSet := make(map[int]struct{})

	// Start with gen set
	for defID := range r.blockGen[blockID] {
		outSet[defID] = struct{}{}
	}

	// Add in - kill
	for defID := range inSet {
		varName := r.defVar[defID]
		if _, killed := r.blockKill[blockID][varName]; !killed {
			outSet[defID] = struct{}{}
		}
	}

	return outSet
}

// setsEqual checks if two sets are equal.
func (r *ReachingDefsAnalyzer) setsEqual(a, b map[int]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// buildDefUseChains builds def-use chains from the computed in/out sets.
// It handles branch merging by considering definitions from all paths.
func (r *ReachingDefsAnalyzer) buildDefUseChains(cfgInfo *cfg.CFGInfo, refs []VarRef, in, out map[string]map[int]struct{}) []DataflowEdge {
	var edges []DataflowEdge

	// For each block, find uses and connect to reaching definitions
	for blockID, block := range cfgInfo.Blocks {
		// Get reaching definitions at entry to this block
		reaching := in[blockID]

		// Process each use in this block
		for _, ref := range refs {
			if ref.RefType != RefTypeUse {
				continue
			}

			// Check if this use is in the current block
			if !r.isRefInBlock(ref, block) {
				continue
			}

			// Find definitions that reach this use
			for defID := range reaching {
				defVarName := r.defVar[defID]
				if defVarName == ref.Name {
					// Find the actual definition reference
					defRef := r.findDefRef(refs, defID)
					if defRef != nil {
						edges = append(edges, DataflowEdge{
							DefRef:  *defRef,
							UseRef:  ref,
							VarName: ref.Name,
						})
					}
				}
			}

			// Also check definitions generated in this block before this use
			for defID := range r.blockGen[blockID] {
				defVarName := r.defVar[defID]
				if defVarName == ref.Name {
					// Check if definition comes before use in the block
					defRef := r.findDefRef(refs, defID)
					if defRef != nil && defRef.Line < ref.Line {
						edges = append(edges, DataflowEdge{
							DefRef:  *defRef,
							UseRef:  ref,
							VarName: ref.Name,
						})
					}
				}
			}
		}

		// Handle merge points (branches): definitions from both paths reach after merge
		if block.Type == cfg.BlockTypePlain || block.Type == cfg.BlockTypeLoopBody {
			// Check if this block has multiple predecessors (merge point)
			preds := r.getPredecessors(cfgInfo, blockID)
			if len(preds) > 1 {
				// Union of definitions from all predecessor blocks reach here
				mergedDefs := make(map[int]struct{})
				for _, predID := range preds {
					for defID := range out[predID] {
						mergedDefs[defID] = struct{}{}
					}
				}

				// Connect merged definitions to uses in this block
				for _, ref := range refs {
					if ref.RefType != RefTypeUse {
						continue
					}
					if !r.isRefInBlock(ref, block) {
						continue
					}

					for defID := range mergedDefs {
						defVarName := r.defVar[defID]
						if defVarName == ref.Name {
							defRef := r.findDefRef(refs, defID)
							if defRef != nil {
								edges = append(edges, DataflowEdge{
									DefRef:  *defRef,
									UseRef:  ref,
									VarName: ref.Name,
								})
							}
						}
					}
				}
			}
		}
	}

	// Remove duplicate edges
	return r.deduplicateEdges(edges)
}

// getPredecessors returns the predecessor block IDs for a given block.
func (r *ReachingDefsAnalyzer) getPredecessors(cfgInfo *cfg.CFGInfo, blockID string) []string {
	var preds []string
	for _, edge := range cfgInfo.Edges {
		if edge.TargetID == blockID {
			preds = append(preds, edge.SourceID)
		}
	}
	return preds
}

// isRefInBlock checks if a reference is within a block's line range.
func (r *ReachingDefsAnalyzer) isRefInBlock(ref VarRef, block cfg.CFGBlock) bool {
	return ref.Line >= block.StartLine && ref.Line <= block.EndLine
}

// findDefRef finds the VarRef for a given definition ID.
func (r *ReachingDefsAnalyzer) findDefRef(refs []VarRef, defID int) *VarRef {
	idx := 0
	for i, ref := range refs {
		if ref.RefType == RefTypeDefinition || ref.RefType == RefTypeUpdate {
			if idx == defID {
				result := refs[i]
				return &result
			}
			idx++
		}
	}
	return nil
}

// deduplicateEdges removes duplicate edges from the list.
func (r *ReachingDefsAnalyzer) deduplicateEdges(edges []DataflowEdge) []DataflowEdge {
	seen := make(map[string]bool)
	var result []DataflowEdge

	for _, edge := range edges {
		key := fmt.Sprintf("%d:%d->%d:%d", edge.DefRef.Line, edge.DefRef.Column, edge.UseRef.Line, edge.UseRef.Column)
		if !seen[key] {
			seen[key] = true
			result = append(result, edge)
		}
	}

	return result
}
