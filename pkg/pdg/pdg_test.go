package pdg

import (
	"encoding/json"
	"testing"

	"github.com/l3aro/go-context-query/pkg/cfg"
	"github.com/l3aro/go-context-query/pkg/dfg"
)

// TestPDGNodeSerialization tests JSON marshal/unmarshal of PDGNode
func TestPDGNodeSerialization(t *testing.T) {
	tests := []struct {
		name     string
		node     PDGNode
		expected string
	}{
		{
			name: "basic statement node",
			node: PDGNode{
				ID:         "block_1",
				Type:       NodeTypeStatement,
				StartLine:  1,
				EndLine:    1,
				CFGBlockID: "block_1",
			},
			expected: `{"id":"block_1","type":"statement","start_line":1,"end_line":1,"definitions":null,"uses":null,"cfg_block_id":"block_1"}`,
		},
		{
			name: "branch node",
			node: PDGNode{
				ID:         "branch_1",
				Type:       NodeTypeBranch,
				StartLine:  5,
				EndLine:    7,
				CFGBlockID: "branch_1",
			},
			expected: `{"id":"branch_1","type":"branch","start_line":5,"end_line":7,"definitions":null,"uses":null,"cfg_block_id":"branch_1"}`,
		},
		{
			name: "entry node",
			node: PDGNode{
				ID:         "entry",
				Type:       NodeTypeEntry,
				StartLine:  1,
				EndLine:    1,
				CFGBlockID: "entry",
			},
			expected: `{"id":"entry","type":"entry","start_line":1,"end_line":1,"definitions":null,"uses":null,"cfg_block_id":"entry"}`,
		},
		{
			name: "exit node",
			node: PDGNode{
				ID:         "exit",
				Type:       NodeTypeExit,
				StartLine:  10,
				EndLine:    10,
				CFGBlockID: "exit",
			},
			expected: `{"id":"exit","type":"exit","start_line":10,"end_line":10,"definitions":null,"uses":null,"cfg_block_id":"exit"}`,
		},
		{
			name: "branch node",
			node: PDGNode{
				ID:         "branch_1",
				Type:       NodeTypeBranch,
				StartLine:  5,
				EndLine:    7,
				CFGBlockID: "branch_1",
			},
			expected: `{"id":"branch_1","type":"branch","start_line":5,"end_line":7,"definitions":null,"uses":null,"cfg_block_id":"branch_1"}`,
		},
		{
			name: "node with definitions and uses",
			node: PDGNode{
				ID:        "block_2",
				Type:      NodeTypeStatement,
				StartLine: 2,
				EndLine:   3,
				Definitions: []dfg.VarRef{
					{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
				},
				Uses: []dfg.VarRef{
					{Name: "y", RefType: dfg.RefTypeUse, Line: 3, Column: 4},
				},
				CFGBlockID: "block_2",
			},
			expected: `{"id":"block_2","type":"statement","start_line":2,"end_line":3,"definitions":[{"name":"x","ref_type":"definition","line":2,"column":0}],"uses":[{"name":"y","ref_type":"use","line":3,"column":4}],"cfg_block_id":"block_2"}`,
		},
		{
			name: "entry node",
			node: PDGNode{
				ID:         "entry",
				Type:       NodeTypeEntry,
				StartLine:  1,
				EndLine:    1,
				CFGBlockID: "entry",
			},
			expected: `{"id":"entry","type":"entry","start_line":1,"end_line":1,"definitions":null,"uses":null,"cfg_block_id":"entry"}`,
		},
		{
			name: "exit node",
			node: PDGNode{
				ID:         "exit",
				Type:       NodeTypeExit,
				StartLine:  10,
				EndLine:    10,
				CFGBlockID: "exit",
			},
			expected: `{"id":"exit","type":"exit","start_line":10,"end_line":10,"definitions":null,"uses":null,"cfg_block_id":"exit"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.node)
			if err != nil {
				t.Fatalf("Marshal() failed: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Marshal() = %s, want %s", string(data), tt.expected)
			}

			// Unmarshal
			var decoded PDGNode
			err = json.Unmarshal([]byte(tt.expected), &decoded)
			if err != nil {
				t.Fatalf("Unmarshal() failed: %v", err)
			}

			if decoded.ID != tt.node.ID {
				t.Errorf("Unmarshal().ID = %s, want %s", decoded.ID, tt.node.ID)
			}
			if decoded.Type != tt.node.Type {
				t.Errorf("Unmarshal().Type = %s, want %s", decoded.Type, tt.node.Type)
			}
			if decoded.StartLine != tt.node.StartLine {
				t.Errorf("Unmarshal().StartLine = %d, want %d", decoded.StartLine, tt.node.StartLine)
			}
			if decoded.EndLine != tt.node.EndLine {
				t.Errorf("Unmarshal().EndLine = %d, want %d", decoded.EndLine, tt.node.EndLine)
			}
		})
	}
}

// TestPDGEdgeSerialization tests JSON marshal/unmarshal of PDGEdge
func TestPDGEdgeSerialization(t *testing.T) {
	tests := []struct {
		name     string
		edge     PDGEdge
		expected string
	}{
		{
			name: "control edge",
			edge: PDGEdge{
				SourceID: "block_1",
				TargetID: "block_2",
				DepType:  DepTypeControl,
				Label:    "unconditional",
			},
			expected: `{"source_id":"block_1","target_id":"block_2","dep_type":"control","label":"unconditional"}`,
		},
		{
			name: "data edge",
			edge: PDGEdge{
				SourceID: "block_1",
				TargetID: "block_2",
				DepType:  DepTypeData,
				Label:    "x",
			},
			expected: `{"source_id":"block_1","target_id":"block_2","dep_type":"data","label":"x"}`,
		},
		{
			name: "edge with empty label",
			edge: PDGEdge{
				SourceID: "entry",
				TargetID: "block_1",
				DepType:  DepTypeControl,
				Label:    "",
			},
			expected: `{"source_id":"entry","target_id":"block_1","dep_type":"control","label":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.edge)
			if err != nil {
				t.Fatalf("Marshal() failed: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Marshal() = %s, want %s", string(data), tt.expected)
			}

			// Unmarshal
			var decoded PDGEdge
			err = json.Unmarshal([]byte(tt.expected), &decoded)
			if err != nil {
				t.Fatalf("Unmarshal() failed: %v", err)
			}

			if decoded.SourceID != tt.edge.SourceID {
				t.Errorf("Unmarshal().SourceID = %s, want %s", decoded.SourceID, tt.edge.SourceID)
			}
			if decoded.TargetID != tt.edge.TargetID {
				t.Errorf("Unmarshal().TargetID = %s, want %s", decoded.TargetID, tt.edge.TargetID)
			}
			if decoded.DepType != tt.edge.DepType {
				t.Errorf("Unmarshal().DepType = %s, want %s", decoded.DepType, tt.edge.DepType)
			}
			if decoded.Label != tt.edge.Label {
				t.Errorf("Unmarshal().Label = %s, want %s", decoded.Label, tt.edge.Label)
			}
		})
	}
}

