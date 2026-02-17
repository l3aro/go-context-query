package callgraph

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestIsPythonFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"py file", "main.py", true},
		{"pyi file", "stub.pyi", true},
		{"pyw file", "gui.pyw", true},
		{"pyc file", "compiled.pyc", false},
		{"pyo file", "optimized.pyo", false},
		{"txt file", "readme.txt", false},
		{"go file", "main.go", false},
		{"uppercase PY", "MAIN.PY", true},
		{"path with dir", "src/main.py", true},
		{"no extension", "Makefile", false},
		{"dot file", ".gitignore", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPythonFile(tt.path)
			if result != tt.expected {
				t.Errorf("IsPythonFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsCompiledPython(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"pyc file", "compiled.pyc", true},
		{"pyo file", "optimized.pyo", true},
		{"pyd file", "module.pyd", true},
		{"py file", "main.py", false},
		{"pyi file", "stub.pyi", false},
		{"uppercase PYC", "COMPILED.PYC", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCompiledPython(tt.path)
			if result != tt.expected {
				t.Errorf("IsCompiledPython(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsInPycache(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"in pycache", "__pycache__/module.cpython-39.pyc", true},
		{"in nested pycache", "src/__pycache__/utils.cpython-39.pyc", true},
		{"not in pycache", "src/utils.py", false},
		{"pycache as filename", "__pycache__.py", false},
		{"windows path", "src\\__pycache__\\module.pyc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsInPycache(tt.path)
			if result != tt.expected {
				t.Errorf("IsInPycache(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"test prefix", "test_utils.py", true},
		{"test suffix", "utils_test.py", true},
		{"conftest", "conftest.py", true},
		{"not a test", "utils.py", false},
		{"main file", "main.py", false},
		{"nested test", "tests/test_utils.py", true},
		{"windows path", "tests\\test_utils.py", true},
		{"pytest file", "test_something.py", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTestFile(tt.path)
			if result != tt.expected {
				t.Errorf("IsTestFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestScanProject(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"main.py":                           "print('hello')",
		"utils.py":                          "def helper(): pass",
		"lib/core.py":                       "def core(): pass",
		"lib/__init__.py":                   "",
		"test_main.py":                      "def test_main(): pass",
		"tests/test_utils.py":               "def test_utils(): pass",
		"__pycache__/module.cpython-39.pyc": "compiled",
		".hidden/module.py":                 "# hidden",
		"readme.md":                         "# README",
		"main.go":                           "package main",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	// Test scanning with default options (excluding tests)
	result, err := ScanProject(tmpDir, "python")
	if err != nil {
		t.Fatalf("ScanProject failed: %v", err)
	}

	// Sort for consistent comparison
	sort.Strings(result)

	// Expected: main.py, utils.py, lib/core.py, lib/__init__.py
	// Excluded: test files, __pycache__, .hidden, non-python files
	expected := []string{"lib/__init__.py", "lib/core.py", "main.py", "utils.py"}

	if len(result) != len(expected) {
		t.Errorf("Expected %d files, got %d: %v", len(expected), len(result), result)
	}

	for i, exp := range expected {
		if i >= len(result) || result[i] != exp {
			t.Errorf("Expected file %d to be %q, got %q", i, exp, result[i])
		}
	}
}

func TestScanProjectWithOptions(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"main.py":             "print('hello')",
		"test_main.py":        "def test_main(): pass",
		"tests/test_utils.py": "def test_utils(): pass",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	// Test with IncludeTestFiles = false
	opts := ScanOptions{IncludeTestFiles: false}
	result, err := ScanProjectWithOptions(tmpDir, "python", opts)
	if err != nil {
		t.Fatalf("ScanProjectWithOptions failed: %v", err)
	}

	if len(result) != 1 || result[0] != "main.py" {
		t.Errorf("Expected [main.py], got %v", result)
	}

	// Test with IncludeTestFiles = true
	opts = ScanOptions{IncludeTestFiles: true}
	result, err = ScanProjectWithOptions(tmpDir, "python", opts)
	if err != nil {
		t.Fatalf("ScanProjectWithOptions failed: %v", err)
	}

	sort.Strings(result)
	expected := []string{"main.py", "test_main.py", "tests/test_utils.py"}
	if len(result) != len(expected) {
		t.Errorf("Expected %d files, got %d: %v", len(expected), len(result), result)
	}
}

func TestScanProjectUnsupportedLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := ScanProject(tmpDir, "javascript")
	if err == nil {
		t.Error("Expected error for unsupported language, got nil")
	}

	if err.Error() != "unsupported language: javascript (only 'python' is supported)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestScanProjectRespectsGcqignore(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"main.py":         "print('hello')",
		"vendor/lib.py":   "# vendor code",
		"build/output.py": "# build output",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	// Create .gcqignore file
	gcqignoreContent := `vendor/
build/
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gcqignore"), []byte(gcqignoreContent), 0644); err != nil {
		t.Fatalf("Failed to create .gcqignore: %v", err)
	}

	// Test scanning
	result, err := ScanProject(tmpDir, "python")
	if err != nil {
		t.Fatalf("ScanProject failed: %v", err)
	}

	// Should only have main.py
	if len(result) != 1 || result[0] != "main.py" {
		t.Errorf("Expected [main.py], got %v", result)
	}
}

func TestScanProjectAllLanguages(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create test files of various languages
	files := map[string]string{
		"main.py":   "print('hello')",
		"utils.go":  "package utils",
		"app.js":    "console.log('hello')",
		"README.md": "# README",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	// Scan all languages
	result, err := ScanProjectAllLanguages(tmpDir)
	if err != nil {
		t.Fatalf("ScanProjectAllLanguages failed: %v", err)
	}

	// Check that we have files for each language
	if _, ok := result["python"]; !ok {
		t.Error("Expected python files in result")
	}
	if _, ok := result["go"]; !ok {
		t.Error("Expected go files in result")
	}
	if _, ok := result["javascript"]; !ok {
		t.Error("Expected javascript files in result")
	}
	if _, ok := result["markdown"]; !ok {
		t.Error("Expected markdown files in result")
	}

	// Verify specific files
	if len(result["python"]) != 1 || result["python"][0] != "main.py" {
		t.Errorf("Expected [main.py] for python, got %v", result["python"])
	}
}

func TestDefaultScanOptions(t *testing.T) {
	opts := DefaultScanOptions()

	if opts.IncludeTestFiles {
		t.Error("Expected IncludeTestFiles to be false by default")
	}

	if opts.ExcludePatterns != nil {
		t.Error("Expected ExcludePatterns to be nil by default")
	}
}

func TestScanProjectEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := ScanProject(tmpDir, "python")
	if err != nil {
		t.Fatalf("ScanProject failed on empty directory: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result for empty directory, got %v", result)
	}
}

func TestScanProjectPythonExtensions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with different Python extensions
	files := []string{
		"main.py",
		"stub.pyi",
		"gui.pyw",
		"compiled.pyc",
		"optimized.pyo",
	}

	for _, filename := range files {
		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	result, err := ScanProject(tmpDir, "python")
	if err != nil {
		t.Fatalf("ScanProject failed: %v", err)
	}

	sort.Strings(result)

	// Should have .py, .pyi, .pyw but not .pyc, .pyo
	expected := []string{"gui.pyw", "main.py", "stub.pyi"}
	if len(result) != len(expected) {
		t.Errorf("Expected %d files, got %d: %v", len(expected), len(result), result)
	}

	for i, exp := range expected {
		if i >= len(result) || result[i] != exp {
			t.Errorf("Expected file %d to be %q, got %q", i, exp, result[i])
		}
	}
}
