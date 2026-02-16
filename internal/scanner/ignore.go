package scanner

import (
	"path/filepath"
	"strings"
)

// IgnorePattern represents a single gitignore-style pattern.
type IgnorePattern struct {
	pattern     string // Original pattern
	isNegation  bool   // True if pattern starts with !
	isDirectory bool   // True if pattern ends with /
	isAbsolute  bool   // True if pattern starts with /
	segments    []string
}

// ParseIgnorePattern parses a gitignore-style pattern string.
func ParseIgnorePattern(pattern string) IgnorePattern {
	p := IgnorePattern{pattern: pattern}

	// Check for negation
	if strings.HasPrefix(pattern, "!") {
		p.isNegation = true
		pattern = pattern[1:]
	}

	// Check if directory pattern (ends with /)
	if strings.HasSuffix(pattern, "/") {
		p.isDirectory = true
		pattern = strings.TrimSuffix(pattern, "/")
	}

	// Check if absolute path pattern (starts with /)
	if strings.HasPrefix(pattern, "/") {
		p.isAbsolute = true
		pattern = pattern[1:]
	}

	// Split pattern into segments
	p.segments = strings.Split(pattern, "/")

	return p
}

// Match checks if the given path matches this ignore pattern.
// Returns true if the path should be ignored (matches and is not negation),
// or if it matches a negation pattern (to be handled by caller).
func (p IgnorePattern) Match(path string) bool {
	path = filepath.ToSlash(path)

	// For directory patterns, check if the path is within the directory
	if p.isDirectory {
		return p.matchDirectory(path)
	}

	// Handle glob patterns
	if strings.Contains(p.pattern, "*") || strings.Contains(p.pattern, "?") || strings.Contains(p.pattern, "[") {
		return p.matchGlob(path)
	}

	// Split path into segments
	pathSegments := strings.Split(path, "/")

	// Try matching at each possible starting position
	for startIdx := 0; startIdx < len(pathSegments); startIdx++ {
		if p.matchSegments(pathSegments[startIdx:]) {
			return true
		}
	}

	return false
}

// IsNegation returns true if this pattern is a negation pattern.
func (p IgnorePattern) IsNegation() bool {
	return p.isNegation
}

// matchDirectory checks if the path is within a directory matching the pattern.
func (p IgnorePattern) matchDirectory(path string) bool {
	// For absolute directory patterns, check from root
	if p.isAbsolute {
		pathSegments := strings.Split(path, "/")
		if len(pathSegments) < len(p.segments) {
			return false
		}
		for i, seg := range p.segments {
			if !strings.EqualFold(seg, pathSegments[i]) {
				return false
			}
		}
		return true
	}

	// For relative directory patterns, check at any level
	pathSegments := strings.Split(path, "/")
	for startIdx := 0; startIdx <= len(pathSegments)-len(p.segments); startIdx++ {
		match := true
		for i, seg := range p.segments {
			if !strings.EqualFold(seg, pathSegments[startIdx+i]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}

	return false
}

// matchGlob handles wildcard patterns (*, ?, and [...]).
func (p IgnorePattern) matchGlob(path string) bool {
	pattern := p.pattern
	if p.isNegation {
		pattern = pattern[1:]
	}
	if strings.HasPrefix(pattern, "/") {
		pattern = pattern[1:]
	}

	segments := strings.Split(pattern, "/")
	pathSegments := strings.Split(path, "/")

	// Try matching at each possible starting position
	for startIdx := 0; startIdx < len(pathSegments); startIdx++ {
		if p.matchGlobSegments(segments, pathSegments[startIdx:]) {
			return true
		}
	}

	return false
}

// matchGlobSegments matches glob patterns against path segments.
func (p IgnorePattern) matchGlobSegments(patternSegs, pathSegs []string) bool {
	if len(patternSegs) == 0 {
		return len(pathSegs) == 0
	}

	// Handle ** (match any number of directories)
	if patternSegs[0] == "**" {
		if len(patternSegs) == 1 {
			return true // ** matches everything
		}
		// Try matching the rest of the pattern at each position
		for i := 0; i <= len(pathSegs); i++ {
			if p.matchGlobSegments(patternSegs[1:], pathSegs[i:]) {
				return true
			}
		}
		return false
	}

	if len(pathSegs) == 0 {
		return false
	}

	if !matchGlobSegment(patternSegs[0], pathSegs[0]) {
		return false
	}

	return p.matchGlobSegments(patternSegs[1:], pathSegs[1:])
}

// matchSegments matches simple (non-glob) pattern segments against path segments.
func (p IgnorePattern) matchSegments(pathSegments []string) bool {
	if len(p.segments) > len(pathSegments) {
		return false
	}

	for i, seg := range p.segments {
		if seg == "**" {
			// Handle ** pattern
			if i == len(p.segments)-1 {
				return true // ** at end matches everything
			}
			// Try matching the rest at each position
			for j := i; j <= len(pathSegments); j++ {
				if p.matchSegmentsAt(pathSegments[j:], p.segments[i+1:]) {
					return true
				}
			}
			return false
		}

		if !strings.EqualFold(seg, pathSegments[i]) {
			return false
		}
	}

	return len(p.segments) == len(pathSegments)
}

// matchSegmentsAt matches remaining pattern segments at a specific position.
func (p IgnorePattern) matchSegmentsAt(pathSegments, patternSegments []string) bool {
	if len(patternSegments) > len(pathSegments) {
		return false
	}

	for i, seg := range patternSegments {
		if !strings.EqualFold(seg, pathSegments[i]) {
			return false
		}
	}

	return true
}

// matchGlobSegment matches a single glob pattern segment against a path segment.
func matchGlobSegment(pattern, segment string) bool {
	// Simple glob matching for *, ?, and [...]
	patternIdx, segmentIdx := 0, 0

	for patternIdx < len(pattern) && segmentIdx < len(segment) {
		p := pattern[patternIdx]
		s := segment[segmentIdx]

		switch p {
		case '*':
			// Handle ** separately
			if patternIdx+1 < len(pattern) && pattern[patternIdx+1] == '*' {
				// This is ** - should be handled at segment level
				return matchGlobSegment(pattern[patternIdx+1:], segment[segmentIdx:])
			}
			// * matches any sequence of characters
			if patternIdx+1 == len(pattern) {
				return true
			}
			nextChar := pattern[patternIdx+1]
			for segmentIdx < len(segment) && segment[segmentIdx] != nextChar {
				segmentIdx++
			}
			patternIdx++
		case '?':
			patternIdx++
			segmentIdx++
		case '[':
			// Character class [...]
			endIdx := strings.IndexByte(pattern[patternIdx:], ']')
			if endIdx == -1 {
				return false
			}
			charClass := pattern[patternIdx+1 : patternIdx+endIdx]
			if !strings.ContainsRune(charClass, rune(s)) {
				return false
			}
			patternIdx += endIdx + 1
			segmentIdx++
		default:
			if p != s {
				return false
			}
			patternIdx++
			segmentIdx++
		}
	}

	// Handle trailing *
	for patternIdx < len(pattern) && pattern[patternIdx] == '*' {
		patternIdx++
	}

	return patternIdx == len(pattern) && segmentIdx == len(segment)
}
