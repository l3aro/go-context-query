package callgraph

import (
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/extractor"
)

// TestGoCrossFileResolution tests that the resolver can index Go files correctly
// and build a function index that can be used for cross-file resolution.
func TestGoCrossFileResolution(t *testing.T) {
	testDataDir := filepath.Join("..", "..", "testdata", "go")

	// Get all Go files
	goFiles := []string{
		filepath.Join(testDataDir, "main.go"),
		filepath.Join(testDataDir, "helper.go"),
		filepath.Join(testDataDir, "utils", "math.go"),
	}

	// Create Go extractor
	goExtractor := extractor.NewGoExtractor()

	// Create resolver with Go extractor
	resolver := NewResolver(testDataDir, goExtractor)

	// Build the function index
	err := resolver.BuildIndex(goFiles)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	// Verify functions are indexed
	index := resolver.GetIndex()

	// Check that main.go functions are indexed
	mainFile := filepath.Join(testDataDir, "main.go")
	if funcs := index.GetFunctionsInFile(mainFile); len(funcs) == 0 {
		t.Error("Expected functions in main.go to be indexed")
	}

	// Check that helper.go functions are indexed
	helperFile := filepath.Join(testDataDir, "helper.go")
	if funcs := index.GetFunctionsInFile(helperFile); len(funcs) == 0 {
		t.Error("Expected functions in helper.go to be indexed")
	}

	// Check that utils/math.go functions are indexed
	mathFile := filepath.Join(testDataDir, "utils", "math.go")
	if funcs := index.GetFunctionsInFile(mathFile); len(funcs) == 0 {
		t.Error("Expected functions in utils/math.go to be indexed")
	}

	// Test function lookup
	// The module name for main.go should be "main" (file path relative to root)
	if file, found := index.Lookup("main", "main"); !found {
		t.Error("Expected to find 'main' function")
	} else if filepath.Base(file) != "main.go" {
		t.Errorf("Expected main function in main.go, got %s", file)
	}

	// Test lookup - main.go has a local helper() so use qualified lookup for unique name
	if file, found := index.Lookup("main", "HelperFunction"); !found {
		t.Error("Expected to find 'HelperFunction' function")
	} else if filepath.Base(file) != "helper.go" {
		t.Errorf("Expected HelperFunction in helper.go, got %s", file)
	}

	// Test lookup of math package functions
	// The module name for utils/math.go should be "utils.math"
	if file, found := index.Lookup("utils.math", "Add"); !found {
		t.Error("Expected to find 'Add' function in utils.math")
	} else if filepath.Base(filepath.Dir(file)) != "utils" {
		t.Errorf("Expected Add function in utils/, got %s", file)
	}
}

// TestTypeScriptCrossFileResolution tests that the resolver can index TypeScript files correctly
// and build a function index that can be used for cross-file resolution.
func TestTypeScriptCrossFileResolution(t *testing.T) {
	testDataDir := filepath.Join("..", "..", "testdata", "typescript")

	// Get all TypeScript files
	tsFiles := []string{
		filepath.Join(testDataDir, "main.ts"),
		filepath.Join(testDataDir, "helper.ts"),
		filepath.Join(testDataDir, "utils", "math.ts"),
	}

	// Create TypeScript extractor
	tsExtractor := extractor.NewTypeScriptExtractor()

	// Create resolver with TypeScript extractor
	resolver := NewResolver(testDataDir, tsExtractor)

	// Build the function index
	err := resolver.BuildIndex(tsFiles)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	// Verify functions are indexed
	index := resolver.GetIndex()

	// Check that main.ts functions are indexed
	mainFile := filepath.Join(testDataDir, "main.ts")
	if funcs := index.GetFunctionsInFile(mainFile); len(funcs) == 0 {
		t.Error("Expected functions in main.ts to be indexed")
	}

	// Check that helper.ts is indexed (as a class)
	helperFile := filepath.Join(testDataDir, "helper.ts")
	if funcs := index.GetFunctionsInFile(helperFile); len(funcs) == 0 {
		t.Error("Expected HelperClass in helper.ts to be indexed")
	}

	// Check that utils/math.ts is indexed (as a class)
	mathFile := filepath.Join(testDataDir, "utils", "math.ts")
	if funcs := index.GetFunctionsInFile(mathFile); len(funcs) == 0 {
		t.Error("Expected MathUtils in utils/math.ts to be indexed")
	}

	// Test function lookup for main.ts
	// The module name for main.ts should be "main"
	if file, found := index.Lookup("main", "MainClass"); !found {
		t.Error("Expected to find 'MainClass'")
	} else if filepath.Base(file) != "main.ts" {
		t.Errorf("Expected MainClass in main.ts, got %s", file)
	}

	// Test lookup of HelperClass
	if file, found := index.Lookup("helper", "HelperClass"); !found {
		t.Error("Expected to find 'HelperClass'")
	} else if filepath.Base(file) != "helper.ts" {
		t.Errorf("Expected HelperClass in helper.ts, got %s", file)
	}

	// Test lookup of MathUtils
	if file, found := index.Lookup("utils.math", "MathUtils"); !found {
		t.Error("Expected to find 'MathUtils'")
	} else if filepath.Base(filepath.Dir(file)) != "utils" {
		t.Errorf("Expected MathUtils in utils/, got %s", file)
	}
}

