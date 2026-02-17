// Package callgraph provides cross-file project indexing functionality.
package callgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewProjectIndex verifies the creation of a new project index.
func TestNewProjectIndex(t *testing.T) {
	tmpDir := t.TempDir()
	idx := NewProjectIndex(tmpDir)

	if idx == nil {
		t.Fatal("NewProjectIndex returned nil")
	}

	if idx.rootDir != tmpDir {
		t.Errorf("expected rootDir %s, got %s", tmpDir, idx.rootDir)
	}

	if idx.funcToFile == nil {
		t.Error("funcToFile map not initialized")
	}

	if idx.entries == nil {
		t.Error("entries map not initialized")
	}

	if idx.fileToFunctions == nil {
		t.Error("fileToFunctions map not initialized")
	}

	if idx.parsedFiles == nil {
		t.Error("parsedFiles map not initialized")
	}
}

// TestProjectIndex_BuildIndex tests building the index from Python files.
func TestProjectIndex_BuildIndex(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test Python files
	createTestFile(t, tmpDir, "module_a.py", `
def function_a():
    pass

class MyClass:
    def method_a(self):
        pass
`)

	createTestFile(t, tmpDir, "module_b.py", `
def function_b():
    pass

class AnotherClass:
    def method_b(self):
        pass
`)

	createTestFile(t, tmpDir, "pkg", "utils.py", `
def helper():
    pass

def nested_outer():
    def inner():
        pass
    return inner
`)

	idx := NewProjectIndex(tmpDir)

	files := []string{
		filepath.Join(tmpDir, "module_a.py"),
		filepath.Join(tmpDir, "module_b.py"),
		filepath.Join(tmpDir, "pkg", "utils.py"),
	}

	err := idx.BuildIndex(files)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	stats := idx.GetStats()
	if stats.TotalFiles != 3 {
		t.Errorf("expected 3 files indexed, got %d", stats.TotalFiles)
	}

	if stats.ParsedFiles != 3 {
		t.Errorf("expected 3 parsed files, got %d", stats.ParsedFiles)
	}
}

// TestProjectIndex_Lookup tests the Lookup method.
func TestProjectIndex_Lookup(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "test_module.py", `
def standalone_func():
    pass

class MyClass:
    def my_method(self):
        pass
`)

	idx := NewProjectIndex(tmpDir)
	files := []string{filepath.Join(tmpDir, "test_module.py")}

	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	tests := []struct {
		name         string
		funcName     string
		shouldExist  bool
		expectedPath string
	}{
		{
			name:         "simple function name",
			funcName:     "standalone_func",
			shouldExist:  true,
			expectedPath: filepath.Join(tmpDir, "test_module.py"),
		},
		{
			name:         "class method",
			funcName:     "MyClass.my_method",
			shouldExist:  true,
			expectedPath: filepath.Join(tmpDir, "test_module.py"),
		},
		{
			name:        "non-existent function",
			funcName:    "non_existent",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, found := idx.Lookup(tt.funcName)

			if tt.shouldExist && !found {
				t.Errorf("expected to find %s, but didn't", tt.funcName)
			}

			if !tt.shouldExist && found {
				t.Errorf("expected not to find %s, but found it at %s", tt.funcName, path)
			}

			if tt.shouldExist && found && path != tt.expectedPath {
				t.Errorf("expected path %s, got %s", tt.expectedPath, path)
			}
		})
	}
}

// TestProjectIndex_LookupWithModule tests the LookupWithModule method.
func TestProjectIndex_LookupWithModule(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "pkg", "utils.py", `
def helper_func():
    pass

class HelperClass:
    def do_something(self):
        pass
`)

	idx := NewProjectIndex(tmpDir)
	files := []string{filepath.Join(tmpDir, "pkg", "utils.py")}

	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	tests := []struct {
		name         string
		moduleName   string
		funcName     string
		shouldExist  bool
		expectedPath string
	}{
		{
			name:         "module and function",
			moduleName:   "pkg.utils",
			funcName:     "helper_func",
			shouldExist:  true,
			expectedPath: filepath.Join(tmpDir, "pkg", "utils.py"),
		},
		{
			name:         "module and class method",
			moduleName:   "pkg.utils",
			funcName:     "HelperClass.do_something",
			shouldExist:  true,
			expectedPath: filepath.Join(tmpDir, "pkg", "utils.py"),
		},
		{
			name:        "non-existent module",
			moduleName:  "nonexistent",
			funcName:    "func",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, found := idx.LookupWithModule(tt.moduleName, tt.funcName)

			if tt.shouldExist && !found {
				t.Errorf("expected to find %s.%s, but didn't", tt.moduleName, tt.funcName)
			}

			if !tt.shouldExist && found {
				t.Errorf("expected not to find %s.%s, but found it at %s", tt.moduleName, tt.funcName, path)
			}

			if tt.shouldExist && found && path != tt.expectedPath {
				t.Errorf("expected path %s, got %s", tt.expectedPath, path)
			}
		})
	}
}

