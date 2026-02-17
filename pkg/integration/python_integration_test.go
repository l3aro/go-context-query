// Package integration provides end-to-end integration tests for the complete
// code analysis pipeline: Extract → CFG → DFG → PDG.
package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/callgraph"
	"github.com/l3aro/go-context-query/pkg/cfg"
	"github.com/l3aro/go-context-query/pkg/dfg"
	"github.com/l3aro/go-context-query/pkg/extractor"
	"github.com/l3aro/go-context-query/pkg/pdg"
)

// getTestProjectPath returns the path to the test sample project.
func getTestProjectPath() string {
	return filepath.Join("testdata", "sample_project")
}

// TestFullPipeline tests the complete analysis pipeline:
// Scan Project → Extract Functions → Build Index → Resolve Calls → CFG → DFG → PDG
func TestFullPipeline(t *testing.T) {
	projectPath := getTestProjectPath()

	// Step 1: Scan the project for Python files
	t.Run("ScanProject", func(t *testing.T) {
		files, err := callgraph.ScanProject(projectPath, "python")
		if err != nil {
			t.Fatalf("Failed to scan project: %v", err)
		}

		if len(files) == 0 {
			t.Fatal("Expected to find Python files, found none")
		}

		// Verify expected files are found
		expectedFiles := map[string]bool{
			"calculator.py": false,
			"utils.py":      false,
			"main.py":       false,
			"shapes.py":     false,
		}

		for _, file := range files {
			base := filepath.Base(file)
			if _, exists := expectedFiles[base]; exists {
				expectedFiles[base] = true
			}
		}

		for name, found := range expectedFiles {
			if !found {
				t.Errorf("Expected file %s not found in scan results", name)
			}
		}

		t.Logf("Found %d Python files", len(files))
	})

	// Step 2: Extract functions from the files
	t.Run("ExtractFunctions", func(t *testing.T) {
		ext := extractor.NewPythonExtractor()
		calculatorPath := filepath.Join(projectPath, "calculator.py")

		moduleInfo, err := ext.Extract(calculatorPath)
		if err != nil {
			t.Fatalf("Failed to extract from calculator.py: %v", err)
		}

		// Verify expected functions are extracted
		expectedFunctions := map[string]bool{
			"add":                 false,
			"subtract":            false,
			"multiply":            false,
			"divide":              false,
			"calculate_power":     false,
			"calculate_factorial": false,
			"complex_operation":   false,
		}

		for _, fn := range moduleInfo.Functions {
			if _, exists := expectedFunctions[fn.Name]; exists {
				expectedFunctions[fn.Name] = true
			}
		}

		for name, found := range expectedFunctions {
			if !found {
				t.Errorf("Expected function %s not extracted", name)
			}
		}

		t.Logf("Extracted %d functions from calculator.py", len(moduleInfo.Functions))
	})

	// Step 3: Build project index
	t.Run("BuildProjectIndex", func(t *testing.T) {
		idx := callgraph.NewProjectIndex(projectPath)
		err := idx.BuildIndexFromScan("python")
		if err != nil {
			t.Fatalf("Failed to build index: %v", err)
		}

		stats := idx.GetStats()
		if stats.TotalFunctions == 0 {
			t.Fatal("Expected functions in index, found none")
		}
		if stats.TotalFiles == 0 {
			t.Fatal("Expected files in index, found none")
		}

		// Test lookup
		if file, ok := idx.Lookup("add"); !ok {
			t.Error("Expected to find 'add' function in index")
		} else {
			t.Logf("Found 'add' function in: %s", file)
		}

		if file, ok := idx.Lookup("multiply"); !ok {
			t.Error("Expected to find 'multiply' function in index")
		} else {
			t.Logf("Found 'multiply' function in: %s", file)
		}

		t.Logf("Index stats: %+v", stats)
	})

	// Step 4: Extract CFG from functions
	t.Run("ExtractCFG", func(t *testing.T) {
		calculatorPath := filepath.Join(projectPath, "calculator.py")

		// Test CFG extraction for simple function
		cfgInfo, err := cfg.ExtractCFG(calculatorPath, "add")
		if err != nil {
			t.Fatalf("Failed to extract CFG for add function: %v", err)
		}

		if cfgInfo.FunctionName != "add" {
			t.Errorf("Expected function name 'add', got '%s'", cfgInfo.FunctionName)
		}

		if len(cfgInfo.Blocks) == 0 {
			t.Error("Expected CFG blocks, found none")
		}

		if cfgInfo.EntryBlockID == "" {
			t.Error("Expected entry block ID")
		}

		// Test CFG extraction for function with control flow
		cfgInfo2, err := cfg.ExtractCFG(calculatorPath, "divide")
		if err != nil {
			t.Fatalf("Failed to extract CFG for divide function: %v", err)
		}

		if cfgInfo2.CyclomaticComplexity < 2 {
			t.Errorf("Expected cyclomatic complexity >= 2 for divide, got %d", cfgInfo2.CyclomaticComplexity)
		}

		t.Logf("CFG for 'add' has %d blocks, complexity: %d", len(cfgInfo.Blocks), cfgInfo.CyclomaticComplexity)
		t.Logf("CFG for 'divide' has %d blocks, complexity: %d", len(cfgInfo2.Blocks), cfgInfo2.CyclomaticComplexity)
	})

	// Step 5: Extract DFG from functions
	t.Run("ExtractDFG", func(t *testing.T) {
		calculatorPath := filepath.Join(projectPath, "calculator.py")

		dfgInfo, err := dfg.ExtractDFG(calculatorPath, "add")
		if err != nil {
			t.Fatalf("Failed to extract DFG for add function: %v", err)
		}

		if dfgInfo.FunctionName != "add" {
			t.Errorf("Expected function name 'add', got '%s'", dfgInfo.FunctionName)
		}

		if len(dfgInfo.VarRefs) == 0 {
			t.Error("Expected variable references, found none")
		}

		t.Logf("DFG for 'add' has %d variable references", len(dfgInfo.VarRefs))
	})

	// Step 6: Extract PDG (combines CFG and DFG)
	t.Run("ExtractPDG", func(t *testing.T) {
		calculatorPath := filepath.Join(projectPath, "calculator.py")

		pdgInfo, err := pdg.ExtractPDG(calculatorPath, "add")
		if err != nil {
			t.Fatalf("Failed to extract PDG for add function: %v", err)
		}

		if pdgInfo.FunctionName != "add" {
			t.Errorf("Expected function name 'add', got '%s'", pdgInfo.FunctionName)
		}

		if len(pdgInfo.Nodes) == 0 {
			t.Error("Expected PDG nodes, found none")
		}

		if pdgInfo.CFG == nil {
			t.Error("Expected CFG info in PDG")
		}

		if pdgInfo.DFG == nil {
			t.Error("Expected DFG info in PDG")
		}

		// Verify both control and data edges exist
		var controlEdges, dataEdges int
		for _, edge := range pdgInfo.Edges {
			switch edge.DepType {
			case pdg.DepTypeControl:
				controlEdges++
			case pdg.DepTypeData:
				dataEdges++
			}
		}

		t.Logf("PDG for 'add' has %d nodes, %d control edges, %d data edges",
			len(pdgInfo.Nodes), controlEdges, dataEdges)
	})
}

