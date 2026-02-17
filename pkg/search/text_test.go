package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestTextSearcher_Search_BasicRegex(t *testing.T) {
	// Create a temp directory with test files
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte(`package main

func Hello() string {
	return "Hello, World!"
}

func Goodbye() string {
	return "Goodbye, World!"
}
`), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	searcher := NewTextSearcher(TextSearchOptions{})

	matches, err := searcher.Search(context.Background(), "Hello", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(matches) == 0 {
		t.Error("expected matches but got none")
	}

	// Check first match
	found := false
	for _, m := range matches {
		if m.Match == "Hello" {
			found = true
			if m.LineNumber != 3 {
				t.Errorf("expected line 3, got %d", m.LineNumber)
			}
			break
		}
	}
	if !found {
		t.Error("expected to find 'Hello' match")
	}
}

func TestTextSearcher_Search_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte(`package main

func Hello() string {
	return "Hello, World!"
}
`), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	searcher := NewTextSearcher(TextSearchOptions{})

	matches, err := searcher.Search(context.Background(), "NonExistentPattern12345", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("expected no matches but got %d", len(matches))
	}
}

func TestTextSearcher_Search_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple test files
	files := map[string]string{
		"main.go": `package main

func main() {
	fmt.Println("hello world")
}`,
		"helper.go": `package main

func helper() {
	fmt.Println("hello there")
}`,
		"utils.go": `package utils

func util() {
	fmt.Println("general kenobi")
}`,
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
		if err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	searcher := NewTextSearcher(TextSearchOptions{})

	matches, err := searcher.Search(context.Background(), "hello", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find "hello" in main.go and helper.go
	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}
}

