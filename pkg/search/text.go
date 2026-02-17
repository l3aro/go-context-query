// Package search provides text search functionality for regex/pattern matching
// across files in a project directory.
package search

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// DefaultExcludes are the default directories to exclude from text search.
var DefaultExcludes = []string{
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
	"target",
	"bin",
	"obj",
	".gitignore",
	".gcqignore",
}

// TextSearchOptions configures text search behavior.
type TextSearchOptions struct {
	// Extensions limits search to files with these extensions (e.g., [".py", ".go"]).
	// Empty means all files.
	Extensions []string
	// ContextLines is the number of lines to include before and after each match.
	ContextLines int
	// MaxResults limits the total number of matches returned.
	// 0 means no limit.
	MaxResults int
	// Excludes is a list of directory names to exclude from search.
	// If nil, DefaultExcludes is used.
	Excludes []string
	// CaseSensitive determines if the search is case-sensitive.
	CaseSensitive bool
}

// TextMatch represents a single regex match in a file.
type TextMatch struct {
	// FilePath is the path to the file containing the match.
	FilePath string
	// LineNumber is the 1-based line number where the match was found.
	LineNumber int
	// LineContent is the full line containing the match.
	LineContent string
	// Column is the 0-based column offset where the match starts.
	Column int
	// Match is the matched text.
	Match string
	// ContextBefore contains lines before the match (if ContextLines > 0).
	ContextBefore []string
	// ContextAfter contains lines after the match (if ContextLines > 0).
	ContextAfter []string
}

// TextSearcher provides regex-based text search across files.
type TextSearcher struct {
	opts TextSearchOptions
}

// NewTextSearcher creates a new TextSearcher with the given options.
func NewTextSearcher(opts TextSearchOptions) *TextSearcher {
	if opts.Excludes == nil {
		opts.Excludes = DefaultExcludes
	}
	return &TextSearcher{opts: opts}
}

// Search performs a regex search for pattern in all files under root.
// It returns all matches with their locations and optional context.
func (s *TextSearcher) Search(ctx context.Context, pattern, root string) ([]TextMatch, error) {
	if pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path: %w", err)
	}

	// Verify root exists
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("checking root path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", absRoot)
	}

	// Compile regex
	var regex *regexp.Regexp
	flags := ""
	if !s.opts.CaseSensitive {
		flags = "(?i)"
	}
	regex, err = regexp.Compile(flags + pattern)
	if err != nil {
		return nil, fmt.Errorf("compiling regex: %w", err)
	}

	// Collect files to search
	files, err := s.collectFiles(absRoot)
	if err != nil {
		return nil, fmt.Errorf("collecting files: %w", err)
	}

	// Search files concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	matches := make([]TextMatch, 0)
	resultChan := make(chan []TextMatch, len(files))

	// Limit concurrency
	sem := make(chan struct{}, 10)

	for _, file := range files {
		wg.Add(1)
		go func(file string) {
			sem <- struct{}{}
			defer wg.Done()
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			fileMatches, err := s.searchFile(ctx, absRoot, file, regex)
			if err != nil {
				// Skip file on error, log if needed
				return
			}
			if len(fileMatches) > 0 {
				resultChan <- fileMatches
			}
		}(file)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for fileMatches := range resultChan {
		mu.Lock()
		for _, m := range fileMatches {
			if s.opts.MaxResults > 0 && len(matches) >= s.opts.MaxResults {
				mu.Unlock()
				return matches, nil
			}
			matches = append(matches, m)
		}
		mu.Unlock()
	}

	return matches, nil
}

// collectFiles walks the directory tree and returns files matching the options.
func (s *TextSearcher) collectFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		// Check if directory should be excluded
		if d.IsDir() {
			dirName := filepath.Base(path)
			if s.isExcluded(dirName) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check extension filter
		if len(s.opts.Extensions) > 0 {
			ext := filepath.Ext(path)
			extMatch := false
			for _, e := range s.opts.Extensions {
				if ext == e {
					extMatch = true
					break
				}
			}
			if !extMatch {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// isExcluded checks if a directory name should be excluded.
func (s *TextSearcher) isExcluded(name string) bool {
	for _, exclude := range s.opts.Excludes {
		if strings.EqualFold(name, exclude) {
			return true
		}
	}
	return false
}

// searchFile searches a single file for pattern matches.
func (s *TextSearcher) searchFile(ctx context.Context, root, filePath string, regex *regexp.Regexp) ([]TextMatch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var matches []TextMatch
	contextLines := s.opts.ContextLines

	for i, line := range lines {
		select {
		case <-ctx.Done():
			return matches, ctx.Err()
		default:
		}

		loc := regex.FindStringIndex(line)
		if loc == nil {
			continue
		}

		match := TextMatch{
			FilePath:    filePath,
			LineNumber:  i + 1, // 1-based
			LineContent: line,
			Column:      loc[0],
			Match:       line[loc[0]:loc[1]],
		}

		// Add context lines
		if contextLines > 0 {
			start := i - contextLines
			if start < 0 {
				start = 0
			}
			for j := start; j < i; j++ {
				match.ContextBefore = append(match.ContextBefore, lines[j])
			}

			end := i + contextLines + 1
			if end > len(lines) {
				end = len(lines)
			}
			for j := i + 1; j < end; j++ {
				match.ContextAfter = append(match.ContextAfter, lines[j])
			}
		}

		matches = append(matches, match)

		// Check max results
		if s.opts.MaxResults > 0 && len(matches) >= s.opts.MaxResults {
			break
		}
	}

	return matches, nil
}

// Search is a convenience function that performs text search with default options.
func Search(ctx context.Context, pattern, root string) ([]TextMatch, error) {
	return NewTextSearcher(TextSearchOptions{}).Search(ctx, pattern, root)
}