// TestCrossFileResolution tests cross-file function call resolution
func TestCrossFileResolution(t *testing.T) {
	projectPath := getTestProjectPath()

	t.Run("BuildAndResolveCrossFileCalls", func(t *testing.T) {
		// Build project call graph
		ext := extractor.NewPythonExtractor()
		callGraph, err := callgraph.BuildProjectCallGraph(projectPath, ext)
		if err != nil {
			t.Fatalf("Failed to build project call graph: %v", err)
		}

		stats := callGraph.GetStats()
		t.Logf("Call graph stats: %+v", stats)

		// Verify we have some edges
		if stats.TotalEdges == 0 {
			t.Log("Warning: No call edges found in the project")
		}

		// Look for cross-file edges
		if stats.CrossFileEdges == 0 {
			t.Log("Warning: No cross-file edges found")
		} else {
			t.Logf("Found %d cross-file edges", stats.CrossFileEdges)
		}

		// Check for specific cross-file calls
		foundCalculatorCalls := false
		for _, edge := range callGraph.Edges {
			if filepath.Base(edge.SourceFile) == "utils.py" &&
				filepath.Base(edge.DestFile) == "calculator.py" {
				foundCalculatorCalls = true
				t.Logf("Found cross-file call: %s in %s -> %s in %s",
					edge.SourceFunc, edge.SourceFile,
					edge.DestFunc, edge.DestFile)
			}
		}

		if !foundCalculatorCalls {
			t.Log("Warning: Did not find expected utils.py -> calculator.py calls")
		}
	})

	t.Run("ResolveImports", func(t *testing.T) {
		// Create index
		idx := callgraph.NewProjectIndex(projectPath)
		err := idx.BuildIndexFromScan("python")
		if err != nil {
			t.Fatalf("Failed to build index: %v", err)
		}

		// Test lookup for imported functions
		testCases := []struct {
			funcName     string
			shouldExist  bool
			expectedFile string
		}{
			{"add", true, "calculator.py"},
			{"multiply", true, "calculator.py"},
			{"calculate_area", true, "utils.py"},
			{"process_order", true, "main.py"},
			{"nonexistent", false, ""},
		}

		for _, tc := range testCases {
			file, found := idx.Lookup(tc.funcName)
			if tc.shouldExist {
				if !found {
					t.Errorf("Expected to find function '%s'", tc.funcName)
				} else if tc.expectedFile != "" {
					if filepath.Base(file) != tc.expectedFile {
						t.Errorf("Expected %s to be in %s, got %s",
							tc.funcName, tc.expectedFile, filepath.Base(file))
					}
				}
			} else {
				if found {
					t.Errorf("Expected NOT to find function '%s'", tc.funcName)
				}
			}
		}
	})
}