func TestTextSearcher_Search_ContextLines(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte(`package main

// Comment line 3
func Hello() string {
	// Comment line 5
	return "Hello"
}

// Comment line 8
func Bye() string {
	return "Bye"
}
`), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	searcher := NewTextSearcher(TextSearchOptions{
		ContextLines: 2,
	})

	matches, err := searcher.Search(context.Background(), "func Hello", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(matches) == 0 {
		t.Fatal("expected matches but got none")
	}

	m := matches[0]
	if len(m.ContextBefore) != 2 {
		t.Errorf("expected 2 context before lines, got %d", len(m.ContextBefore))
	}
	if len(m.ContextAfter) != 2 {
		t.Errorf("expected 2 context after lines, got %d", len(m.ContextAfter))
	}

	// Verify context content
	// Line 1: package main, Line 2: (empty), Line 3: // Comment, Line 4: func Hello
	// With contextLines=2, we get lines 2-3 before
	if len(m.ContextBefore) > 1 {
		if m.ContextBefore[1] != "// Comment line 3" {
			t.Errorf("expected context before[1] to be '// Comment line 3', got %q", m.ContextBefore[1])
		}
	}
}

func TestTextSearcher_Search_ExtensionsFilter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with different extensions
	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main\nfunc test()"), 0644)
	if err != nil {
		t.Fatalf("failed to create test.go: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "test.py"), []byte("def test():\n    pass"), 0644)
	if err != nil {
		t.Fatalf("failed to create test.py: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("failed to create test.txt: %v", err)
	}

	// Search only .go files
	searcher := NewTextSearcher(TextSearchOptions{
		Extensions: []string{".go"},
	})

	matches, err := searcher.Search(context.Background(), "test", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should only match in test.go
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
}

func TestTextSearcher_Search_MaxResults(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with multiple matches
	content := `package main
func test1() {}
func test2() {}
func test3() {}
func test4() {}
func test5() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	searcher := NewTextSearcher(TextSearchOptions{
		MaxResults: 3,
	})

	matches, err := searcher.Search(context.Background(), "func test", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(matches) != 3 {
		t.Errorf("expected 3 matches (max), got %d", len(matches))
	}
}

func TestTextSearcher_Search_ExcludesDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file in main directory
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nvar found = true"), 0644)
	if err != nil {
		t.Fatalf("failed to create main.go: %v", err)
	}

	// Create node_modules with a file
	nodeModules := filepath.Join(tmpDir, "node_modules", "pkg")
	err = os.MkdirAll(nodeModules, 0755)
	if err != nil {
		t.Fatalf("failed to create node_modules: %v", err)
	}
	err = os.WriteFile(filepath.Join(nodeModules, "test.go"), []byte("var found = false"), 0644)
	if err != nil {
		t.Fatalf("failed to create test.go in node_modules: %v", err)
	}

	searcher := NewTextSearcher(TextSearchOptions{})

	matches, err := searcher.Search(context.Background(), "found", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should only find the one in main.go, not in node_modules
	if len(matches) != 1 {
		t.Errorf("expected 1 match (excluding node_modules), got %d", len(matches))
	}
	if matches[0].FilePath != filepath.Join(tmpDir, "main.go") {
		t.Errorf("expected match in main.go, got %s", matches[0].FilePath)
	}
}

func TestTextSearcher_Search_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(`package main
var HELLO = "world"
`), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Case insensitive search (default)
	searcher := NewTextSearcher(TextSearchOptions{
		CaseSensitive: false,
	})

	matches, err := searcher.Search(context.Background(), "hello", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(matches) == 0 {
		t.Error("expected match with case insensitive search")
	}

	// Case sensitive search
	searcherCaseSensitive := NewTextSearcher(TextSearchOptions{
		CaseSensitive: true,
	})

	matches, err = searcherCaseSensitive.Search(context.Background(), "hello", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should not find "hello" (lowercase) when it's "HELLO" (uppercase)
	if len(matches) != 0 {
		t.Errorf("expected no matches with case sensitive search, got %d", len(matches))
	}
}

func TestTextSearcher_Search_InvalidRegex(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	searcher := NewTextSearcher(TextSearchOptions{})

	_, err = searcher.Search(context.Background(), "[invalid(regex", tmpDir)
	if err == nil {
		t.Error("expected error for invalid regex, got nil")
	}
}

func TestTextSearcher_Search_EmptyPattern(t *testing.T) {
	tmpDir := t.TempDir()

	searcher := NewTextSearcher(TextSearchOptions{})

	_, err := searcher.Search(context.Background(), "", tmpDir)
	if err == nil {
		t.Error("expected error for empty pattern, got nil")
	}
}

func TestTextSearcher_Search_NonExistentRoot(t *testing.T) {
	searcher := NewTextSearcher(TextSearchOptions{})

	_, err := searcher.Search(context.Background(), "test", "/nonexistent/path/12345")
	if err == nil {
		t.Error("expected error for nonexistent root, got nil")
	}
}

func TestTextSearcher_Search_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdir first
	subdir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subdir, 0755)
	if err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Create multiple files
	for i := 0; i < 20; i++ {
		err = os.WriteFile(filepath.Join(subdir, fmt.Sprintf("file%d.go", i)),
			[]byte("package main\nfunc test() {}\n"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	searcher := NewTextSearcher(TextSearchOptions{
		Extensions: []string{".go"},
	})

	matches, err := searcher.Search(context.Background(), "func test", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find at least one match
	if len(matches) == 0 {
		t.Error("expected matches in multiple files")
	}
}

func TestNewTextSearcher_DefaultExcludes(t *testing.T) {
	searcher := NewTextSearcher(TextSearchOptions{})

	// Verify default excludes are set
	if len(searcher.opts.Excludes) == 0 {
		t.Error("expected default excludes to be set")
	}

	// Verify node_modules is in excludes
	found := false
	for _, ex := range searcher.opts.Excludes {
		if ex == "node_modules" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'node_modules' in default excludes")
	}
}

func TestSearch_ConvenienceFunction(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main\nvar test = 1"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	matches, err := Search(context.Background(), "test", tmpDir)
	if err != nil {
		t.Fatalf("Search convenience function failed: %v", err)
	}

	if len(matches) == 0 {
		t.Error("expected matches")
	}
}

func TestTextMatch_ColumnOffset(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte(`package main
var helloWorld = "test"
`), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	searcher := NewTextSearcher(TextSearchOptions{})

	matches, err := searcher.Search(context.Background(), "hello", tmpDir)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(matches) == 0 {
		t.Fatal("expected match")
	}

	m := matches[0]
	// "hello" should start at column 4 (0-based)
	if m.Column != 4 {
		t.Errorf("expected column 4, got %d", m.Column)
	}
}