// TestPDGBuilderBasic tests PDGBuilder with simple mock CFG/DFG
func TestPDGBuilderBasic(t *testing.T) {
	// Create mock CFG with 3 blocks: entry -> body -> exit
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "test_function",
		Blocks: map[string]cfg.CFGBlock{
			"entry": {
				ID:         "entry",
				Type:       cfg.BlockTypeEntry,
				StartLine:  1,
				EndLine:    1,
				Statements: []string{},
			},
			"body": {
				ID:         "body",
				Type:       cfg.BlockTypePlain,
				StartLine:  2,
				EndLine:    4,
				Statements: []string{"x := 1", "y := x + 1"},
			},
			"exit": {
				ID:         "exit",
				Type:       cfg.BlockTypeExit,
				StartLine:  5,
				EndLine:    5,
				Statements: []string{"return y"},
			},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "body", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "body", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	// Create mock DFG with variable x and y
	dfgInfo := &dfg.DFGInfo{
		FunctionName: "test_function",
		VarRefs: []dfg.VarRef{
			{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
			{Name: "y", RefType: dfg.RefTypeDefinition, Line: 3, Column: 0},
			{Name: "x", RefType: dfg.RefTypeUse, Line: 3, Column: 4},
			{Name: "y", RefType: dfg.RefTypeUse, Line: 5, Column: 7},
		},
		DataflowEdges: []dfg.DataflowEdge{
			{
				DefRef:  dfg.VarRef{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
				UseRef:  dfg.VarRef{Name: "x", RefType: dfg.RefTypeUse, Line: 3, Column: 4},
				VarName: "x",
			},
			{
				DefRef:  dfg.VarRef{Name: "y", RefType: dfg.RefTypeDefinition, Line: 3, Column: 0},
				UseRef:  dfg.VarRef{Name: "y", RefType: dfg.RefTypeUse, Line: 5, Column: 7},
				VarName: "y",
			},
		},
		Variables: map[string][]dfg.VarRef{
			"x": {
				{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
				{Name: "x", RefType: dfg.RefTypeUse, Line: 3, Column: 4},
			},
			"y": {
				{Name: "y", RefType: dfg.RefTypeDefinition, Line: 3, Column: 0},
				{Name: "y", RefType: dfg.RefTypeUse, Line: 5, Column: 7},
			},
		},
	}

	// Build PDG
	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	pdg := builder.Build()

	// Verify PDG structure
	if pdg.FunctionName != "test_function" {
		t.Errorf("FunctionName = %s, want test_function", pdg.FunctionName)
	}

	// Verify nodes were created
	if len(pdg.Nodes) != 3 {
		t.Errorf("len(Nodes) = %d, want 3", len(pdg.Nodes))
	}

	// Verify entry node
	entryNode, ok := pdg.Nodes["entry"]
	if !ok {
		t.Fatal("entry node not found")
	}
	if entryNode.Type != NodeTypeEntry {
		t.Errorf("entry node type = %s, want %s", entryNode.Type, NodeTypeEntry)
	}
	if entryNode.StartLine != 1 || entryNode.EndLine != 1 {
		t.Errorf("entry node lines = (%d, %d), want (1, 1)", entryNode.StartLine, entryNode.EndLine)
	}

	// Verify body node
	bodyNode, ok := pdg.Nodes["body"]
	if !ok {
		t.Fatal("body node not found")
	}
	if bodyNode.Type != NodeTypeStatement {
		t.Errorf("body node type = %s, want %s", bodyNode.Type, NodeTypeStatement)
	}
	if bodyNode.StartLine != 2 || bodyNode.EndLine != 4 {
		t.Errorf("body node lines = (%d, %d), want (2, 4)", bodyNode.StartLine, bodyNode.EndLine)
	}

	// Verify body node has definitions and uses
	if len(bodyNode.Definitions) != 2 {
		t.Errorf("body node has %d definitions, want 2", len(bodyNode.Definitions))
	}
	if len(bodyNode.Uses) != 1 {
		t.Errorf("body node has %d uses, want 1", len(bodyNode.Uses))
	}

	// Verify exit node
	exitNode, ok := pdg.Nodes["exit"]
	if !ok {
		t.Fatal("exit node not found")
	}
	if exitNode.Type != NodeTypeExit {
		t.Errorf("exit node type = %s, want %s", exitNode.Type, NodeTypeExit)
	}

	// Verify edges were created
	if len(pdg.Edges) != 3 {
		t.Errorf("len(Edges) = %d, want 3", len(pdg.Edges))
	}

	// Count control and data edges
	var controlEdges, dataEdges int
	for _, edge := range pdg.Edges {
		if edge.DepType == DepTypeControl {
			controlEdges++
		} else if edge.DepType == DepTypeData {
			dataEdges++
		}
	}

	if controlEdges != 2 {
		t.Errorf("control edges = %d, want 2", controlEdges)
	}
	if dataEdges != 1 {
		t.Errorf("data edges = %d, want 1", dataEdges)
	}
}

// TestBackwardSlice tests backward slice returns correct lines
func TestBackwardSlice(t *testing.T) {
	// Create a simple PDG for testing
	// Entry -> Block1 (x := 1) -> Block2 (y := x + 1) -> Exit
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "test_slice",
		Blocks: map[string]cfg.CFGBlock{
			"entry":  {ID: "entry", Type: cfg.BlockTypeEntry, StartLine: 1, EndLine: 1},
			"block1": {ID: "block1", Type: cfg.BlockTypePlain, StartLine: 2, EndLine: 2},
			"block2": {ID: "block2", Type: cfg.BlockTypePlain, StartLine: 3, EndLine: 3},
			"exit":   {ID: "exit", Type: cfg.BlockTypeExit, StartLine: 4, EndLine: 4},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "block1", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block1", TargetID: "block2", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block2", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	dfgInfo := &dfg.DFGInfo{
		FunctionName: "test_slice",
		VarRefs: []dfg.VarRef{
			{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
			{Name: "y", RefType: dfg.RefTypeDefinition, Line: 3, Column: 0},
			{Name: "x", RefType: dfg.RefTypeUse, Line: 3, Column: 6},
		},
		DataflowEdges: []dfg.DataflowEdge{
			{
				DefRef:  dfg.VarRef{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
				UseRef:  dfg.VarRef{Name: "x", RefType: dfg.RefTypeUse, Line: 3, Column: 6},
				VarName: "x",
			},
		},
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	pdg := builder.Build()

	// Test backward slice from line 3 (y := x + 1)
	// Should include line 2 (x := 1) and line 3 through control flow
	result := BackwardSlice(pdg, 3, nil)
	if len(result) == 0 {
		t.Fatal("BackwardSlice() returned empty result")
	}

	// Should include at least lines 2 and 3 (block1 and block2)
	hasLine2 := false
	hasLine3 := false
	for _, line := range result {
		if line == 2 {
			hasLine2 = true
		}
		if line == 3 {
			hasLine3 = true
		}
	}

	if !hasLine2 {
		t.Error("BackwardSlice() should include line 2 (x := 1)")
	}
	if !hasLine3 {
		t.Error("BackwardSlice() should include line 3 (current line)")
	}
}

// TestForwardSlice tests forward slice returns correct lines
func TestForwardSlice(t *testing.T) {
	// Create a simple PDG for testing
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "test_forward_slice",
		Blocks: map[string]cfg.CFGBlock{
			"entry":  {ID: "entry", Type: cfg.BlockTypeEntry, StartLine: 1, EndLine: 1},
			"block1": {ID: "block1", Type: cfg.BlockTypePlain, StartLine: 2, EndLine: 2},
			"block2": {ID: "block2", Type: cfg.BlockTypePlain, StartLine: 3, EndLine: 3},
			"exit":   {ID: "exit", Type: cfg.BlockTypeExit, StartLine: 4, EndLine: 4},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "block1", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block1", TargetID: "block2", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block2", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	dfgInfo := &dfg.DFGInfo{
		FunctionName: "test_forward_slice",
		VarRefs: []dfg.VarRef{
			{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
			{Name: "y", RefType: dfg.RefTypeDefinition, Line: 3, Column: 0},
			{Name: "x", RefType: dfg.RefTypeUse, Line: 3, Column: 6},
		},
		DataflowEdges: []dfg.DataflowEdge{
			{
				DefRef:  dfg.VarRef{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
				UseRef:  dfg.VarRef{Name: "x", RefType: dfg.RefTypeUse, Line: 3, Column: 6},
				VarName: "x",
			},
		},
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	pdg := builder.Build()

	// Test forward slice from line 2 (x := 1)
	result := ForwardSlice(pdg, 2, nil)
	if len(result) == 0 {
		t.Fatal("ForwardSlice() returned empty result")
	}

	// Should include line 2 (current) and line 3 (dependent)
	hasLine2 := false
	hasLine3 := false
	for _, line := range result {
		if line == 2 {
			hasLine2 = true
		}
		if line == 3 {
			hasLine3 = true
		}
	}

	if !hasLine2 {
		t.Error("ForwardSlice() should include line 2 (current line)")
	}
	if !hasLine3 {
		t.Error("ForwardSlice() should include line 3 (dependent line)")
	}
}

// TestVariableFilter tests variable filtering works
func TestVariableFilter(t *testing.T) {
	// Create a PDG with multiple variables
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "test_var_filter",
		Blocks: map[string]cfg.CFGBlock{
			"entry":  {ID: "entry", Type: cfg.BlockTypeEntry, StartLine: 1, EndLine: 1},
			"block1": {ID: "block1", Type: cfg.BlockTypePlain, StartLine: 2, EndLine: 2},
			"block2": {ID: "block2", Type: cfg.BlockTypePlain, StartLine: 3, EndLine: 3},
			"block3": {ID: "block3", Type: cfg.BlockTypePlain, StartLine: 4, EndLine: 4},
			"exit":   {ID: "exit", Type: cfg.BlockTypeExit, StartLine: 5, EndLine: 5},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "block1", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block1", TargetID: "block2", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block2", TargetID: "block3", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block3", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	// x := 1 (line 2)
	// y := 2 (line 3)
	// z := x + y (line 4)
	dfgInfo := &dfg.DFGInfo{
		FunctionName: "test_var_filter",
		VarRefs: []dfg.VarRef{
			{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
			{Name: "y", RefType: dfg.RefTypeDefinition, Line: 3, Column: 0},
			{Name: "z", RefType: dfg.RefTypeDefinition, Line: 4, Column: 0},
			{Name: "x", RefType: dfg.RefTypeUse, Line: 4, Column: 6},
			{Name: "y", RefType: dfg.RefTypeUse, Line: 4, Column: 10},
		},
		DataflowEdges: []dfg.DataflowEdge{
			{
				DefRef:  dfg.VarRef{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
				UseRef:  dfg.VarRef{Name: "x", RefType: dfg.RefTypeUse, Line: 4, Column: 6},
				VarName: "x",
			},
			{
				DefRef:  dfg.VarRef{Name: "y", RefType: dfg.RefTypeDefinition, Line: 3, Column: 0},
				UseRef:  dfg.VarRef{Name: "y", RefType: dfg.RefTypeUse, Line: 4, Column: 10},
				VarName: "y",
			},
		},
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	pdg := builder.Build()

	// Test backward slice with variable filter for "x"
	var xVar string = "x"
	result := BackwardSlice(pdg, 4, &xVar)
	if len(result) == 0 {
		t.Fatal("BackwardSlice() with variable filter returned empty result")
	}

	hasLine2 := false
	hasLine4 := false
	for _, line := range result {
		if line == 2 {
			hasLine2 = true
		}
		if line == 4 {
			hasLine4 = true
		}
	}

	if !hasLine2 {
		t.Error("BackwardSlice() with filter 'x' should include line 2 (x definition)")
	}
	if !hasLine4 {
		t.Error("BackwardSlice() with filter 'x' should include line 4 (target)")
	}
}

// TestEmptyFunction tests edge case: empty function
func TestEmptyFunction(t *testing.T) {
	// Create a CFG with only entry and exit
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "empty_function",
		Blocks: map[string]cfg.CFGBlock{
			"entry": {ID: "entry", Type: cfg.BlockTypeEntry, StartLine: 1, EndLine: 1},
			"exit":  {ID: "exit", Type: cfg.BlockTypeExit, StartLine: 2, EndLine: 2},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	builder := NewPDGBuilder(cfgInfo, nil)
	pdg := builder.Build()

	if pdg.FunctionName != "empty_function" {
		t.Errorf("FunctionName = %s, want empty_function", pdg.FunctionName)
	}

	if len(pdg.Nodes) != 2 {
		t.Errorf("len(Nodes) = %d, want 2", len(pdg.Nodes))
	}

	if len(pdg.Edges) != 1 {
		t.Errorf("len(Edges) = %d, want 1", len(pdg.Edges))
	}

	result := BackwardSlice(pdg, 1, nil)
	if len(result) == 0 {
		t.Error("BackwardSlice() on entry should return at least the entry line")
	}

	result = ForwardSlice(pdg, 1, nil)
	if len(result) != 2 {
		t.Errorf("ForwardSlice() from entry = %d lines, want 2", len(result))
	}
}

// TestFunctionWithOnlyReturn tests edge case: function with only return
func TestFunctionWithOnlyReturn(t *testing.T) {
	// Create a CFG with entry, return, exit
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "only_return",
		Blocks: map[string]cfg.CFGBlock{
			"entry":  {ID: "entry", Type: cfg.BlockTypeEntry, StartLine: 1, EndLine: 1},
			"return": {ID: "return", Type: cfg.BlockTypeReturn, StartLine: 2, EndLine: 2},
			"exit":   {ID: "exit", Type: cfg.BlockTypeExit, StartLine: 3, EndLine: 3},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "return", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "return", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	dfgInfo := &dfg.DFGInfo{
		FunctionName: "only_return",
		VarRefs: []dfg.VarRef{
			{Name: "result", RefType: dfg.RefTypeDefinition, Line: 2, Column: 7},
			{Name: "result", RefType: dfg.RefTypeUse, Line: 2, Column: 7},
		},
		DataflowEdges: []dfg.DataflowEdge{
			{
				DefRef:  dfg.VarRef{Name: "result", RefType: dfg.RefTypeDefinition, Line: 2, Column: 7},
				UseRef:  dfg.VarRef{Name: "result", RefType: dfg.RefTypeUse, Line: 2, Column: 7},
				VarName: "result",
			},
		},
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	pdg := builder.Build()

	// Should have entry, return, exit nodes
	if len(pdg.Nodes) != 3 {
		t.Errorf("len(Nodes) = %d, want 3", len(pdg.Nodes))
	}

	// Verify return node type
	returnNode, ok := pdg.Nodes["return"]
	if !ok {
		t.Fatal("return node not found")
	}
	if returnNode.Type != NodeTypeStatement {
		t.Errorf("return node type = %s, want %s", returnNode.Type, NodeTypeStatement)
	}

	// Verify edges
	if len(pdg.Edges) != 2 { // 1 control + 1 data (self-loop skipped)
		t.Errorf("len(Edges) = %d, want 2", len(pdg.Edges))
	}

	// Backward slice should work
	result := BackwardSlice(pdg, 2, nil)
	if len(result) == 0 {
		t.Error("BackwardSlice() on return line should not be empty")
	}

	// Forward slice should work
	result = ForwardSlice(pdg, 1, nil)
	if len(result) == 0 {
		t.Error("ForwardSlice() on entry should not be empty")
	}
}

// TestPDGBuilderWithNilCFG tests PDGBuilder with nil CFG
func TestPDGBuilderWithNilCFG(t *testing.T) {
	builder := NewPDGBuilder(nil, nil)
	pdg := builder.Build()

	if pdg.Nodes == nil {
		t.Error("Nodes should not be nil")
	}
	if len(pdg.Nodes) != 0 {
		t.Errorf("len(Nodes) = %d, want 0", len(pdg.Nodes))
	}
	if pdg.Edges == nil {
		t.Error("Edges should not be nil")
	}
	if len(pdg.Edges) != 0 {
		t.Errorf("len(Edges) = %d, want 0", len(pdg.Edges))
	}
}

// TestBackwardSliceNilPDG tests BackwardSlice with nil PDG
func TestBackwardSliceNilPDG(t *testing.T) {
	result := BackwardSlice(nil, 1, nil)
	if result != nil {
		t.Errorf("BackwardSlice(nil) = %v, want nil", result)
	}
}

// TestForwardSliceNilPDG tests ForwardSlice with nil PDG
func TestForwardSliceNilPDG(t *testing.T) {
	result := ForwardSlice(nil, 1, nil)
	if result != nil {
		t.Errorf("ForwardSlice(nil) = %v, want nil", result)
	}
}

// TestGetDependencies tests GetDependencies function
func TestGetDependencies(t *testing.T) {
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "test_deps",
		Blocks: map[string]cfg.CFGBlock{
			"entry":  {ID: "entry", Type: cfg.BlockTypeEntry, StartLine: 1, EndLine: 1},
			"block1": {ID: "block1", Type: cfg.BlockTypePlain, StartLine: 2, EndLine: 2},
			"exit":   {ID: "exit", Type: cfg.BlockTypeExit, StartLine: 3, EndLine: 3},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "block1", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block1", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	dfgInfo := &dfg.DFGInfo{
		FunctionName: "test_deps",
		VarRefs: []dfg.VarRef{
			{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
		},
		DataflowEdges: []dfg.DataflowEdge{},
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	pdg := builder.Build()

	// Get dependencies for line 2
	deps := GetDependencies(pdg, 2)

	// Should have control in (from entry) and control out (to exit)
	if len(deps.ControlIn) == 0 {
		t.Error("Should have control in edges")
	}
	if len(deps.ControlOut) == 0 {
		t.Error("Should have control out edges")
	}
}

// TestGetDependenciesNilPDG tests GetDependencies with nil PDG
func TestGetDependenciesNilPDG(t *testing.T) {
	deps := GetDependencies(nil, 1)
	if len(deps.ControlIn) != 0 || len(deps.ControlOut) != 0 ||
		len(deps.DataIn) != 0 || len(deps.DataOut) != 0 {
		t.Error("GetDependencies(nil) should return empty DependencyInfo")
	}
}

// TestGetVariableNames tests GetVariableNames function
func TestGetVariableNames(t *testing.T) {
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "test_vars",
		Blocks: map[string]cfg.CFGBlock{
			"entry":  {ID: "entry", Type: cfg.BlockTypeEntry, StartLine: 1, EndLine: 1},
			"block1": {ID: "block1", Type: cfg.BlockTypePlain, StartLine: 2, EndLine: 2},
			"block2": {ID: "block2", Type: cfg.BlockTypePlain, StartLine: 3, EndLine: 3},
			"exit":   {ID: "exit", Type: cfg.BlockTypeExit, StartLine: 4, EndLine: 4},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "block1", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block1", TargetID: "block2", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block2", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	dfgInfo := &dfg.DFGInfo{
		FunctionName: "test_vars",
		VarRefs: []dfg.VarRef{
			{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
			{Name: "y", RefType: dfg.RefTypeDefinition, Line: 3, Column: 0},
		},
		DataflowEdges: []dfg.DataflowEdge{
			{
				DefRef:  dfg.VarRef{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
				UseRef:  dfg.VarRef{Name: "x", RefType: dfg.RefTypeUse, Line: 3, Column: 0},
				VarName: "x",
			},
		},
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	pdg := builder.Build()

	vars := GetVariableNames(pdg)
	if len(vars) != 1 {
		t.Errorf("GetVariableNames() = %v, want ['x']", vars)
	}
	if len(vars) > 0 && vars[0] != "x" {
		t.Errorf("GetVariableNames() = %v, want ['x']", vars)
	}
}

// TestFindNodesByVariable tests FindNodesByVariable function
func TestFindNodesByVariable(t *testing.T) {
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "test_find",
		Blocks: map[string]cfg.CFGBlock{
			"entry": {ID: "entry", Type: cfg.BlockTypeEntry, StartLine: 1, EndLine: 1},
			"block": {ID: "block", Type: cfg.BlockTypePlain, StartLine: 2, EndLine: 2},
			"exit":  {ID: "exit", Type: cfg.BlockTypeExit, StartLine: 3, EndLine: 3},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "block", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	dfgInfo := &dfg.DFGInfo{
		FunctionName: "test_find",
		VarRefs: []dfg.VarRef{
			{Name: "x", RefType: dfg.RefTypeDefinition, Line: 2, Column: 0},
		},
		DataflowEdges: []dfg.DataflowEdge{},
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	pdg := builder.Build()

	nodes := FindNodesByVariable(pdg, "x")
	if len(nodes) == 0 {
		t.Error("FindNodesByVariable('x') should return at least one node")
	}

	// Should not find non-existent variable
	nodes = FindNodesByVariable(pdg, "nonexistent")
	if len(nodes) != 0 {
		t.Error("FindNodesByVariable('nonexistent') should return empty slice")
	}
}

// TestGetNodeAtLine tests GetNodeAtLine function
func TestGetNodeAtLine(t *testing.T) {
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "test_node_at_line",
		Blocks: map[string]cfg.CFGBlock{
			"entry": {ID: "entry", Type: cfg.BlockTypeEntry, StartLine: 1, EndLine: 1},
			"block": {ID: "block", Type: cfg.BlockTypePlain, StartLine: 2, EndLine: 3},
			"exit":  {ID: "exit", Type: cfg.BlockTypeExit, StartLine: 4, EndLine: 4},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "block", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	builder := NewPDGBuilder(cfgInfo, nil)
	pdg := builder.Build()

	// Get node at line 2 (within block)
	node := GetNodeAtLine(pdg, 2)
	if node == nil {
		t.Error("GetNodeAtLine(2) should return a node")
	}
	if node != nil && node.ID != "block" {
		t.Errorf("GetNodeAtLine(2) = %s, want block", node.ID)
	}

	// Get node at line 3 (also within block)
	node = GetNodeAtLine(pdg, 3)
	if node == nil {
		t.Error("GetNodeAtLine(3) should return a node")
	}

	// Get node at line 5 (outside any block)
	node = GetNodeAtLine(pdg, 5)
	if node != nil {
		t.Error("GetNodeAtLine(5) should return nil")
	}
}

// TestGetAllNodesAtLine tests GetAllNodesAtLine function
func TestGetAllNodesAtLine(t *testing.T) {
	cfgInfo := &cfg.CFGInfo{
		FunctionName: "test_all_nodes",
		Blocks: map[string]cfg.CFGBlock{
			"entry": {ID: "entry", Type: cfg.BlockTypeEntry, StartLine: 1, EndLine: 2},
			"block": {ID: "block", Type: cfg.BlockTypePlain, StartLine: 1, EndLine: 3},
			"exit":  {ID: "exit", Type: cfg.BlockTypeExit, StartLine: 4, EndLine: 4},
		},
		Edges: []cfg.CFGEdge{
			{SourceID: "entry", TargetID: "block", EdgeType: cfg.EdgeTypeUnconditional},
			{SourceID: "block", TargetID: "exit", EdgeType: cfg.EdgeTypeUnconditional},
		},
		EntryBlockID: "entry",
		ExitBlockIDs: []string{"exit"},
	}

	builder := NewPDGBuilder(cfgInfo, nil)
	pdg := builder.Build()

	// Line 1 is in both entry and block
	nodes := GetAllNodesAtLine(pdg, 1)
	if len(nodes) != 2 {
		t.Errorf("GetAllNodesAtLine(1) = %d nodes, want 2", len(nodes))
	}

	// Line 4 is only in exit
	nodes = GetAllNodesAtLine(pdg, 4)
	if len(nodes) != 1 {
		t.Errorf("GetAllNodesAtLine(4) = %d nodes, want 1", len(nodes))
	}

	// Line 10 is in no blocks
	nodes = GetAllNodesAtLine(pdg, 10)
	if len(nodes) != 0 {
		t.Errorf("GetAllNodesAtLine(10) = %d nodes, want 0", len(nodes))
	}
}
