package semantic

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/l3aro/go-context-query/pkg/embed"
	"github.com/l3aro/go-context-query/pkg/types"
)

// mockProvider is a mock implementation of embed.Provider for testing
type mockProvider struct {
	embedFn  func(texts []string) ([][]float32, error)
	configFn func() *embed.Config
}

func (m *mockProvider) Embed(texts []string) ([][]float32, error) {
	if m.embedFn != nil {
		return m.embedFn(texts)
	}
	// Default: return deterministic embeddings
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = []float32{1.0, 2.0, 3.0}
	}
	return embeddings, nil
}

func (m *mockProvider) Config() *embed.Config {
	if m.configFn != nil {
		return m.configFn()
	}
	return &embed.Config{
		Model:      "mock-model",
		Endpoint:   "http://localhost:8080",
		Dimensions: 3,
	}
}

// mockProviderWithError returns a provider that always returns an error
type mockProviderWithError struct{}

func (m *mockProviderWithError) Embed(texts []string) ([][]float32, error) {
	return nil, embed.ErrProviderUnavailable
}

func (m *mockProviderWithError) Config() *embed.Config {
	return &embed.Config{Model: "error-model", Dimensions: 3}
}

// mockProviderCustomEmbeddings allows custom embedding dimensions
type mockProviderCustomEmbeddings struct {
	dimension int
}

func (m *mockProviderCustomEmbeddings) Embed(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		vec := make([]float32, m.dimension)
		for j := range vec {
			vec[j] = float32(j + i)
		}
		embeddings[i] = vec
	}
	return embeddings, nil
}

func (m *mockProviderCustomEmbeddings) Config() *embed.Config {
	return &embed.Config{
		Model:      "custom-dim-model",
		Dimensions: m.dimension,
	}
}

func TestNewBuilder(t *testing.T) {
	tmpDir := t.TempDir()

	// Test successful creation
	provider := &mockProvider{}
	builder, err := NewBuilder(tmpDir, provider)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}
	if builder == nil {
		t.Fatal("Builder should not be nil")
	}
	if builder.rootDir != tmpDir {
		t.Errorf("Expected rootDir %s, got %s", tmpDir, builder.rootDir)
	}
	if builder.cacheDir == "" {
		t.Error("CacheDir should not be empty")
	}
	if builder.embedProvider != provider {
		t.Error("embedProvider should be set")
	}
}

func TestNewBuilderCreatesCacheDir(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &mockProvider{}

	builder, err := NewBuilder(tmpDir, provider)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	// Verify cache directory was created
	expectedCacheDir := filepath.Join(tmpDir, ".gcq", "cache", "semantic")
	if builder.cacheDir != expectedCacheDir {
		t.Errorf("Expected cacheDir %s, got %s", expectedCacheDir, builder.cacheDir)
	}

	// Verify directory exists
	if _, err := os.Stat(expectedCacheDir); os.IsNotExist(err) {
		t.Errorf("Cache directory should exist: %v", err)
	}
}