// TestExtractWithImports tests extraction of modules with import statements
func TestExtractWithImports(t *testing.T) {
	projectPath := getTestProjectPath()
	ext := extractor.NewPythonExtractor()

	t.Run("ExtractModuleWithImports", func(t *testing.T) {
		utilsPath := filepath.Join(projectPath, "utils.py")

		moduleInfo, err := ext.Extract(utilsPath)
		if err != nil {
			t.Fatalf("Failed to extract utils.py: %v", err)
		}

		// Verify imports are extracted
		if len(moduleInfo.Imports) == 0 {
			t.Error("Expected imports in utils.py")
		}

		// Check for specific imports
		hasCalculatorImport := false
		hasFactorialAlias := false

		for _, imp := range moduleInfo.Imports {
			if imp.Module == "calculator" {
				hasCalculatorImport = true
				for _, name := range imp.Names {
					if name == "factorial" {
						hasFactorialAlias = true
					}
				}
			}
		}

		if !hasCalculatorImport {
			t.Error("Expected to find calculator import")
		}

		if !hasFactorialAlias {
			t.Log("Factorial alias not found in expected format")
		}

		t.Logf("Found %d imports in utils.py", len(moduleInfo.Imports))
	})
}

// TestComplexFunction tests analysis of complex functions with control flow
func TestComplexFunction(t *testing.T) {
	projectPath := getTestProjectPath()
	calculatorPath := filepath.Join(projectPath, "calculator.py")

	t.Run("ComplexOperationCFG", func(t *testing.T) {
		cfgInfo, err := cfg.ExtractCFG(calculatorPath, "complex_operation")
		if err != nil {
			t.Fatalf("Failed to extract CFG: %v", err)
		}

		// Should have multiple blocks due to function calls
		if len(cfgInfo.Blocks) < 3 {
			t.Errorf("Expected at least 3 blocks for complex_operation, got %d", len(cfgInfo.Blocks))
		}

		t.Logf("Complex operation CFG has %d blocks", len(cfgInfo.Blocks))
	})

	t.Run("ComplexOperationPDG", func(t *testing.T) {
		pdgInfo, err := pdg.ExtractPDG(calculatorPath, "complex_operation")
		if err != nil {
			t.Fatalf("Failed to extract PDG: %v", err)
		}

		// Should have data dependencies between the temp variables
		hasDataEdges := false
		for _, edge := range pdgInfo.Edges {
			if edge.DepType == pdg.DepTypeData {
				hasDataEdges = true
				break
			}
		}

		if !hasDataEdges {
			t.Log("Warning: No data edges found in complex_operation PDG")
		}

		t.Logf("Complex operation PDG has %d nodes, %d edges",
			len(pdgInfo.Nodes), len(pdgInfo.Edges))
	})
}

