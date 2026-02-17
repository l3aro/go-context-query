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
	"sync"
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

	ignoreFiles, err := s.findIgnoreFiles(absRoot)
	if err != nil {
		return nil, fmt.Errorf("finding ignore files: %w", err)
	}

	var files []FileInfo
	var filesMu sync.Mutex

	type pendingFile struct {
		path         string
		relPathSlash string
		info         os.FileInfo
	}

	var pendingFiles []pendingFile

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}

		if relPath == "." {
			return nil
		}

		relPathSlash := filepath.ToSlash(relPath)

		if s.opts.SkipHidden && s.isHidden(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			if s.isDefaultExcluded(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		patterns := s.getPatternsForPath(absRoot, relPath, ignoreFiles)
		if s.matchesIgnorePatterns(relPathSlash, patterns) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		pendingFiles = append(pendingFiles, pendingFile{
			path:         path,
			relPathSlash: relPathSlash,
			info:         info,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	var wg sync.WaitGroup
	results := make([]FileInfo, len(pendingFiles))

	for i, pf := range pendingFiles {
		wg.Add(1)
		go func(index int, p pendingFile) {
			defer wg.Done()

			info := p.info

			if info.Mode()&os.ModeSymlink != 0 {
				if !s.opts.FollowSymlinks {
					return
				}
				realPath, err := filepath.EvalSymlinks(p.path)
				if err != nil {
					return
				}
				realAbs, err := filepath.Abs(realPath)
				if err != nil {
					return
				}
				if !strings.HasPrefix(realAbs, absRoot+string(filepath.Separator)) && realAbs != absRoot {
					return
				}
				targetInfo, err := os.Stat(realPath)
				if err != nil {
					return
				}
				if targetInfo.IsDir() {
					return
				}
				info = targetInfo
			}

			language := DetectLanguage(filepath.Ext(p.path))

			filesMu.Lock()
			results[index] = FileInfo{
				Path:     p.relPathSlash,
				FullPath: p.path,
				Language: language,
				Size:     info.Size(),
			}
			filesMu.Unlock()
		}(i, pf)
	}

	wg.Wait()

	for _, f := range results {
		if f.Path != "" {
			files = append(files, f)
		}
	}

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

// findIgnoreFiles finds all .gcqignore files in the directory tree.
func (s *Scanner) findIgnoreFiles(root string) (map[string][]IgnorePattern, error) {
	ignoreFiles := make(map[string][]IgnorePattern)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		// Skip hidden directories
		if s.opts.SkipHidden && s.isHidden(d.Name()) {
			return nil
		}

		if s.isDefaultExcluded(d.Name()) {
			return nil
		}

		patterns, err := s.loadIgnorePatterns(path)
		if err != nil {
			return nil // Continue walking even if we can't read ignore file
		}

		if len(patterns) > 0 {
			ignoreFiles[path] = patterns
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("finding ignore files: %w", err)
	}

	return ignoreFiles, nil
}

// getPatternsForPath returns all ignore patterns that apply to a given path.
// Patterns are collected from all ancestor directories, with closer ancestors
// taking precedence (later patterns override earlier ones).
func (s *Scanner) getPatternsForPath(absRoot, relPath string, ignoreFiles map[string][]IgnorePattern) []IgnorePattern {
	var result []IgnorePattern

	relPath = filepath.ToSlash(relPath)
	parts := strings.Split(relPath, "/")

	// Build all ancestor paths from root to the file's parent
	for i := 0; i <= len(parts); i++ {
		var ancestorPath string
		if i == 0 {
			ancestorPath = absRoot
		} else {
			ancestorPath = filepath.Join(absRoot, strings.Join(parts[:i], string(filepath.Separator)))
		}

		if patterns, ok := ignoreFiles[ancestorPath]; ok {
			result = append(result, patterns...)
		}
	}

	return result
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
