// Package scanner provides file tree walking functionality with ignore pattern support.
// It respects .gcqignore files with gitignore-style patterns and provides language
// detection based on file extensions.
package scanner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo represents information about a discovered file.
type FileInfo struct {
	Path     string // Relative path from root
	FullPath string // Absolute path
	Language string // Detected language from extension
	Size     int64  // File size in bytes
}

// Options configures the scanner behavior.
type Options struct {
	SkipHidden      bool     // Skip hidden files and directories (starting with .)
	FollowSymlinks  bool     // Follow symlinks (within root only)
	DefaultExcludes []string // Default directories to exclude
	IgnoreFileName  string   // Name of the ignore file (default: .gcqignore)
}

// DefaultOptions returns scanner options with sensible defaults.
func DefaultOptions() Options {
	return Options{
		SkipHidden:     true,
		FollowSymlinks: false,
		IgnoreFileName: ".gcqignore",
		DefaultExcludes: []string{
			"node_modules",
			".git",
			"__pycache__",
			".venv",
			"venv",
			"dist",
			"build",
			".idea",
			".vscode",
			"vendor",
			".hg",
			".svn",
			"CVS",
			".tox",
			".nox",
			"target", // Rust/Java build
			"bin",
			"obj",
		},
	}
}

// Scanner provides file tree scanning capabilities.
type Scanner struct {
	opts Options
	root string
}

// New creates a new Scanner with the given options.
func New(opts Options) *Scanner {
	return &Scanner{opts: opts}
}

// Scan recursively scans the directory at root and returns a list of FileInfo.
// It respects .gcqignore patterns and default exclusions.
func (s *Scanner) Scan(root string) ([]FileInfo, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path: %w", err)
	}
	s.root = absRoot

	// Load ignore patterns from root
	ignorePatterns, err := s.loadIgnorePatterns(absRoot)
	if err != nil {
		return nil, fmt.Errorf("loading ignore patterns: %w", err)
	}

	var files []FileInfo

	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Log error but continue walking
			return nil
		}

		// Get relative path for pattern matching
		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}

		// Skip root itself
		if relPath == "." {
			return nil
		}

		// Normalize path for pattern matching (use forward slashes)
		relPathSlash := filepath.ToSlash(relPath)

		// Check if should skip hidden files/directories
		if s.opts.SkipHidden && s.isHidden(info.Name()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check default excludes for directories
		if info.IsDir() {
			if s.isDefaultExcluded(info.Name()) {
				return filepath.SkipDir
			}
			// Load nested .gcqignore if present
			nestedPatterns, err := s.loadIgnorePatterns(path)
			if err == nil && len(nestedPatterns) > 0 {
				ignorePatterns = append(ignorePatterns, nestedPatterns...)
			}
			return nil
		}

		// Check ignore patterns
		if s.matchesIgnorePatterns(relPathSlash, ignorePatterns) {
			return nil
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			if !s.opts.FollowSymlinks {
				return nil
			}
			// Resolve symlink and check if it's within root
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil // Skip broken symlinks
			}
			realAbs, err := filepath.Abs(realPath)
			if err != nil {
				return nil
			}
			// Ensure symlink target is within root
			if !strings.HasPrefix(realAbs, absRoot+string(filepath.Separator)) && realAbs != absRoot {
				return nil
			}
			// Get info of the target
			targetInfo, err := os.Stat(realPath)
			if err != nil {
				return nil
			}
			if targetInfo.IsDir() {
				return nil // Don't follow directory symlinks
			}
			info = targetInfo
		}

		// Detect language from extension
		language := DetectLanguage(filepath.Ext(path))

		files = append(files, FileInfo{
			Path:     relPathSlash,
			FullPath: path,
			Language: language,
			Size:     info.Size(),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return files, nil
}

// isHidden checks if a file or directory name indicates it's hidden.
func (s *Scanner) isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

// isDefaultExcluded checks if the name matches default exclusion patterns.
func (s *Scanner) isDefaultExcluded(name string) bool {
	for _, exclude := range s.opts.DefaultExcludes {
		if strings.EqualFold(name, exclude) {
			return true
		}
	}
	return false
}

// loadIgnorePatterns loads ignore patterns from .gcqignore file in the given directory.
func (s *Scanner) loadIgnorePatterns(dir string) ([]IgnorePattern, error) {
	ignorePath := filepath.Join(dir, s.opts.IgnoreFileName)
	file, err := os.Open(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var patterns []IgnorePattern
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, ParseIgnorePattern(line))
	}

	return patterns, scanner.Err()
}

// matchesIgnorePatterns checks if the given path should be ignored based on patterns.
// It implements gitignore semantics: patterns are checked in order, and negation
// patterns can override previous positive matches.
func (s *Scanner) matchesIgnorePatterns(relPath string, patterns []IgnorePattern) bool {
	ignored := false
	for _, pattern := range patterns {
		if pattern.Match(relPath) {
			if pattern.IsNegation() {
				ignored = false
			} else {
				ignored = true
			}
		}
	}
	return ignored
}

// Scan is a convenience function that scans a directory with default options.
func Scan(root string) ([]FileInfo, error) {
	scanner := New(DefaultOptions())
	return scanner.Scan(root)
}

// ScanWithOptions scans a directory with custom options.
func ScanWithOptions(root string, opts Options) ([]FileInfo, error) {
	scanner := New(opts)
	return scanner.Scan(root)
}