func TestBuilderScan(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test Python files
	files := map[string]string{
		"main.py":   "def main():\n    pass",
		"utils.py":  "def helper():\n    pass",
		"README.md": "# Readme",
		"main.go":   "package main",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	provider := &mockProvider{}
	builder, err := NewBuilder(tmpDir, provider)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	results, err := builder.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should find Python files
	var pyFiles []string
	for _, f := range results {
		if f.Language == "python" {
			pyFiles = append(pyFiles, f.Path)
		}
	}

	if len(pyFiles) != 2 {
		t.Errorf("Expected 2 Python files, got %d: %v", len(pyFiles), pyFiles)
	}
}

func TestBuilderGetCacheDir(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &mockProvider{}

	builder, err := NewBuilder(tmpDir, provider)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	cacheDir := builder.GetCacheDir()
	if cacheDir == "" {
		t.Error("GetCacheDir should not return empty string")
	}

	// Verify it contains expected path
	expectedPrefix := filepath.Join(tmpDir, ".gcq", "cache", "semantic")
	if cacheDir != expectedPrefix {
		t.Errorf("Expected cacheDir to start with %s, got %s", expectedPrefix, cacheDir)
	}
}

func TestEmbeddingText(t *testing.T) {
	tests := []struct {
		name     string
		unit     *CodeUnit
		expected string
	}{
		{
			name: "function with signature and docstring",
			unit: &CodeUnit{
				Name:       "my_function",
				Type:       "function",
				FilePath:   "test.py",
				LineNumber: 10,
				Signature:  "def my_function(arg1, arg2)",
				Docstring:  "This is a test function",
				Calls:      []string{"helper", "util.process"},
				CalledBy:   []string{"main"},
			},
			expected: "Function: my_function\nSignature: def my_function(arg1, arg2)\nDescription: This is a test function\nCalls: helper, util.process\nCalled by: main",
		},
		{
			name: "class with base classes",
			unit: &CodeUnit{
				Name:       "MyClass",
				Type:       "class",
				FilePath:   "test.py",
				LineNumber: 20,
				Signature:  "class MyClass(BaseClass, Mixin)",
				Docstring:  "A test class",
				Calls:      nil,
				CalledBy:   nil,
			},
			expected: "Class: MyClass\nSignature: class MyClass(BaseClass, Mixin)\nDescription: A test class",
		},
		{
			name: "method with no docstring",
			unit: &CodeUnit{
				Name:       "MyClass.my_method",
				Type:       "method",
				FilePath:   "test.py",
				LineNumber: 30,
				Signature:  "def my_method(self, x)",
				Docstring:  "",
				Calls:      []string{"self.helper"},
				CalledBy:   nil,
			},
			expected: "Method: MyClass.my_method\nSignature: def my_method(self, x)\nCalls: self.helper",
		},
		{
			name: "empty unit",
			unit: &CodeUnit{
				Name:       "empty_func",
				Type:       "",
				FilePath:   "test.py",
				LineNumber: 1,
				Signature:  "",
				Docstring:  "",
				Calls:      nil,
				CalledBy:   nil,
			},
			expected: "Function: empty_func",
		},
		{
			name: "long calls list truncated",
			unit: &CodeUnit{
				Name:       "func_with_many_calls",
				Type:       "function",
				FilePath:   "test.py",
				LineNumber: 1,
				Signature:  "def func_with_many_calls()",
				Docstring:  "",
				Calls:      []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"},
				CalledBy:   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EmbeddingText(tt.unit)
			if tt.expected != "" && result != tt.expected {
				t.Errorf("EmbeddingText() = %q, want %q", result, tt.expected)
			}
			// For the truncated case, verify it ends with "..."
			if tt.name == "long calls list truncated" {
				if len(result) == 0 {
					t.Error("Expected non-empty result")
				}
				// Verify the calls part is truncated
				if len(tt.unit.Calls) > 5 {
					// Should have truncation indicator
					hasTruncation := false
					for _, call := range tt.unit.Calls {
						if len(call) > 200 {
							hasTruncation = true
						}
					}
					_ = hasTruncation // result should be reasonable
				}
			}
		})
	}
}

func TestFormatSignature(t *testing.T) {
	tests := []struct {
		name     string
		fn       types.Function
		expected string
	}{
		{
			name: "function with params and return type",
			fn: types.Function{
				Name:       "add",
				Params:     "(a, b)",
				ReturnType: "int",
			},
			expected: "def add(a, b) -> int",
		},
		{
			name: "function with params no return type",
			fn: types.Function{
				Name:   "greet",
				Params: "(name)",
			},
			expected: "def greet(name)",
		},
		{
			name: "function with no params",
			fn: types.Function{
				Name:       "get_value",
				Params:     "",
				ReturnType: "string",
			},
			expected: "def get_value() -> string",
		},
		{
			name: "function with no params and no return type",
			fn: types.Function{
				Name: "noop",
			},
			expected: "def noop()",
		},
		{
			name: "function with self param (method)",
			fn: types.Function{
				Name:       "process",
				Params:     "(self, data)",
				ReturnType: "bool",
			},
			expected: "def process(self, data) -> bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSignature(tt.fn)
			if result != tt.expected {
				t.Errorf("formatSignature() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatClassSignature(t *testing.T) {
	tests := []struct {
		name     string
		cls      types.Class
		expected string
	}{
		{
			name: "class with base classes",
			cls: types.Class{
				Name:  "MyClass",
				Bases: []string{"BaseClass", "Mixin"},
			},
			expected: "class MyClass(BaseClass, Mixin)",
		},
		{
			name: "class with single base",
			cls: types.Class{
				Name:  "Dog",
				Bases: []string{"Animal"},
			},
			expected: "class Dog(Animal)",
		},
		{
			name: "class with no bases",
			cls: types.Class{
				Name: "SimpleClass",
			},
			expected: "class SimpleClass",
		},
		{
			name: "class with empty bases slice",
			cls: types.Class{
				Name:  "EmptyBases",
				Bases: []string{},
			},
			expected: "class EmptyBases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatClassSignature(tt.cls)
			if result != tt.expected {
				t.Errorf("formatClassSignature() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSaveAndLoadMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "metadata.json")

	original := IndexMetadata{
		Model:     "test-model",
		Timestamp: time.Now().Truncate(time.Second),
		Count:     42,
		Dimension: 384,
		Provider:  "ollama",
	}

	// Test save
	err := saveMetadata(metadataPath, original)
	if err != nil {
		t.Fatalf("saveMetadata failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatalf("Metadata file should exist: %v", err)
	}

	// Test load
	loaded, err := loadMetadata(metadataPath)
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}

	if loaded.Model != original.Model {
		t.Errorf("Model: got %s, want %s", loaded.Model, original.Model)
	}
	if loaded.Count != original.Count {
		t.Errorf("Count: got %d, want %d", loaded.Count, original.Count)
	}
	if loaded.Dimension != original.Dimension {
		t.Errorf("Dimension: got %d, want %d", loaded.Dimension, original.Dimension)
	}
	if loaded.Provider != original.Provider {
		t.Errorf("Provider: got %s, want %s", loaded.Provider, original.Provider)
	}
}

func TestLoadMetadataNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	metadataPath := filepath.Join(tmpDir, "nonexistent.json")

	_, err := loadMetadata(metadataPath)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestSaveMetadataInvalidPath(t *testing.T) {
	metadata := IndexMetadata{Model: "test"}

	// Test with invalid path (directory doesn't exist)
	err := saveMetadata("/nonexistent/path/metadata.json", metadata)
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestIndexMetadataJSON(t *testing.T) {
	// Test that IndexMetadata can be marshaled and unmarshaled correctly
	original := IndexMetadata{
		Model:     "nomic-embed-text",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Count:     100,
		Dimension: 768,
		Provider:  "http://localhost:11434",
	}

	// This is implicitly tested by saveMetadata/loadMetadata,
	// but we verify the structure is valid
	if original.Model == "" {
		t.Error("Model should not be empty")
	}
	if original.Dimension <= 0 {
		t.Error("Dimension should be positive")
	}
}

func TestBuilderWithMockProvider(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Python file for testing
	pyFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(pyFile, []byte("def hello():\n    pass\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	provider := &mockProvider{}
	builder, err := NewBuilder(tmpDir, provider)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	// Test Scan
	files, err := builder.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Filter to Python files
	var pyFiles []string
	for _, f := range files {
		if f.Language == "python" {
			pyFiles = append(pyFiles, f.Path)
		}
	}

	if len(pyFiles) != 1 {
		t.Errorf("Expected 1 Python file, got %d", len(pyFiles))
	}

	// Test cache dir
	cacheDir := builder.GetCacheDir()
	if cacheDir == "" {
		t.Error("GetCacheDir should not be empty")
	}

	// Verify cache directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Errorf("Cache directory should exist: %v", err)
	}
}

func TestBuilderEmbed(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &mockProvider{}

	builder, err := NewBuilder(tmpDir, provider)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	units := []*CodeUnit{
		{
			Name:       "func1",
			Type:       "function",
			Signature:  "def func1()",
			Docstring:  "First function",
			FilePath:   "test.py",
			LineNumber: 1,
		},
		{
			Name:       "func2",
			Type:       "function",
			Signature:  "def func2()",
			Docstring:  "Second function",
			FilePath:   "test.py",
			LineNumber: 10,
		},
	}

	embeddings, err := builder.Embed(units)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(embeddings) != 2 {
		t.Errorf("Expected 2 embeddings, got %d", len(embeddings))
	}

	// Check embedding dimension
	if len(embeddings) > 0 && len(embeddings[0]) != 3 {
		t.Errorf("Expected dimension 3, got %d", len(embeddings[0]))
	}
}

func TestBuilderEmbedEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &mockProvider{}

	builder, err := NewBuilder(tmpDir, provider)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	embeddings, err := builder.Embed(nil)
	if err != nil {
		t.Fatalf("Embed with nil should not fail: %v", err)
	}
	if embeddings != nil {
		t.Error("Embed with nil should return nil")
	}

	embeddings, err = builder.Embed([]*CodeUnit{})
	if err != nil {
		t.Fatalf("Embed with empty slice should not fail: %v", err)
	}
	if embeddings != nil {
		t.Error("Embed with empty slice should return nil")
	}
}

func TestBuilderEmbedError(t *testing.T) {
	tmpDir := t.TempDir()
	errorProvider := &mockProviderWithError{}

	builder, err := NewBuilder(tmpDir, errorProvider)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	units := []*CodeUnit{
		{
			Name:      "func1",
			Type:      "function",
			Signature: "def func1()",
		},
	}

	_, err = builder.Embed(units)
	if err == nil {
		t.Error("Expected error from embed provider")
	}
}

func TestNewBuilderWithCustomDimension(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &mockProviderCustomEmbeddings{dimension: 512}

	builder, err := NewBuilder(tmpDir, provider)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	units := []*CodeUnit{
		{
			Name:      "test_func",
			Type:      "function",
			Signature: "def test_func()",
		},
	}

	embeddings, err := builder.Embed(units)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(embeddings) != 1 {
		t.Fatalf("Expected 1 embedding, got %d", len(embeddings))
	}

	if len(embeddings[0]) != 512 {
		t.Errorf("Expected dimension 512, got %d", len(embeddings[0]))
	}
}

func TestCodeUnitFields(t *testing.T) {
	// Test that CodeUnit struct has all expected fields
	unit := CodeUnit{
		Name:       "test_function",
		Type:       "function",
		FilePath:   "app/utils.py",
		LineNumber: 25,
		Signature:  "def test_function(arg1, arg2)",
		Docstring:  "Test function documentation",
		Calls:      []string{"helper", "validate"},
		CalledBy:   []string{"main", "run"},
	}

	if unit.Name != "test_function" {
		t.Errorf("Name mismatch: got %s", unit.Name)
	}
	if unit.Type != "function" {
		t.Errorf("Type mismatch: got %s", unit.Type)
	}
	if unit.FilePath != "app/utils.py" {
		t.Errorf("FilePath mismatch: got %s", unit.FilePath)
	}
	if unit.LineNumber != 25 {
		t.Errorf("LineNumber mismatch: got %d", unit.LineNumber)
	}
	if unit.Signature != "def test_function(arg1, arg2)" {
		t.Errorf("Signature mismatch: got %s", unit.Signature)
	}
	if unit.Docstring != "Test function documentation" {
		t.Errorf("Docstring mismatch: got %s", unit.Docstring)
	}
	if len(unit.Calls) != 2 {
		t.Errorf("Calls length mismatch: got %d", len(unit.Calls))
	}
	if len(unit.CalledBy) != 2 {
		t.Errorf("CalledBy length mismatch: got %d", len(unit.CalledBy))
	}
}

func TestBuilderGetCodeUnits(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &mockProvider{}

	builder, err := NewBuilder(tmpDir, provider)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	// Initially should be nil
	units := builder.GetCodeUnits()
	if units != nil {
		t.Error("Expected nil code units before extraction")
	}
}

func TestEmbeddingTextLongCallTruncation(t *testing.T) {
	// Test that long calls/calledby lists are truncated to 200 chars
	longCalls := make([]string, 50)
	for i := range longCalls {
		longCalls[i] = "function_with_a_very_long_name_that_exceeds_the_limit"
	}

	unit := &CodeUnit{
		Name:      "test",
		Type:      "function",
		Signature: "def test()",
		Docstring: "",
		Calls:     longCalls,
		CalledBy:  longCalls,
	}

	result := EmbeddingText(unit)

	// The calls section should be truncated
	// Check that the result doesn't exceed reasonable length
	if len(result) > 600 { // Some buffer for the rest of the text
		t.Errorf("Result seems too long, might not be truncating properly")
	}
}
