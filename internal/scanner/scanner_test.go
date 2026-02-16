package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScannerScan(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"main.go":                  "package main",
		"utils/helper.go":          "package utils",
		"README.md":                "# Test",
		"src/app.py":               "print('hello')",
		"src/index.js":             "console.log('hi')",
		".hidden/file.txt":         "hidden content",
		"node_modules/pkg/main.js": "module.exports = {}",
		".git/config":              "[core]",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Test scanning with default options
	scanner := New(DefaultOptions())
	results, err := scanner.Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should find: main.go, utils/helper.go, README.md, src/app.py, src/index.js
	// Should NOT find: .hidden/file.txt (hidden), node_modules/pkg/main.js (excluded), .git/config (excluded)
	expectedFiles := map[string]string{
		"main.go":         "go",
		"utils/helper.go": "go",
		"README.md":       "markdown",
		"src/app.py":      "python",
		"src/index.js":    "javascript",
	}

	foundFiles := make(map[string]bool)
	for _, f := range results {
		foundFiles[f.Path] = true
		if expectedLang, ok := expectedFiles[f.Path]; ok {
			if f.Language != expectedLang {
				t.Errorf("Expected %s to have language %s, got %s", f.Path, expectedLang, f.Language)
			}
		}
	}

	for expected := range expectedFiles {
		if !foundFiles[expected] {
			t.Errorf("Expected to find %s, but it wasn't found", expected)
		}
	}

	// Ensure excluded files are not present
	excludedFiles := []string{".hidden/file.txt", "node_modules/pkg/main.js", ".git/config"}
	for _, excluded := range excludedFiles {
		if foundFiles[excluded] {
			t.Errorf("Expected %s to be excluded, but it was found", excluded)
		}
	}
}

func TestScannerWithGcqignore(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create .gcqignore file
	gcqignoreContent := `# Ignore test files
*.test.js
# Ignore build directory
build/
# Ignore specific file
secret.txt
`
	err := os.WriteFile(filepath.Join(tmpDir, ".gcqignore"), []byte(gcqignoreContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gcqignore: %v", err)
	}

	// Create test files
	files := []string{
		"app.js",
		"app.test.js",
		"main.go",
		"build/output.js",
		"secret.txt",
		"public/index.html",
	}

	for _, path := range files {
		fullPath := filepath.Join(tmpDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		err = os.WriteFile(fullPath, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Test scanning
	scanner := New(DefaultOptions())
	results, err := scanner.Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	foundFiles := make(map[string]bool)
	for _, f := range results {
		foundFiles[f.Path] = true
	}

	// Should find
	expectedFiles := []string{"app.js", "main.go", "public/index.html"}
	for _, expected := range expectedFiles {
		if !foundFiles[expected] {
			t.Errorf("Expected to find %s", expected)
		}
	}

	// Should NOT find (ignored by .gcqignore)
	ignoredFiles := []string{"app.test.js", "build/output.js", "secret.txt"}
	for _, ignored := range ignoredFiles {
		if foundFiles[ignored] {
			t.Errorf("Expected %s to be ignored", ignored)
		}
	}
}

func TestScannerSkipHidden(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files
	os.WriteFile(filepath.Join(tmpDir, "visible.txt"), []byte("visible"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, ".hidden"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".hidden/file.txt"), []byte("hidden"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("node_modules"), 0644)

	// Test with SkipHidden = true
	opts := DefaultOptions()
	scanner := New(opts)
	results, _ := scanner.Scan(tmpDir)

	foundHidden := false
	for _, f := range results {
		if f.Path == ".hidden/file.txt" || f.Path == ".gitignore" {
			foundHidden = true
		}
	}
	if foundHidden {
		t.Error("Should skip hidden files when SkipHidden=true")
	}

	// Test with SkipHidden = false
	opts.SkipHidden = false
	scanner = New(opts)
	results, _ = scanner.Scan(tmpDir)

	foundGitignore := false
	for _, f := range results {
		if f.Path == ".gitignore" {
			foundGitignore = true
		}
	}
	if !foundGitignore {
		t.Error("Should find .gitignore when SkipHidden=false")
	}
}

func TestLanguageDetection(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".go", "go"},
		{".py", "python"},
		{".js", "javascript"},
		{".ts", "typescript"},
		{".rs", "rust"},
		{".java", "java"},
		{".cpp", "cpp"},
		{".md", "markdown"},
		{".json", "json"},
		{".yaml", "yaml"},
		{".unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		result := DetectLanguage(tt.ext)
		if result != tt.expected {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.ext, result, tt.expected)
		}
	}
}

func TestIgnorePattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		// Simple patterns
		{"*.js", "file.js", true},
		{"*.js", "dir/file.js", true},
		{"*.js", "file.txt", false},
		{"build/", "build/file.js", true},
		{"build/", "other/build/file.js", true},
		{"build/", "builder.js", false},

		// Absolute patterns
		{"/build/", "build/file.js", true},
		{"/build/", "src/build/file.js", false},

		// Directory patterns
		{"node_modules/", "node_modules/pkg/file.js", true},
		{"node_modules/", "src/node_modules/pkg/file.js", true},

		// Glob patterns
		{"*.test.js", "app.test.js", true},
		{"*.test.js", "deep/app.test.js", true},
		{"src/*.js", "src/app.js", true},
		{"src/*.js", "src/deep/app.js", false},

		// Double asterisk
		{"**/test/**", "test/file.js", true},
		{"**/test/**", "src/test/file.js", true},
		{"**/test/**", "src/deep/test/file.js", true},
		{"**/test/**", "testing/file.js", false},

		// Question mark
		{"file?.js", "file1.js", true},
		{"file?.js", "file12.js", false},

		// Negation - pattern matches but is negation
		{"!*.js", "file.js", true}, // Negation pattern still matches the file
	}

	for _, tt := range tests {
		pattern := ParseIgnorePattern(tt.pattern)
		result := pattern.Match(tt.path)
		if result != tt.match {
			t.Errorf("Pattern %q matching %q: got %v, want %v", tt.pattern, tt.path, result, tt.match)
		}
	}
}