// TestProjectIndex_LookupEntry tests the LookupEntry method.
func TestProjectIndex_LookupEntry(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "test.py", `
def test_func():
    """A test function."""
    pass
`)

	idx := NewProjectIndex(tmpDir)
	files := []string{filepath.Join(tmpDir, "test.py")}

	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	entry, found := idx.LookupEntry("test.test_func")
	if !found {
		t.Fatal("expected to find entry for test.test_func")
	}

	if entry.Name != "test_func" {
		t.Errorf("expected Name to be 'test_func', got %s", entry.Name)
	}

	if entry.ModuleName != "test" {
		t.Errorf("expected ModuleName to be 'test', got %s", entry.ModuleName)
	}

	if !strings.HasSuffix(entry.FilePath, "test.py") {
		t.Errorf("expected FilePath to end with 'test.py', got %s", entry.FilePath)
	}
}

// TestProjectIndex_GetFunctionsInFile tests retrieving functions by file.
func TestProjectIndex_GetFunctionsInFile(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "multi_func.py", `
def func_a():
    pass

def func_b():
    pass

class MyClass:
    def method_x(self):
        pass
`)

	idx := NewProjectIndex(tmpDir)
	filePath := filepath.Join(tmpDir, "multi_func.py")
	files := []string{filePath}

	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	funcs := idx.GetFunctionsInFile(filePath)
	if len(funcs) == 0 {
		t.Fatal("expected to find functions in file")
	}

	// Should have at least func_a, func_b, MyClass, MyClass.method_x, method_x
	if len(funcs) < 4 {
		t.Errorf("expected at least 4 functions, got %d: %v", len(funcs), funcs)
	}
}

// TestProjectIndex_GetModuleFile tests retrieving file by module name.
func TestProjectIndex_GetModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "deep", "nested", "module.py", `
def deep_func():
    pass
`)

	idx := NewProjectIndex(tmpDir)
	files := []string{filepath.Join(tmpDir, "deep", "nested", "module.py")}

	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	filePath, found := idx.GetModuleFile("deep.nested.module")
	if !found {
		t.Fatal("expected to find module file")
	}

	expectedPath := filepath.Join(tmpDir, "deep", "nested", "module.py")
	if filePath != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, filePath)
	}
}

// TestProjectIndex_GetStats tests index statistics.
func TestProjectIndex_GetStats(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "stats_test.py", `
def func1():
    pass

def func2():
    pass

class TestClass:
    def method1(self):
        pass
    
    def method2(self):
        pass
`)

	idx := NewProjectIndex(tmpDir)
	files := []string{filepath.Join(tmpDir, "stats_test.py")}

	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	stats := idx.GetStats()

	if stats.TotalFiles != 1 {
		t.Errorf("expected 1 file, got %d", stats.TotalFiles)
	}

	if stats.TotalModules != 1 {
		t.Errorf("expected 1 module, got %d", stats.TotalModules)
	}

	if stats.ParsedFiles != 1 {
		t.Errorf("expected 1 parsed file, got %d", stats.ParsedFiles)
	}

	// Should have multiple entries (func1, func2, TestClass, TestClass.method1, etc.)
	if stats.TotalFunctions < 5 {
		t.Errorf("expected at least 5 functions, got %d", stats.TotalFunctions)
	}
}

// TestProjectIndex_AddNestedFunction tests adding nested functions.
func TestProjectIndex_AddNestedFunction(t *testing.T) {
	tmpDir := t.TempDir()
	idx := NewProjectIndex(tmpDir)

	filePath := filepath.Join(tmpDir, "test.py")
	idx.AddNestedFunction(filePath, "test", "outer_func", "inner_func", 10)

	// Lookup by qualified name
	path, found := idx.Lookup("outer_func.inner_func")
	if !found {
		t.Fatal("expected to find nested function")
	}
	if path != filePath {
		t.Errorf("expected path %s, got %s", filePath, path)
	}

	// Lookup with module
	path, found = idx.LookupWithModule("test", "outer_func.inner_func")
	if !found {
		t.Fatal("expected to find nested function with module")
	}

	// Lookup entry
	entry, found := idx.LookupEntry("test.outer_func.inner_func")
	if !found {
		t.Fatal("expected to find entry")
	}
	if !entry.IsNested {
		t.Error("expected IsNested to be true")
	}
	if entry.ParentName != "outer_func" {
		t.Errorf("expected ParentName to be 'outer_func', got %s", entry.ParentName)
	}
}

// TestProjectIndex_RefreshFile tests refreshing a file in the index.
func TestProjectIndex_RefreshFile(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "refresh_test.py")
	createTestFile(t, tmpDir, "refresh_test.py", `
def old_func():
    pass
`)

	idx := NewProjectIndex(tmpDir)
	files := []string{filePath}

	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	// Verify old_func exists
	_, found := idx.Lookup("old_func")
	if !found {
		t.Fatal("expected to find old_func")
	}

	// Update file content
	content := `
def new_func():
    pass
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Refresh the file
	if err := idx.RefreshFile(filePath); err != nil {
		t.Fatalf("RefreshFile failed: %v", err)
	}

	// old_func should be gone
	_, found = idx.Lookup("old_func")
	if found {
		t.Error("expected old_func to be removed after refresh")
	}

	// new_func should exist
	_, found = idx.Lookup("new_func")
	if !found {
		t.Error("expected to find new_func after refresh")
	}
}

// TestProjectIndex_IsIndexed tests checking if a file is indexed.
func TestProjectIndex_IsIndexed(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "indexed.py", `
def func():
    pass