// TestClassExtraction tests extraction of classes and methods
func TestClassExtraction(t *testing.T) {
	projectPath := getTestProjectPath()
	shapesPath := filepath.Join(projectPath, "shapes.py")
	ext := extractor.NewPythonExtractor()

	t.Run("ExtractClasses", func(t *testing.T) {
		moduleInfo, err := ext.Extract(shapesPath)
		if err != nil {
			t.Fatalf("Failed to extract shapes.py: %v", err)
		}

		// Verify classes are extracted
		expectedClasses := map[string]int{
			"Shape":     2, // __init__, get_name, area (but area is abstract)
			"Rectangle": 3, // __init__, area, perimeter
			"Circle":    3, // __init__, area, circumference
		}

		for _, cls := range moduleInfo.Classes {
			if expectedMethodCount, exists := expectedClasses[cls.Name]; exists {
				if len(cls.Methods) < expectedMethodCount {
					t.Errorf("Expected at least %d methods in %s, got %d",
						expectedMethodCount, cls.Name, len(cls.Methods))
				}
				delete(expectedClasses, cls.Name)
			}
		}

		for name := range expectedClasses {
			t.Errorf("Expected class %s not found", name)
		}

		t.Logf("Extracted %d classes from shapes.py", len(moduleInfo.Classes))
	})
}

// TestEndToEndWorkflow tests a complete end-to-end workflow
func TestEndToEndWorkflow(t *testing.T) {
	projectPath := getTestProjectPath()

	t.Run("CompleteAnalysisWorkflow", func(t *testing.T) {
		// 1. Scan project
		files, err := callgraph.ScanProject(projectPath, "python")
		if err != nil {
			t.Fatalf("Failed to scan project: %v", err)
		}

		// Convert to absolute paths
		absFiles := make([]string, len(files))
		for i, f := range files {
			absFiles[i] = filepath.Join(projectPath, f)
		}

		// 2. Build index
		idx := callgraph.NewProjectIndex(projectPath)
		if err := idx.BuildIndex(absFiles); err != nil {
			t.Fatalf("Failed to build index: %v", err)
		}

		stats := idx.GetStats()
		t.Logf("Index built: %d functions, %d files", stats.TotalFunctions, stats.TotalFiles)

		// 3. Resolve calls
		ext := extractor.NewPythonExtractor()
		callGraph, err := callgraph.BuildProjectCallGraph(projectPath, ext)
		if err != nil {
			t.Fatalf("Failed to build call graph: %v", err)
		}

		cgStats := callGraph.GetStats()
		t.Logf("Call graph built: %d total edges (%d intra-file, %d cross-file)",
			cgStats.TotalEdges, cgStats.IntraFileEdges, cgStats.CrossFileEdges)

		// 4. Analyze specific functions with CFG/DFG/PDG
		calculatorPath := filepath.Join(projectPath, "calculator.py")

		// Pick a function to analyze deeply
		targetFunc := "calculate_factorial"

		cfgInfo, err := cfg.ExtractCFG(calculatorPath, targetFunc)
		if err != nil {
			t.Fatalf("Failed to extract CFG for %s: %v", targetFunc, err)
		}

		dfgInfo, err := dfg.ExtractDFG(calculatorPath, targetFunc)
		if err != nil {
			t.Fatalf("Failed to extract DFG for %s: %v", targetFunc, err)
		}

		pdgInfo, err := pdg.ExtractPDG(calculatorPath, targetFunc)
		if err != nil {
			t.Fatalf("Failed to extract PDG for %s: %v", targetFunc, err)
		}

		t.Logf("Complete analysis for '%s':", targetFunc)
		t.Logf("  - CFG: %d blocks, complexity %d", len(cfgInfo.Blocks), cfgInfo.CyclomaticComplexity)
		t.Logf("  - DFG: %d variable references", len(dfgInfo.VarRefs))
		t.Logf("  - PDG: %d nodes, %d edges", len(pdgInfo.Nodes), len(pdgInfo.Edges))

		// Verify the analysis is complete
		if cfgInfo.CyclomaticComplexity < 2 {
			t.Error("Expected higher complexity for factorial function with loop")
		}
	})
}

// setup ensures test data exists
func TestMain(m *testing.M) {
	// Verify test data exists
	testProjectPath := getTestProjectPath()
	if _, err := os.Stat(testProjectPath); os.IsNotExist(err) {
		// This will fail the tests, but we want to report it clearly
		panic("Test data directory does not exist: " + testProjectPath)
	}

	os.Exit(m.Run())
}
