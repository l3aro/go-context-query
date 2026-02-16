package extractor

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractorInterface verifies the Extractor interface is properly defined
func TestExtractorInterface(t *testing.T) {
	// Test that PythonExtractor implements Extractor interface
	var _ Extractor = NewPythonExtractor()
}

// TestLanguageRegistry tests the language registry functionality
func TestLanguageRegistry(t *testing.T) {
	registry := NewLanguageRegistry()

	// Test Python extension mapping
	tests := []struct {
		ext      string
		expected Language
		wantErr  bool
	}{
		{"test.py", Python, false},
		{"test.pyw", Python, false},
		{"test.pyi", Python, false},
		{"test.go", Go, false},
		{"test.ts", TypeScript, false},
		{"test.js", JavaScript, false},
		{"test", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			lang, err := registry.GetLanguage(tt.ext)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLanguage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && lang != tt.expected {
				t.Errorf("GetLanguage() = %v, want %v", lang, tt.expected)
			}
		})
	}
}

// TestIsSupported tests the IsSupported method
func TestIsSupported(t *testing.T) {
	registry := NewLanguageRegistry()

	tests := []struct {
		filePath string
		expected bool
	}{
		{"test.py", true},
		{"/path/to/file.py", true},
		{"test.pyw", true},
		{"test.go", true},
		{"test.ts", true},
		{"test.js", true},
		{"test", false},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got := registry.IsSupported(tt.filePath)
			if got != tt.expected {
				t.Errorf("IsSupported() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestExtractPythonFile tests extracting information from a Python file
func TestExtractPythonFile(t *testing.T) {
	// Create a temporary Python file
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test_module.py")

	pythonCode := `
"""Test module docstring."""

import os
import sys
from pathlib import Path

def standalone_function(x, y):
    """A standalone function."""
    return x + y

async def async_function():
    """An async function."""
    pass

class TestClass:
    """A test class."""
    
    def __init__(self):
        pass
    
    def method(self, arg):
        """A class method."""
        return arg
`

	err := os.WriteFile(pyFile, []byte(pythonCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test extraction
	extractor := NewPythonExtractor()
	info, err := extractor.Extract(pyFile)
	if err != nil {
		t.Fatalf("Extract() failed: %v", err)
	}

	// Verify basic info
	if info.Path != pyFile {
		t.Errorf("Path = %v, want %v", info.Path, pyFile)
	}

	// Verify imports were extracted
	if len(info.Imports) == 0 {
		t.Error("Expected imports to be extracted")
	}

	// Check for specific imports
	importModules := make(map[string]bool)
	for _, imp := range info.Imports {
		importModules[imp.Module] = true
	}

	expectedImports := []string{"os", "sys", "pathlib"}
	for _, mod := range expectedImports {
		if !importModules[mod] {
			t.Errorf("Expected import %s not found", mod)
		}
	}

	// Verify functions were extracted
	if len(info.Functions) == 0 {
		t.Error("Expected functions to be extracted")
	}

	// Check for standalone functions
	functionNames := make(map[string]bool)
	for _, fn := range info.Functions {
		functionNames[fn.Name] = true
		if fn.IsMethod {
			t.Errorf("Expected standalone function, got method: %s", fn.Name)
		}
	}

	if !functionNames["standalone_function"] {
		t.Error("Expected 'standalone_function' to be extracted")
	}

	if !functionNames["async_function"] {
		t.Error("Expected 'async_function' to be extracted")
	}

	// Verify classes were extracted
	if len(info.Classes) == 0 {
		t.Error("Expected classes to be extracted")
	}

	foundTestClass := false
	for _, cls := range info.Classes {
		if cls.Name == "TestClass" {
			foundTestClass = true
			if cls.Docstring == "" {
				t.Error("Expected TestClass to have a docstring")
			}
			// Verify methods were extracted
			if len(cls.Methods) == 0 {
				t.Error("Expected TestClass to have methods")
			}
			for _, method := range cls.Methods {
				if !method.IsMethod {
					t.Errorf("Expected %s to be marked as a method", method.Name)
				}
			}
		}
	}

	if !foundTestClass {
		t.Error("Expected 'TestClass' to be extracted")
	}
}

// TestExtractFileWithRegistry tests the convenience ExtractFile function
func TestExtractFileWithRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test.py")

	pythonCode := `def hello(): pass`

	err := os.WriteFile(pyFile, []byte(pythonCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	info, err := ExtractFile(pyFile)
	if err != nil {
		t.Fatalf("ExtractFile() failed: %v", err)
	}

	if info.Path != pyFile {
		t.Errorf("Path = %v, want %v", info.Path, pyFile)
	}
}

// TestExtractUnsupportedFile tests extraction of unsupported file types
func TestExtractUnsupportedFile(t *testing.T) {
	tmpDir := t.TempDir()
	unsupportedFile := filepath.Join(tmpDir, "test.xyz")

	err := os.WriteFile(unsupportedFile, []byte("unsupported content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = ExtractFile(unsupportedFile)
	if err == nil {
		t.Error("Expected error for unsupported file type, got nil")
	}
}

// TestNewPythonParser tests the parser factory
func TestNewPythonParser(t *testing.T) {
	parser := NewPythonParser()
	if parser == nil {
		t.Fatal("NewPythonParser() returned nil")
	}
}

// TestGetParser tests getting a parser from the registry
func TestGetParser(t *testing.T) {
	registry := NewLanguageRegistry()

	parser, err := registry.GetParser("test.py")
	if err != nil {
		t.Fatalf("GetParser() failed: %v", err)
	}
	if parser == nil {
		t.Error("GetParser() returned nil parser")
	}

	_, err = registry.GetParser("test.unknown")
	if err == nil {
		t.Error("Expected error for unsupported file type")
	}
}

// TestGetSupportedExtensions tests getting supported extensions
func TestGetSupportedExtensions(t *testing.T) {
	registry := NewLanguageRegistry()
	extensions := registry.GetSupportedExtensions()

	if len(extensions) == 0 {
		t.Error("Expected supported extensions to be returned")
	}

	// Check that Python extensions are included
	hasPy := false
	for _, ext := range extensions {
		if ext == ".py" {
			hasPy = true
			break
		}
	}

	if !hasPy {
		t.Error("Expected .py to be in supported extensions")
	}
}

// TestGetExtractor tests getting an extractor from the registry
func TestGetExtractor(t *testing.T) {
	registry := NewLanguageRegistry()

	extractor, err := registry.GetExtractor("test.py")
	if err != nil {
		t.Fatalf("GetExtractor() failed: %v", err)
	}
	if extractor == nil {
		t.Error("GetExtractor() returned nil extractor")
	}

	_, err = registry.GetExtractor("test.unknown")
	if err == nil {
		t.Error("Expected error for unsupported file type")
	}
}
