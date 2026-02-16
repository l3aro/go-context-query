package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/go-context-query/pkg/types"
)

// TestPythonExtractorAdvanced tests advanced extraction features
func TestPythonExtractorAdvanced(t *testing.T) {
	// Create a temporary Python file with complex constructs
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "advanced_test.py")

	pythonCode := `"""Advanced test module with various Python constructs."""

import os
from typing import List, Optional, Dict
from functools import wraps

# Decorator definitions
def my_decorator(func):
    """A sample decorator."""
    @wraps(func)
    def wrapper(*args, **kwargs):
        return func(*args, **kwargs)
    return wrapper

def decorator_with_args(arg1, arg2):
    """Decorator factory."""
    def decorator(func):
        return func
    return decorator

# Standalone functions
def simple_function(x: int, y: int) -> int:
    """A simple function with type annotations."""
    return x + y

@my_decorator
def decorated_function():
    """A decorated function."""
    pass

@decorator_with_args("arg1", "arg2")
def decorated_with_args():
    """Function with decorator that has arguments."""
    pass

async def async_function() -> str:
    """An async function with return type."""
    return "hello"

async def async_decorated_function():
    """An async decorated function."""
    pass

def function_no_types(a, b):
    """Function without type annotations."""
    return a + b

def no_docstring():
    pass

# Class definitions
class BaseClass:
    """A base class."""
    
    def __init__(self, value: int):
        self.value = value
    
    def base_method(self) -> int:
        """A method in base class."""
        return self.value

class ChildClass(BaseClass):
    """A child class inheriting from BaseClass."""
    
    def child_method(self, x: int) -> int:
        """A method in child class."""
        return x * 2

class MultipleInheritance(BaseClass, object):
    """Class with multiple inheritance."""
    pass

@my_decorator
class DecoratedClass:
    """A decorated class."""
    
    @staticmethod
    def static_method():
        """A static method."""
        pass
    
    @classmethod
    def class_method(cls):
        """A class method."""
        pass
    
    async def async_method(self) -> None:
        """An async method."""
        pass

class GenericClass(List[int]):
    """Class with generic base."""
    pass

class QualifiedBase(os.PathLike):
    """Class with module-qualified base."""
    pass

# Functions that should NOT be extracted as top-level
class ContainerClass:
    """A container class with methods."""
    
    def method1(self):
        """First method."""
        pass
    
    def method2(self):
        """Second method."""
        pass
    
    def nested_function_container(self):
        """Method that contains a nested function."""
        def nested():
            """Nested function - should not be extracted as top-level."""
            pass
        return nested
`

	err := os.WriteFile(pyFile, []byte(pythonCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Extract
	extractor := NewPythonExtractor()
	info, err := extractor.Extract(pyFile)
	if err != nil {
		t.Fatalf("Extract() failed: %v", err)
	}

	// Test function extraction
	t.Run("FunctionExtraction", func(t *testing.T) {
		if len(info.Functions) == 0 {
			t.Fatal("Expected functions to be extracted")
		}

		// Build a map of function names
		funcMap := make(map[string]bool)
		for _, fn := range info.Functions {
			funcMap[fn.Name] = true
		}

		// Check that standalone functions are extracted
		expectedFunctions := []string{
			"simple_function",
			"decorated_function",
			"decorated_with_args",
			"async_function",
			"async_decorated_function",
			"function_no_types",
			"no_docstring",
		}

		for _, name := range expectedFunctions {
			if !funcMap[name] {
				t.Errorf("Expected function '%s' not found", name)
			}
		}

		// Check that methods are NOT in top-level functions
		notExpected := []string{"base_method", "child_method", "method1", "method2", "nested"}
		for _, name := range notExpected {
			if funcMap[name] {
				t.Errorf("Method '%s' should not be in top-level functions", name)
			}
		}
	})

	// Test async function detection
	t.Run("AsyncFunctions", func(t *testing.T) {
		var asyncFound bool
		for _, fn := range info.Functions {
			if fn.Name == "async_function" && !fn.IsAsync {
				t.Error("async_function should have IsAsync=true")
			}
			if fn.Name == "async_function" && fn.IsAsync {
				asyncFound = true
			}
			if fn.Name == "simple_function" && fn.IsAsync {
				t.Error("simple_function should have IsAsync=false")
			}
		}
		if !asyncFound {
			t.Error("async_function not found or not marked as async")
		}
	})

	// Test decorators
	t.Run("Decorators", func(t *testing.T) {
		for _, fn := range info.Functions {
			if fn.Name == "decorated_function" {
				if len(fn.Decorators) == 0 {
					t.Error("decorated_function should have decorators")
				}
			}
			if fn.Name == "decorated_with_args" {
				if len(fn.Decorators) == 0 {
					t.Error("decorated_with_args should have decorators")
				}
			}
			if fn.Name == "simple_function" && len(fn.Decorators) > 0 {
				t.Error("simple_function should not have decorators")
			}
		}
	})

	// Test docstring extraction
	t.Run("Docstrings", func(t *testing.T) {
		for _, fn := range info.Functions {
			if fn.Name == "simple_function" {
				if fn.Docstring == "" {
					t.Error("simple_function should have a docstring")
				}
				if fn.Docstring != `"""A simple function with type annotations."""` {
					t.Errorf("simple_function has unexpected docstring: %s", fn.Docstring)
				}
			}
			if fn.Name == "no_docstring" && fn.Docstring != "" {
				t.Errorf("no_docstring should not have a docstring, got: %s", fn.Docstring)
			}
		}
	})

	// Test parameter extraction
	t.Run("Parameters", func(t *testing.T) {
		for _, fn := range info.Functions {
			if fn.Name == "simple_function" {
				if fn.Params != "(x: int, y: int)" {
					t.Errorf("simple_function has unexpected params: %s", fn.Params)
				}
			}
			if fn.Name == "function_no_types" {
				if fn.Params != "(a, b)" {
					t.Errorf("function_no_types has unexpected params: %s", fn.Params)
				}
			}
		}
	})

	// Test class extraction
	t.Run("ClassExtraction", func(t *testing.T) {
		if len(info.Classes) == 0 {
			t.Fatal("Expected classes to be extracted")
		}

		classMap := make(map[string]*types.Class)
		for i := range info.Classes {
			classMap[info.Classes[i].Name] = &info.Classes[i]
		}

		// Check that expected classes are extracted
		expectedClasses := []string{
			"BaseClass",
			"ChildClass",
			"MultipleInheritance",
			"DecoratedClass",
			"GenericClass",
			"QualifiedBase",
			"ContainerClass",
		}

		for _, name := range expectedClasses {
			if _, ok := classMap[name]; !ok {
				t.Errorf("Expected class '%s' not found", name)
			}
		}
	})

	// Test class inheritance
	t.Run("ClassInheritance", func(t *testing.T) {
		classMap := make(map[string]*types.Class)
		for i := range info.Classes {
			classMap[info.Classes[i].Name] = &info.Classes[i]
		}

		// Check ChildClass inherits from BaseClass
		if child, ok := classMap["ChildClass"]; ok {
			found := false
			for _, base := range child.Bases {
				if base == "BaseClass" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ChildClass should inherit from BaseClass, got: %v", child.Bases)
			}
		}

		// Check MultipleInheritance has multiple bases
		if mi, ok := classMap["MultipleInheritance"]; ok {
			if len(mi.Bases) < 2 {
				t.Errorf("MultipleInheritance should have at least 2 bases, got: %v", mi.Bases)
			}
		}

		// Check QualifiedBase inherits from os.PathLike
		if qb, ok := classMap["QualifiedBase"]; ok {
			found := false
			for _, base := range qb.Bases {
				if base == "os.PathLike" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("QualifiedBase should inherit from os.PathLike, got: %v", qb.Bases)
			}
		}
	})

	// Test method extraction
	t.Run("MethodExtraction", func(t *testing.T) {
		classMap := make(map[string]*types.Class)
		for i := range info.Classes {
			classMap[info.Classes[i].Name] = &info.Classes[i]
		}

		// Check ContainerClass has methods
		if container, ok := classMap["ContainerClass"]; ok {
			if len(container.Methods) == 0 {
				t.Error("ContainerClass should have methods")
			}

			methodMap := make(map[string]bool)
			for _, m := range container.Methods {
				methodMap[m.Name] = true
				if !m.IsMethod {
					t.Errorf("Method %s should have IsMethod=true", m.Name)
				}
			}

			expectedMethods := []string{"method1", "method2", "nested_function_container"}
			for _, name := range expectedMethods {
				if !methodMap[name] {
					t.Errorf("Expected method '%s' not found in ContainerClass", name)
				}
			}
		}
	})

	// Test line numbers
	t.Run("LineNumbers", func(t *testing.T) {
		for _, fn := range info.Functions {
			if fn.LineNumber == 0 {
				t.Errorf("Function %s should have a line number", fn.Name)
			}
		}

		for _, cls := range info.Classes {
			if cls.LineNumber == 0 {
				t.Errorf("Class %s should have a line number", cls.Name)
			}
		}
	})
}

// TestPythonExtractorEdgeCases tests edge cases
func TestPythonExtractorEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		code     string
		expected int // expected number of top-level functions
	}{
		{
			name:     "empty_file",
			code:     "",
			expected: 0,
		},
		{
			name:     "only_imports",
			code:     "import os\nimport sys\n",
			expected: 0,
		},
		{
			name:     "class_only",
			code:     "class MyClass:\n    pass\n",
			expected: 0,
		},
		{
			name: "nested_functions_not_toplevel",
			code: `def outer():
    def inner():
        pass
    return inner
`,
			expected: 1, // only outer should be extracted
		},
		{
			name:     "lambda_not_extracted",
			code:     "f = lambda x: x + 1\n",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pyFile := filepath.Join(tmpDir, tt.name+".py")
			err := os.WriteFile(pyFile, []byte(tt.code), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			extractor := NewPythonExtractor()
			info, err := extractor.Extract(pyFile)
			if err != nil {
				t.Fatalf("Extract() failed: %v", err)
			}

			if len(info.Functions) != tt.expected {
				t.Errorf("Expected %d functions, got %d", tt.expected, len(info.Functions))
			}
		})
	}
}