// TestGoModuleNameResolution tests that module names are derived correctly from file paths.
func TestGoModuleNameResolution(t *testing.T) {
	testDataDir := filepath.Join("..", "..", "testdata", "go")

	goExtractor := extractor.NewGoExtractor()
	resolver := NewResolver(testDataDir, goExtractor)

	tests := []struct {
		filePath     string
		expectedName string
	}{
		{"main.go", "main"},
		{"helper.go", "helper"},
		{"utils/math.go", "utils.math"},
		{"utils/strings/format.go", "utils.strings.format"},
	}

	for _, tt := range tests {
		name := resolver.filePathToModuleName(tt.filePath)
		if name != tt.expectedName {
			t.Errorf("Expected module name %q, got %q for file %s", tt.expectedName, name, tt.filePath)
		}
	}
}

// TestTypeScriptModuleNameResolution tests that module names are derived correctly from file paths.
func TestTypeScriptModuleNameResolution(t *testing.T) {
	testDataDir := filepath.Join("..", "..", "testdata", "typescript")

	tsExtractor := extractor.NewTypeScriptExtractor()
	resolver := NewResolver(testDataDir, tsExtractor)

	tests := []struct {
		filePath     string
		expectedName string
	}{
		{"main.ts", "main"},
		{"helper.ts", "helper"},
		{"utils/math.ts", "utils.math"},
		{"utils/strings/format.ts", "utils.strings.format"},
	}

	for _, tt := range tests {
		name := resolver.filePathToModuleName(tt.filePath)
		if name != tt.expectedName {
			t.Errorf("Expected module name %q, got %q for file %s", tt.expectedName, name, tt.filePath)
		}
	}
}

// TestFunctionIndexQualifiedNames tests that qualified function names are indexed correctly.
func TestFunctionIndexQualifiedNames(t *testing.T) {
	index := NewFunctionIndex()

	// Add functions with module names
	index.AddFunction("utils.math", "Add", "/path/to/utils/math.go")
	index.AddFunction("utils.math", "Multiply", "/path/to/utils/math.go")
	index.AddFunction("main", "main", "/path/to/main.go")
	index.AddFunction("main", "helper", "/path/to/helper.go")

	// Test simple name lookup
	if file, found := index.Lookup("", "Add"); !found {
		t.Error("Expected to find Add by simple name")
	} else if file != "/path/to/utils/math.go" {
		t.Errorf("Expected /path/to/utils/math.go, got %s", file)
	}

	// Test qualified name lookup
	if file, found := index.Lookup("utils.math", "Add"); !found {
		t.Error("Expected to find Add by qualified name")
	} else if file != "/path/to/utils/math.go" {
		t.Errorf("Expected /path/to/utils/math.go, got %s", file)
	}

	// Test simple module lookup (last component)
	if file, found := index.Lookup("math", "Add"); !found {
		t.Error("Expected to find Add by simple module name")
	} else if file != "/path/to/utils/math.go" {
		t.Errorf("Expected /path/to/utils/math.go, got %s", file)
	}
}

// TestCrossFileEdgeResolution tests that the resolver can identify cross-file edges.
func TestCrossFileEdgeResolution(t *testing.T) {
	testDataDir := filepath.Join("..", "..", "testdata", "go")

	goFiles := []string{
		filepath.Join(testDataDir, "main.go"),
		filepath.Join(testDataDir, "helper.go"),
		filepath.Join(testDataDir, "utils", "math.go"),
	}

	goExtractor := extractor.NewGoExtractor()
	resolver := NewResolver(testDataDir, goExtractor)

	// Build the index
	err := resolver.BuildIndex(goFiles)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	// Get the index and verify it has functions from multiple files
	index := resolver.GetIndex()

	// Verify we have functions from different files
	mainFile := filepath.Join(testDataDir, "main.go")
	helperFile := filepath.Join(testDataDir, "helper.go")
	mathFile := filepath.Join(testDataDir, "utils", "math.go")

	mainFuncs := index.GetFunctionsInFile(mainFile)
	helperFuncs := index.GetFunctionsInFile(helperFile)
	mathFuncs := index.GetFunctionsInFile(mathFile)

	if len(mainFuncs) == 0 {
		t.Error("Expected functions in main.go")
	}
	if len(helperFuncs) == 0 {
		t.Error("Expected functions in helper.go")
	}
	if len(mathFuncs) == 0 {
		t.Error("Expected functions in utils/math.go")
	}

	// Verify cross-file resolution is possible
	// main.go calls helper() from helper.go and math.Add from utils/math.go
	_, foundHelper := index.Lookup("main", "helper")
	if !foundHelper {
		t.Error("Expected to resolve cross-file call to helper")
	}

	_, foundMath := index.Lookup("utils.math", "Add")
	if !foundMath {
		t.Error("Expected to resolve cross-file call to utils.math.Add")
	}
}
