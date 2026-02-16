// Package extractor provides language-specific code extraction functionality.
package extractor

import (
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/user/go-context-query/pkg/types"
)

// Extractor defines the interface for language-specific AST extractors.
// Each language implementation should provide a concrete implementation of this interface.
type Extractor interface {
	// Extract parses a source file and returns structured information about the module.
	Extract(file string) (*types.ModuleInfo, error)
}

// Language represents a supported programming language.
type Language string

const (
	// Python language support
	Python Language = "python"
	// Go language support (future)
	Go Language = "go"
	// TypeScript language support (future)
	TypeScript Language = "typescript"
	// JavaScript language support (future)
	JavaScript Language = "javascript"
)

// ParserFactory is a function that creates a new tree-sitter parser for a language.
type ParserFactory func() *sitter.Parser

// LanguageRegistry maps file extensions to their corresponding extractors and parsers.
type LanguageRegistry struct {
	extractors map[Language]Extractor
	parsers    map[Language]ParserFactory
	extensions map[string]Language
}

// NewLanguageRegistry creates a new language registry with default language mappings.
func NewLanguageRegistry() *LanguageRegistry {
	registry := &LanguageRegistry{
		extractors: make(map[Language]Extractor),
		parsers:    make(map[Language]ParserFactory),
		extensions: make(map[string]Language),
	}

	// Register built-in language mappings
	registry.RegisterLanguage(Python, []string{".py", ".pyw", ".pyi"}, NewPythonExtractor, NewPythonParser)

	return registry
}

// RegisterLanguage registers a new language with the registry.
func (r *LanguageRegistry) RegisterLanguage(
	lang Language,
	extensions []string,
	extractorFactory func() Extractor,
	parserFactory ParserFactory,
) {
	r.extractors[lang] = extractorFactory()
	r.parsers[lang] = parserFactory
	for _, ext := range extensions {
		r.extensions[ext] = lang
	}
}

// GetExtractor returns the extractor for a given file path based on its extension.
func (r *LanguageRegistry) GetExtractor(filePath string) (Extractor, error) {
	lang, err := r.GetLanguage(filePath)
	if err != nil {
		return nil, err
	}

	extractor, ok := r.extractors[lang]
	if !ok {
		return nil, fmt.Errorf("no extractor registered for language: %s", lang)
	}

	return extractor, nil
}

// GetParser returns a new parser instance for a given file path.
func (r *LanguageRegistry) GetParser(filePath string) (*sitter.Parser, error) {
	lang, err := r.GetLanguage(filePath)
	if err != nil {
		return nil, err
	}

	factory, ok := r.parsers[lang]
	if !ok {
		return nil, fmt.Errorf("no parser factory registered for language: %s", lang)
	}

	return factory(), nil
}

// GetLanguage returns the language identifier for a given file path.
func (r *LanguageRegistry) GetLanguage(filePath string) (Language, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return "", fmt.Errorf("file has no extension: %s", filePath)
	}

	lang, ok := r.extensions[ext]
	if !ok {
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}

	return lang, nil
}

// IsSupported checks if a file extension is supported.
func (r *LanguageRegistry) IsSupported(filePath string) bool {
	_, err := r.GetLanguage(filePath)
	return err == nil
}

// GetSupportedExtensions returns all registered file extensions.
func (r *LanguageRegistry) GetSupportedExtensions() []string {
	extensions := make([]string, 0, len(r.extensions))
	for ext := range r.extensions {
		extensions = append(extensions, ext)
	}
	return extensions
}

// BaseExtractor provides common functionality for all language extractors.
type BaseExtractor struct {
	parser *sitter.Parser
	lang   Language
}

// NewBaseExtractor creates a new base extractor.
func NewBaseExtractor(parser *sitter.Parser, lang Language) *BaseExtractor {
	return &BaseExtractor{
		parser: parser,
		lang:   lang,
	}
}

// ExtractFile extracts module information from a file using the appropriate extractor.
func ExtractFile(filePath string) (*types.ModuleInfo, error) {
	registry := NewLanguageRegistry()
	extractor, err := registry.GetExtractor(filePath)
	if err != nil {
		return nil, err
	}
	return extractor.Extract(filePath)
}