// TestExtractFromBytes tests extraction directly from bytes
func TestExtractFromBytes(t *testing.T) {
	pythonCode := []byte(`
def hello():
    """Say hello."""
    return "hello"

class Greeter:
    """A greeter class."""
    
    def greet(self, name: str) -> str:
        return f"Hello, {name}!"
`)

	extractor := NewPythonExtractor().(*PythonExtractor)
	info, err := extractor.ExtractFromBytes(pythonCode, "test.py")
	if err != nil {
		t.Fatalf("ExtractFromBytes() failed: %v", err)
	}

	if len(info.Functions) != 1 {
		t.Errorf("Expected 1 function, got %d", len(info.Functions))
	}

	if len(info.Classes) != 1 {
		t.Errorf("Expected 1 class, got %d", len(info.Classes))
	}

	if info.Path != "test.py" {
		t.Errorf("Expected path 'test.py', got '%s'", info.Path)
	}
}

// TestIsAsyncFunction tests the IsAsyncFunction helper
func TestIsAsyncFunction(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "async_test.py")

	pythonCode := `
def regular():
    pass

async def async_func():
    pass
`
	err := os.WriteFile(pyFile, []byte(pythonCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extractor := NewPythonExtractor().(*PythonExtractor)

	// Parse the file to get AST
	content, _ := os.ReadFile(pyFile)
	tree := extractor.parser.Parse(nil, content)
	defer tree.Close()
	root := tree.RootNode()

	// Walk the AST to find function nodes
	var foundAsync, foundRegular bool
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child != nil && child.Type() == "function_definition" {
			name := ""
			for j := 0; j < int(child.ChildCount()); j++ {
				if child.Child(j).Type() == "identifier" {
					name = string(content[child.Child(j).StartByte():child.Child(j).EndByte()])
					break
				}
			}

			isAsync := extractor.IsAsyncFunction(child)
			if name == "async_func" {
				if !isAsync {
					t.Error("IsAsyncFunction should return true for async_func")
				}
				foundAsync = true
			}
			if name == "regular" {
				if isAsync {
					t.Error("IsAsyncFunction should return false for regular")
				}
				foundRegular = true
			}
		}
	}

	if !foundAsync {
		t.Error("Did not find async_func in AST")
	}
	if !foundRegular {
		t.Error("Did not find regular in AST")
	}
}
