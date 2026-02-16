package callgraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/go-context-query/pkg/extractor"
	"github.com/user/go-context-query/pkg/types"
)

// TestBuilderBasicCallGraph tests basic call graph building
func TestBuilderBasicCallGraph(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test_calls.py")

	pythonCode := `def helper():
    """A helper function."""
    return 42

def main():
    """Main function that calls helper."""
    result = helper()
    return result

def complex_function():
    """Function with multiple calls."""
    a = helper()
    b = helper()
    return a + b
`

	err := os.WriteFile(pyFile, []byte(pythonCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Extract module info first
	pyExtractor := extractor.NewPythonExtractor()
	moduleInfo, err := pyExtractor.Extract(pyFile)
	if err != nil {
		t.Fatalf("Failed to extract module info: %v", err)
	}

	// Build call graph
	builder := NewBuilder()
	graph, err := builder.BuildFromFile(pyFile, moduleInfo)
	if err != nil {
		t.Fatalf("BuildFromFile() failed: %v", err)
	}

	// Test basic structure
	if graph.FilePath != pyFile {
		t.Errorf("Expected file path %s, got %s", pyFile, graph.FilePath)
	}

	// Should have entries for functions with bodies
	if len(graph.Entries) != 3 {
		t.Errorf("Expected 3 function entries, got %d", len(graph.Entries))
	}

	// Test main function calls helper
	mainCalls := graph.GetCalls("main")
	if len(mainCalls) != 1 {
		t.Errorf("Expected 1 call from main, got %d", len(mainCalls))
	}

	if len(mainCalls) > 0 {
		if mainCalls[0].Name != "helper" {
			t.Errorf("Expected main to call 'helper', got '%s'", mainCalls[0].Name)
		}
		if mainCalls[0].Type != LocalCall {
			t.Errorf("Expected helper to be LocalCall, got %s", mainCalls[0].Type)
		}
	}

	// Test complex_function has multiple calls
	complexCalls := graph.GetCalls("complex_function")
	if len(complexCalls) != 2 {
		t.Errorf("Expected 2 calls from complex_function, got %d", len(complexCalls))
	}

	// Test helper has no calls
	helperCalls := graph.GetCalls("helper")
	if len(helperCalls) != 0 {
		t.Errorf("Expected 0 calls from helper, got %d", len(helperCalls))
	}
}

// TestBuilderMethodCalls tests method call detection
func TestBuilderMethodCalls(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test_methods.py")

	pythonCode := `
class MyClass:
    def method1(self):
        """First method."""
        return 1
    
    def method2(self):
        """Second method that calls method1."""
        return self.method1()
    
    def method3(self, other):
        """Third method that calls method on another object."""
        return other.method1()

def standalone():
    """Standalone function."""
    obj = MyClass()
    return obj.method1()
`

	err := os.WriteFile(pyFile, []byte(pythonCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	pyExtractor := extractor.NewPythonExtractor()
	moduleInfo, err := pyExtractor.Extract(pyFile)
	if err != nil {
		t.Fatalf("Failed to extract module info: %v", err)
	}

	builder := NewBuilder()
	graph, err := builder.BuildFromFile(pyFile, moduleInfo)
	if err != nil {
		t.Fatalf("BuildFromFile() failed: %v", err)
	}

	// Test method2 calls self.method1
	method2Calls := graph.GetCalls("method2")
	if len(method2Calls) != 1 {
		t.Errorf("Expected 1 call from method2, got %d", len(method2Calls))
	}

	if len(method2Calls) > 0 {
		if method2Calls[0].Name != "self.method1" {
			t.Errorf("Expected 'self.method1', got '%s'", method2Calls[0].Name)
		}
		if method2Calls[0].Type != MethodCall {
			t.Errorf("Expected MethodCall, got %s", method2Calls[0].Type)
		}
		if method2Calls[0].Base != "self" {
			t.Errorf("Expected base 'self', got '%s'", method2Calls[0].Base)
		}
		if method2Calls[0].Method != "method1" {
			t.Errorf("Expected method 'method1', got '%s'", method2Calls[0].Method)
		}
	}

	// Test standalone function calls MyClass() and obj.method1
	standaloneCalls := graph.GetCalls("standalone")
	// Both MyClass() constructor and obj.method1() are calls
	if len(standaloneCalls) != 2 {
		t.Errorf("Expected 2 calls from standalone, got %d", len(standaloneCalls))
	}

	// Find the method call among the calls
	var methodCall *CalledFunction
	for i := range standaloneCalls {
		if standaloneCalls[i].Name == "obj.method1" {
			methodCall = &standaloneCalls[i]
			break
		}
	}

	if methodCall == nil {
		t.Errorf("Expected 'obj.method1' call, got: %v", standaloneCalls)
	} else {
		// obj.method1 should be detected as method call (heuristic)
		if methodCall.Type != MethodCall {
			t.Errorf("Expected MethodCall for obj.method1, got %s", methodCall.Type)
		}
	}
}

// TestBuilderExternalCalls tests external call detection
func TestBuilderExternalCalls(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test_external.py")

	pythonCode := `import os
import sys
from typing import List
from collections import defaultdict

def use_os():
    """Function that uses os module."""
    return os.path.join("a", "b")

def use_builtin():
    """Function that uses builtins."""
    return len([1, 2, 3])

def use_imported():
    """Function that uses imported functions."""
    return List([1, 2, 3])

def mixed_calls():
    """Function with mixed call types."""
    a = len("hello")  # builtin
    b = os.path.exists("file.txt")  # module call
    return a, b
`

	err := os.WriteFile(pyFile, []byte(pythonCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	pyExtractor := extractor.NewPythonExtractor()
	moduleInfo, err := pyExtractor.Extract(pyFile)
	if err != nil {
		t.Fatalf("Failed to extract module info: %v", err)
	}

	builder := NewBuilder()
	graph, err := builder.BuildFromFile(pyFile, moduleInfo)
	if err != nil {
		t.Fatalf("BuildFromFile() failed: %v", err)
	}

	// Test os.path.join is detected as external
	useOSCalls := graph.GetCalls("use_os")
	if len(useOSCalls) != 1 {
		t.Errorf("Expected 1 call from use_os, got %d", len(useOSCalls))
	}

	if len(useOSCalls) > 0 {
		if useOSCalls[0].Type != ExternalCall {
			t.Errorf("Expected os.path.join to be ExternalCall, got %s", useOSCalls[0].Type)
		}
	}

	// Test builtin detection
	useBuiltinCalls := graph.GetCalls("use_builtin")
	if len(useBuiltinCalls) != 1 {
		t.Errorf("Expected 1 call from use_builtin, got %d", len(useBuiltinCalls))
	}

	if len(useBuiltinCalls) > 0 {
		if useBuiltinCalls[0].Type != ExternalCall {
			t.Errorf("Expected len() to be ExternalCall, got %s", useBuiltinCalls[0].Type)
		}
	}

	// Test mixed calls
	mixedCalls := graph.GetCalls("mixed_calls")
	if len(mixedCalls) != 2 {
		t.Errorf("Expected 2 calls from mixed_calls, got %d", len(mixedCalls))
	}
}

// TestBuilderNestedCalls tests nested/chained call handling
func TestBuilderNestedCalls(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test_nested.py")

	pythonCode := `def inner():
    return lambda: 42

def outer():
    """Function with nested calls."""
    return inner()()

def chained():
    """Function with chained calls."""
    return "hello".upper().lower()

def list_comp():
    """Function with list comprehension."""
    return [str(x) for x in range(10)]
`

	err := os.WriteFile(pyFile, []byte(pythonCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	pyExtractor := extractor.NewPythonExtractor()
	moduleInfo, err := pyExtractor.Extract(pyFile)
	if err != nil {
		t.Fatalf("Failed to extract module info: %v", err)
	}

	builder := NewBuilder()
	graph, err := builder.BuildFromFile(pyFile, moduleInfo)
	if err != nil {
		t.Fatalf("BuildFromFile() failed: %v", err)
	}

	// Test outer calls inner
	outerCalls := graph.GetCalls("outer")
	if len(outerCalls) == 0 {
		t.Error("Expected outer to have calls")
	}

	// Should detect the inner() call
	foundInner := false
	for _, call := range outerCalls {
		if call.Name == "inner" {
			foundInner = true
			break
		}
	}
	if !foundInner {
		t.Errorf("Expected outer to call 'inner', calls were: %v", outerCalls)
	}
}

// TestBuilderFromBytes tests building from bytes directly
func TestBuilderFromBytes(t *testing.T) {
	pythonCode := []byte(`def foo():
    return bar()

def bar():
    return 42

def main():
    foo()
    bar()
`)

	// Create minimal module info
	moduleInfo := &types.ModuleInfo{
		Path: "test.py",
		Functions: []types.Function{
			{Name: "foo"},
			{Name: "bar"},
			{Name: "main"},
		},
		Imports: []types.Import{},
	}

	builder := NewBuilder()
	graph, err := builder.BuildFromBytes(pythonCode, "test.py", moduleInfo)
	if err != nil {
		t.Fatalf("BuildFromBytes() failed: %v", err)
	}

	// Test foo calls bar
	fooCalls := graph.GetCalls("foo")
	if len(fooCalls) != 1 || fooCalls[0].Name != "bar" {
		t.Errorf("Expected foo to call bar, got: %v", fooCalls)
	}

	// Test main calls both foo and bar
	mainCalls := graph.GetCalls("main")
	if len(mainCalls) != 2 {
		t.Errorf("Expected 2 calls from main, got %d", len(mainCalls))
	}
}

// TestCallGraphQueries tests the query methods
func TestCallGraphQueries(t *testing.T) {
	pythonCode := []byte(`def local_func():
    return 1

def external_func():
    return len("test")

def method_caller(obj):
    return obj.method()

def caller():
    local_func()
    external_func()
    method_caller(None)
`)

	moduleInfo := &types.ModuleInfo{
		Path: "test.py",
		Functions: []types.Function{
			{Name: "local_func"},
			{Name: "external_func"},
			{Name: "method_caller"},
			{Name: "caller"},
		},
		Imports: []types.Import{},
	}

	builder := NewBuilder()
	graph, err := builder.BuildFromBytes(pythonCode, "test.py", moduleInfo)
	if err != nil {
		t.Fatalf("BuildFromBytes() failed: %v", err)
	}

	// Test GetAllFunctions
	allFuncs := graph.GetAllFunctions()
	if len(allFuncs) != 4 {
		t.Errorf("Expected 4 functions, got %d", len(allFuncs))
	}

	// Test HasCalls
	if !graph.HasCalls("caller") {
		t.Error("Expected caller to have calls")
	}
	if graph.HasCalls("local_func") {
		t.Error("Expected local_func to have no calls")
	}

	// Test GetCallCount
	// Total calls: local_func (0) + external_func (1: len) + method_caller (1: obj.method) + caller (3: all local) = 5
	if graph.GetCallCount() != 5 {
		t.Errorf("Expected 5 total calls, got %d", graph.GetCallCount())
	}

	// Test GetLocalCalls for caller
	// caller() calls local_func(), external_func(), method_caller() - all are local functions defined in the file
	localCalls := graph.GetLocalCalls("caller")
	if len(localCalls) != 3 {
		t.Errorf("Expected 3 local calls from caller (local_func, external_func, method_caller), got %d", len(localCalls))
	}

	// Test GetExternalCalls for external_func
	// external_func() calls len() which is a builtin
	externalCalls := graph.GetExternalCalls("external_func")
	if len(externalCalls) != 1 {
		t.Errorf("Expected 1 external call from external_func (len), got %d", len(externalCalls))
	}

	// Test GetMethodCalls for method_caller
	// method_caller() calls obj.method()
	methodCalls := graph.GetMethodCalls("method_caller")
	if len(methodCalls) != 1 {
		t.Errorf("Expected 1 method call from method_caller, got %d", len(methodCalls))
	}
}

// TestToCallGraph tests conversion to types.CallGraph
func TestToCallGraph(t *testing.T) {
	pythonCode := []byte(`def a():
    b()

def b():
    c()

def c():
    pass
`)

	moduleInfo := &types.ModuleInfo{
		Path: "test.py",
		Functions: []types.Function{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		},
		Imports: []types.Import{},
	}

	builder := NewBuilder()
	graph, err := builder.BuildFromBytes(pythonCode, "test.py", moduleInfo)
	if err != nil {
		t.Fatalf("BuildFromBytes() failed: %v", err)
	}

	callGraph := graph.ToCallGraph()

	// Should have 2 edges: a->b and b->c
	if len(callGraph.Edges) != 2 {
		t.Errorf("Expected 2 edges, got %d", len(callGraph.Edges))
	}

	// Check edges
	for _, edge := range callGraph.Edges {
		if edge.SourceFile != "test.py" {
			t.Error("Expected source file to be test.py")
		}
		if edge.DestFile != "test.py" {
			t.Error("Expected dest file to be test.py")
		}
	}
}

// TestCallTypeHelpers tests the call type helper functions
func TestCallTypeHelpers(t *testing.T) {
	// Test isPythonBuiltin
	if !isPythonBuiltin("len") {
		t.Error("Expected 'len' to be a builtin")
	}
	if !isPythonBuiltin("print") {
		t.Error("Expected 'print' to be a builtin")
	}
	if isPythonBuiltin("my_custom_func") {
		t.Error("Expected 'my_custom_func' to not be a builtin")
	}

	// Test isLikelyInstanceName
	if !isLikelyInstanceName("self") {
		t.Error("Expected 'self' to be likely instance")
	}
	if !isLikelyInstanceName("obj") {
		t.Error("Expected 'obj' to be likely instance")
	}
	if !isLikelyInstanceName("x") {
		t.Error("Expected 'x' to be likely instance")
	}
	if isLikelyInstanceName("MyClassName") {
		t.Error("Expected 'MyClassName' to not be likely instance")
	}
}

// TestBuilderEmptyFile tests handling of empty files
func TestBuilderEmptyFile(t *testing.T) {
	pythonCode := []byte(``)

	moduleInfo := &types.ModuleInfo{
		Path: "empty.py",
	}

	builder := NewBuilder()
	graph, err := builder.BuildFromBytes(pythonCode, "empty.py", moduleInfo)
	if err != nil {
		t.Fatalf("BuildFromBytes() failed: %v", err)
	}

	if len(graph.Entries) != 0 {
		t.Errorf("Expected 0 entries for empty file, got %d", len(graph.Entries))
	}

	if graph.GetCallCount() != 0 {
		t.Errorf("Expected 0 calls for empty file, got %d", graph.GetCallCount())
	}
}

// TestBuilderNoCalls tests handling of functions with no calls
func TestBuilderNoCalls(t *testing.T) {
	pythonCode := []byte(`def no_calls():
    x = 1 + 2
    return x

def also_no_calls():
    """Just returns a value."""
    return "hello"
`)

	moduleInfo := &types.ModuleInfo{
		Path: "test.py",
		Functions: []types.Function{
			{Name: "no_calls"},
			{Name: "also_no_calls"},
		},
		Imports: []types.Import{},
	}

	builder := NewBuilder()
	graph, err := builder.BuildFromBytes(pythonCode, "test.py", moduleInfo)
	if err != nil {
		t.Fatalf("BuildFromBytes() failed: %v", err)
	}

	// Should still have entries for functions
	if len(graph.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(graph.Entries))
	}

	// But no calls
	if graph.GetCallCount() != 0 {
		t.Errorf("Expected 0 calls, got %d", graph.GetCallCount())
	}
}

// TestComplexClassStructure tests call graph in complex class structures
func TestComplexClassStructure(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test_class_complex.py")

	pythonCode := `
class Base:
    def base_method(self):
        return "base"

class Child(Base):
    def child_method(self):
        # Calls parent's method
        return self.base_method()
    
    def call_sibling(self):
        return self.another_method()
    
    def another_method(self):
        return "another"

def use_classes():
    c = Child()
    return c.child_method()
`

	err := os.WriteFile(pyFile, []byte(pythonCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	pyExtractor := extractor.NewPythonExtractor()
	moduleInfo, err := pyExtractor.Extract(pyFile)
	if err != nil {
		t.Fatalf("Failed to extract module info: %v", err)
	}

	builder := NewBuilder()
	graph, err := builder.BuildFromFile(pyFile, moduleInfo)
	if err != nil {
		t.Fatalf("BuildFromFile() failed: %v", err)
	}

	// Test child_method calls base_method (self.base_method())
	childMethodCalls := graph.GetCalls("child_method")
	foundBaseMethod := false
	for _, call := range childMethodCalls {
		if call.Method == "base_method" {
			foundBaseMethod = true
			if call.Type != MethodCall {
				t.Errorf("Expected base_method to be MethodCall, got %s", call.Type)
			}
		}
	}
	if !foundBaseMethod {
		t.Errorf("Expected child_method to call base_method, got: %v", childMethodCalls)
	}

	// Test use_classes calls Child() and c.child_method()
	useClassesCalls := graph.GetCalls("use_classes")
	// Both Child() constructor and c.child_method() are calls
	if len(useClassesCalls) != 2 {
		t.Errorf("Expected 2 calls from use_classes, got %d", len(useClassesCalls))
	}
}

// BenchmarkBuilder benchmarks the call graph builder
func BenchmarkBuilder(b *testing.B) {
	pythonCode := []byte(`def helper(n):
    return n * 2

def recursive(n):
    if n <= 0:
        return 0
    return helper(n) + recursive(n - 1)

def main():
    result = 0
    for i in range(100):
        result += recursive(i)
    return result
`)

	moduleInfo := &types.ModuleInfo{
		Path: "bench.py",
		Functions: []types.Function{
			{Name: "helper"},
			{Name: "recursive"},
			{Name: "main"},
		},
		Imports: []types.Import{},
	}

	builder := NewBuilder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.BuildFromBytes(pythonCode, "bench.py", moduleInfo)
		if err != nil {
			b.Fatalf("BuildFromBytes() failed: %v", err)
		}
	}
}