`)

	idx := NewProjectIndex(tmpDir)
	filePath := filepath.Join(tmpDir, "indexed.py")

	// File should not be indexed initially
	if idx.IsIndexed(filePath) {
		t.Error("expected file to not be indexed initially")
	}

	// Index the file
	files := []string{filePath}
	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	// File should now be indexed
	if !idx.IsIndexed(filePath) {
		t.Error("expected file to be indexed after BuildIndex")
	}
}

// TestProjectIndex_BuildIndexFromScan tests building index from scan.
func TestProjectIndex_BuildIndexFromScan(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "scanned.py", `
def scanned_func():
    pass
`)

	createTestFile(t, tmpDir, "scanned_test.py", `
def test_func():
    pass
`)

	idx := NewProjectIndex(tmpDir)

	// Build from scan (should exclude test files by default)
	if err := idx.BuildIndexFromScan("python"); err != nil {
		t.Fatalf("BuildIndexFromScan failed: %v", err)
	}

	// scanned_func should be found
	_, found := idx.Lookup("scanned_func")
	if !found {
		t.Error("expected to find scanned_func")
	}

	// test_func should NOT be found (it's a test file)
	_, found = idx.Lookup("test_func")
	if found {
		t.Error("expected test_func to be excluded (test file)")
	}
}

// TestProjectIndex_SkipsReParsing tests that already parsed files are skipped.
func TestProjectIndex_SkipsReParsing(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "skip.py", `
def skip_func():
    pass
`)

	idx := NewProjectIndex(tmpDir)
	filePath := filepath.Join(tmpDir, "skip.py")
	files := []string{filePath}

	// First build
	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	stats1 := idx.GetStats()

	// Second build (should skip already parsed files)
	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	stats2 := idx.GetStats()

	// Stats should be the same (no new files parsed)
	if stats2.TotalFunctions != stats1.TotalFunctions {
		t.Errorf("expected same function count, got %d vs %d", stats1.TotalFunctions, stats2.TotalFunctions)
	}

	if stats2.ParsedFiles != stats1.ParsedFiles {
		t.Errorf("expected same parsed file count, got %d vs %d", stats1.ParsedFiles, stats2.ParsedFiles)
	}
}

// TestProjectIndex_ClassMethods tests proper handling of class methods.
func TestProjectIndex_ClassMethods(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "class_test.py", `
class Calculator:
    def add(self, a, b):
        return a + b
    
    def subtract(self, a, b):
        return a - b

class Printer:
    def print_value(self, value):
        print(value)
`)

	idx := NewProjectIndex(tmpDir)
	files := []string{filepath.Join(tmpDir, "class_test.py")}

	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	tests := []struct {
		name        string
		funcName    string
		shouldExist bool
	}{
		{"class as type", "Calculator", true},
		{"class method simple", "add", true},
		{"class method simple 2", "subtract", true},
		{"qualified method", "Calculator.add", true},
		{"qualified method 2", "Calculator.subtract", true},
		{"another class", "Printer", true},
		{"another class method", "print_value", true},
		{"another qualified method", "Printer.print_value", true},
		{"non-existent method", "Calculator.multiply", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found := idx.Lookup(tt.funcName)
			if tt.shouldExist && !found {
				t.Errorf("expected to find %s", tt.funcName)
			}
			if !tt.shouldExist && found {
				t.Errorf("expected not to find %s", tt.funcName)
			}
		})
	}
}

// TestProjectIndex_ConcurrentAccess tests thread-safe read access.
func TestProjectIndex_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()

	createTestFile(t, tmpDir, "concurrent.py", `
def func1():
    pass

def func2():
    pass
`)

	idx := NewProjectIndex(tmpDir)
	files := []string{filepath.Join(tmpDir, "concurrent.py")}

	if err := idx.BuildIndex(files); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	// Run multiple concurrent lookups
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				idx.Lookup("func1")
				idx.Lookup("func2")
				idx.GetStats()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// createTestFile creates a test file with the given content.
// It handles nested directories automatically.
func createTestFile(t *testing.T, baseDir string, parts ...string) string {
	t.Helper()

	if len(parts) < 2 {
		t.Fatal("createTestFile requires at least directory/filename and content")
	}

	content := parts[len(parts)-1]
	pathParts := parts[:len(parts)-1]

	fullPath := filepath.Join(append([]string{baseDir}, pathParts...)...)

	// Create directories if needed
	dir := filepath.Dir(fullPath)
	if dir != baseDir {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directories: %v", err)
		}
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	return fullPath
}
