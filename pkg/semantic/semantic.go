// Package semantic provides semantic indexing functionality for code.
// It orchestrates scanning, extraction, embedding, and storage to build
// a searchable vector index of code units.
package semantic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/l3aro/go-context-query/internal/scanner"
	"github.com/l3aro/go-context-query/pkg/callgraph"
	"github.com/l3aro/go-context-query/pkg/embed"
	"github.com/l3aro/go-context-query/pkg/extractor"
	"github.com/l3aro/go-context-query/pkg/index"
	"github.com/l3aro/go-context-query/pkg/types"
)

// CodeUnit represents a single unit of code ready for embedding.
// It combines L1 (local) and L2 (cross-file) data.
type CodeUnit struct {
	// Name is the name of the function/method/class
	Name string `json:"name"`
	// Type is the type of unit (function, method, class)
	Type string `json:"type"`
	// FilePath is the path to the file containing this unit
	FilePath string `json:"file_path"`
	// LineNumber is the line where this unit is defined
	LineNumber int `json:"line_number"`
	// Signature is the function signature
	Signature string `json:"signature"`
	// Docstring is the docstring/comment
	Docstring string `json:"docstring"`
	// Calls is the list of functions this unit calls (forward)
	Calls []string `json:"calls"`
	// CalledBy is the list of functions that call this unit (backward)
	CalledBy []string `json:"called_by"`
	// CFGSummary is an optional control flow graph summary (complexity, blocks)
	CFGSummary string `json:"cfg_summary,omitempty"`
	// DFGSummary is an optional data flow graph summary (variables, edges)
	DFGSummary string `json:"dfg_summary,omitempty"`
}

// EmbeddingText builds rich text for embedding from a CodeUnit.
// It combines L1 (signature + docstring) and L2 (calls + called_by) data.
// This follows the pattern from llm-tldr/tldr/semantic.py build_embedding_text().
func EmbeddingText(unit *CodeUnit) string {
	var parts []string

	// Add name and type for context at the beginning
	typeStr := unit.Type
	if typeStr == "" {
		typeStr = "function"
	}
	parts = append(parts, fmt.Sprintf("%s: %s", strings.Title(typeStr), unit.Name))

	// L1: Signature + docstring
	if unit.Signature != "" {
		parts = append(parts, fmt.Sprintf("Signature: %s", unit.Signature))
	}
	if unit.Docstring != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", unit.Docstring))
	}

	// L2: Call graph (forward - callees)
	if len(unit.Calls) > 0 {
		callsStr := strings.Join(unit.Calls, ", ")
		if len(callsStr) > 200 {
			callsStr = callsStr[:200] + "..."
		}
		parts = append(parts, fmt.Sprintf("Calls: %s", callsStr))
	}

	// L2: Call graph (backward - callers)
	if len(unit.CalledBy) > 0 {
		callersStr := strings.Join(unit.CalledBy, ", ")
		if len(callersStr) > 200 {
			callersStr = callersStr[:200] + "..."
		}
		parts = append(parts, fmt.Sprintf("Called by: %s", callersStr))
	}

	// L3: Control flow summary (optional)
	if unit.CFGSummary != "" {
		parts = append(parts, fmt.Sprintf("Control flow: %s", unit.CFGSummary))
	}

	// L3: Data flow summary (optional)
	if unit.DFGSummary != "" {
		parts = append(parts, fmt.Sprintf("Data flow: %s", unit.DFGSummary))
	}

	return strings.Join(parts, "\n")
}

// IndexMetadata holds metadata about the semantic index
type IndexMetadata struct {
	// Model is the embedding model used
	Model string `json:"model"`
	// Timestamp is when the index was created
	Timestamp time.Time `json:"timestamp"`
	// Count is the number of code units indexed
	Count int `json:"count"`
	// Dimension is the embedding dimension
	Dimension int `json:"dimension"`
	// Provider is the embedding provider used
	Provider string `json:"provider"`
}

// Builder orchestrates the semantic indexing pipeline:
// scan → extract → embed → store
type Builder struct {
	// rootDir is the project root directory
	rootDir string
	// cacheDir is where to store the index
	cacheDir string
	// scanner scans the project for files
	scanner *scanner.Scanner
	// extractor extracts code structure from files
	extractor *extractor.LanguageRegistry
	// callGraphResolver resolves cross-file call graphs
	callGraphResolver *callgraph.Resolver
	// embedProvider generates embeddings
	embedProvider embed.Provider
	// vectorIndex stores the vector index
	vectorIndex *index.VectorIndex
	// codeUnits stores the extracted code units
	codeUnits []*CodeUnit
}

// NewBuilder creates a new semantic index builder
func NewBuilder(rootDir string, embedProvider embed.Provider) (*Builder, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path: %w", err)
	}

	// Create cache directory at project root
	cacheDir := filepath.Join(absRoot, ".gcq", "cache", "semantic")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}

	return &Builder{
		rootDir:           absRoot,
		cacheDir:          cacheDir,
		scanner:           scanner.New(scanner.DefaultOptions()),
		extractor:         extractor.NewLanguageRegistry(),
		callGraphResolver: callgraph.NewResolver(absRoot, extractor.NewPythonExtractor()),
		embedProvider:     embedProvider,
		vectorIndex:       nil,
		codeUnits:         nil,
	}, nil
}

// Scan scans the project for supported files
func (b *Builder) Scan() ([]scanner.FileInfo, error) {
	return b.scanner.Scan(b.rootDir)
}

// Extract extracts code units from scanned files
func (b *Builder) Extract(files []scanner.FileInfo) ([]*CodeUnit, error) {
	// Group files by language for processing
	// We support multiple languages now, not just Python
	languageFiles := make(map[string][]string)
	for _, f := range files {
		lang := f.Language
		if lang == "" {
			continue
		}
		languageFiles[lang] = append(languageFiles[lang], f.FullPath)
	}

	// Get Python files for call graph resolution (if any)
	var pyFiles []string
	if pf, ok := languageFiles["python"]; ok {
		pyFiles = pf
	}

	// Create lookup maps for calls and callers (only for Python for now)
	callsMap := make(map[string][]string)   // func -> functions it calls
	callersMap := make(map[string][]string) // func -> functions that call it

	// Build cross-file call graph only for Python files
	if len(pyFiles) > 0 {
		callGraph, err := b.callGraphResolver.ResolveCalls(pyFiles)
		if err != nil {
			// Don't fail - just log and continue without call graph
			fmt.Printf("Warning: building call graph: %v\n", err)
		} else {
			// Process edges
			for _, edge := range callGraph.CrossFileEdges {
				callerKey := fmt.Sprintf("%s:%s", edge.SourceFile, edge.SourceFunc)
				calleeKey := fmt.Sprintf("%s:%s", edge.DestFile, edge.DestFunc)
				callsMap[callerKey] = append(callsMap[callerKey], calleeKey)
				callersMap[calleeKey] = append(callersMap[callerKey], callerKey)
			}

			// Also process intra-file edges
			for _, edge := range callGraph.IntraFileEdges {
				callerKey := fmt.Sprintf("%s:%s", edge.SourceFile, edge.SourceFunc)
				calleeKey := fmt.Sprintf("%s:%s", edge.DestFile, edge.DestFunc)
				callsMap[callerKey] = append(callsMap[callerKey], calleeKey)
				callersMap[calleeKey] = append(callersMap[callerKey], callerKey)
			}
		}
	}

	// Extract code units from each language's files
	var units []*CodeUnit

	for lang, filePaths := range languageFiles {
		for _, filePath := range filePaths {
			ext, err := b.extractor.GetExtractor(filePath)
			if err != nil {
				// Skip unsupported files
				continue
			}

			moduleInfo, err := ext.Extract(filePath)
			if err != nil {
				// Skip files that can't be parsed
				continue
			}

			relPath, err := filepath.Rel(b.rootDir, filePath)
			if err != nil {
				relPath = filePath
			}

			// Determine language-specific signature prefix
			sigPrefix := getSignaturePrefix(lang)

			// Extract functions
			for _, fn := range moduleInfo.Functions {
				unit := &CodeUnit{
					Name:       fn.Name,
					Type:       "function",
					FilePath:   relPath,
					LineNumber: fn.LineNumber,
					Signature:  formatSignatureForLang(fn, lang, sigPrefix),
					Docstring:  fn.Docstring,
					Calls:      callsMap[fmt.Sprintf("%s:%s", relPath, fn.Name)],
					CalledBy:   callersMap[fmt.Sprintf("%s:%s", relPath, fn.Name)],
				}
				units = append(units, unit)
			}

			// Extract classes
			for _, cls := range moduleInfo.Classes {
				unit := &CodeUnit{
					Name:       cls.Name,
					Type:       "class",
					FilePath:   relPath,
					LineNumber: cls.LineNumber,
					Signature:  formatClassSignatureForLang(cls, lang),
					Docstring:  cls.Docstring,
					Calls:      callsMap[fmt.Sprintf("%s:%s", relPath, cls.Name)],
					CalledBy:   callersMap[fmt.Sprintf("%s:%s", relPath, cls.Name)],
				}
				units = append(units, unit)

				// Extract methods
				for _, method := range cls.Methods {
					methodUnit := &CodeUnit{
						Name:       fmt.Sprintf("%s.%s", cls.Name, method.Name),
						Type:       "method",
						FilePath:   relPath,
						LineNumber: method.LineNumber,
						Signature:  formatMethodSignatureForLang(method, cls.Name, lang, sigPrefix),
						Docstring:  method.Docstring,
						Calls:      callsMap[fmt.Sprintf("%s:%s", relPath, method.Name)],
						CalledBy:   callersMap[fmt.Sprintf("%s:%s", relPath, method.Name)],
					}
					units = append(units, methodUnit)
				}
			}

			// Extract interfaces (for Go/TypeScript)
			for _, iface := range moduleInfo.Interfaces {
				unit := &CodeUnit{
					Name:       iface.Name,
					Type:       "interface",
					FilePath:   relPath,
					LineNumber: iface.LineNumber,
					Signature:  formatInterfaceSignature(iface),
					Docstring:  iface.Docstring,
					Calls:      callsMap[fmt.Sprintf("%s:%s", relPath, iface.Name)],
					CalledBy:   callersMap[fmt.Sprintf("%s:%s", relPath, iface.Name)],
				}
				units = append(units, unit)
			}
		}
	}

	b.codeUnits = units
	return units, nil
}

// getSignaturePrefix returns the language-specific function keyword
func getSignaturePrefix(lang string) string {
	switch lang {
	case "python":
		return "def"
	case "go":
		return "func"
	case "typescript", "javascript":
		return "function"
	default:
		return "def"
	}
}

// formatSignatureForLang formats a function signature for the given language
func formatSignatureForLang(fn types.Function, lang string, prefix string) string {
	params := fn.Params
	if params == "" {
		params = "()"
	}
	if fn.ReturnType != "" {
		switch lang {
		case "python":
			return fmt.Sprintf("%s %s%s -> %s", prefix, fn.Name, params, fn.ReturnType)
		case "go":
			return fmt.Sprintf("%s %s%s %s", prefix, fn.Name, params, fn.ReturnType)
		case "typescript", "javascript":
			return fmt.Sprintf("%s %s%s: %s", prefix, fn.Name, params, fn.ReturnType)
		default:
			return fmt.Sprintf("%s %s%s -> %s", prefix, fn.Name, params, fn.ReturnType)
		}
	}
	return fmt.Sprintf("%s %s%s", prefix, fn.Name, params)
}

// formatMethodSignatureForLang formats a method signature for the given language
func formatMethodSignatureForLang(method types.Method, className string, lang string, prefix string) string {
	params := method.Params
	if params == "" {
		params = "()"
	}
	if method.ReturnType != "" {
		switch lang {
		case "python":
			return fmt.Sprintf("def %s.%s%s -> %s", className, method.Name, params, method.ReturnType)
		case "go":
			return fmt.Sprintf("func (%s) %s%s %s", className, method.Name, params, method.ReturnType)
		case "typescript", "javascript":
			return fmt.Sprintf("%s %s.%s%s: %s", prefix, className, method.Name, params, method.ReturnType)
		default:
			return fmt.Sprintf("%s %s.%s%s -> %s", prefix, className, method.Name, params, method.ReturnType)
		}
	}
	switch lang {
	case "python":
		return fmt.Sprintf("def %s.%s%s", className, method.Name, params)
	case "go":
		return fmt.Sprintf("func (%s) %s%s", className, method.Name, params)
	case "typescript", "javascript":
		return fmt.Sprintf("%s %s.%s%s", prefix, className, method.Name, params)
	default:
		return fmt.Sprintf("%s %s.%s%s", prefix, className, method.Name, params)
	}
}

// formatClassSignatureForLang formats a class signature for the given language
func formatClassSignatureForLang(cls types.Class, lang string) string {
	switch lang {
	case "python":
		if len(cls.Bases) > 0 {
			return fmt.Sprintf("class %s(%s)", cls.Name, strings.Join(cls.Bases, ", "))
		}
		return fmt.Sprintf("class %s", cls.Name)
	case "go":
		return fmt.Sprintf("type %s struct", cls.Name)
	case "typescript", "javascript":
		if len(cls.Bases) > 0 {
			return fmt.Sprintf("class %s extends %s", cls.Name, cls.Bases[0])
		}
		return fmt.Sprintf("class %s", cls.Name)
	default:
		if len(cls.Bases) > 0 {
			return fmt.Sprintf("class %s(%s)", cls.Name, strings.Join(cls.Bases, ", "))
		}
		return fmt.Sprintf("class %s", cls.Name)
	}
}

// formatInterfaceSignature formats an interface signature
func formatInterfaceSignature(iface types.Interface) string {
	if len(iface.Methods) > 0 {
		methodNames := make([]string, len(iface.Methods))
		for i, m := range iface.Methods {
			methodNames[i] = m.Name
		}
		return fmt.Sprintf("interface %s { %s }", iface.Name, strings.Join(methodNames, ", "))
	}
	return fmt.Sprintf("interface %s", iface.Name)
}

// formatSignature formats a function signature
func formatSignature(fn types.Function) string {
	params := fn.Params
	if params == "" {
		params = "()"
	}
	if fn.ReturnType != "" {
		return fmt.Sprintf("def %s%s -> %s", fn.Name, params, fn.ReturnType)
	}
	return fmt.Sprintf("def %s%s", fn.Name, params)
}

// formatClassSignature formats a class signature
func formatClassSignature(cls types.Class) string {
	if len(cls.Bases) > 0 {
		return fmt.Sprintf("class %s(%s)", cls.Name, strings.Join(cls.Bases, ", "))
	}
	return fmt.Sprintf("class %s", cls.Name)
}

// Embed generates embeddings for the code units
func (b *Builder) Embed(units []*CodeUnit) ([][]float32, error) {
	if len(units) == 0 {
		return nil, nil
	}

	// Build embedding texts
	texts := make([]string, len(units))
	for i, unit := range units {
		texts[i] = EmbeddingText(unit)
	}

	// Generate embeddings
	embeddings, err := b.embedProvider.Embed(texts)
	if err != nil {
		return nil, fmt.Errorf("generating embeddings: %w", err)
	}

	return embeddings, nil
}

// Build builds the complete semantic index
func (b *Builder) Build() (*index.VectorIndex, *IndexMetadata, error) {
	// Step 1: Scan
	files, err := b.Scan()
	if err != nil {
		return nil, nil, fmt.Errorf("scanning: %w", err)
	}

	// Step 2: Extract
	units, err := b.Extract(files)
	if err != nil {
		return nil, nil, fmt.Errorf("extracting: %w", err)
	}

	if len(units) == 0 {
		return nil, &IndexMetadata{
			Model:     b.embedProvider.Config().Model,
			Timestamp: time.Now(),
			Count:     0,
			Provider:  b.embedProvider.Config().Endpoint,
		}, nil
	}

	// Step 3: Embed
	embeddings, err := b.Embed(units)
	if err != nil {
		return nil, nil, fmt.Errorf("embedding: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, nil, fmt.Errorf("no embeddings generated")
	}

	dimension := len(embeddings[0])

	// Step 4: Store in vector index
	vecIndex := index.NewVectorIndex(dimension)

	for i, unit := range units {
		unitID := fmt.Sprintf("%s:%s", unit.FilePath, unit.Name)

		// Convert CodeUnit to EmbeddingUnit for storage
		embeddingUnit := types.EmbeddingUnit{
			L1Data: types.ModuleInfo{
				Path: unit.FilePath,
			},
		}

		if err := vecIndex.Add(unitID, embeddings[i], embeddingUnit); err != nil {
			return nil, nil, fmt.Errorf("adding to index: %w", err)
		}
	}

	b.vectorIndex = vecIndex

	// Create metadata
	metadata := &IndexMetadata{
		Model:     b.embedProvider.Config().Model,
		Timestamp: time.Now(),
		Count:     len(units),
		Dimension: dimension,
		Provider:  b.embedProvider.Config().Endpoint,
	}

	return vecIndex, metadata, nil
}

// Save saves the index and metadata to disk
func (b *Builder) Save() error {
	if b.vectorIndex == nil {
		return fmt.Errorf("no index to save")
	}

	// Save vector index
	indexPath := filepath.Join(b.cacheDir, "index.msgpack")
	if err := b.vectorIndex.Save(indexPath); err != nil {
		return fmt.Errorf("saving index: %w", err)
	}

	// Save metadata
	metadataPath := filepath.Join(b.cacheDir, "metadata.json")
	metadata := IndexMetadata{
		Model:     b.embedProvider.Config().Model,
		Timestamp: time.Now(),
		Count:     len(b.codeUnits),
		Dimension: b.vectorIndex.Dimension(),
		Provider:  b.embedProvider.Config().Endpoint,
	}

	if err := saveMetadata(metadataPath, metadata); err != nil {
		return fmt.Errorf("saving metadata: %w", err)
	}

	return nil
}

// Load loads an existing index from disk
func (b *Builder) Load() (*index.VectorIndex, *IndexMetadata, error) {
	indexPath := filepath.Join(b.cacheDir, "index.msgpack")
	metadataPath := filepath.Join(b.cacheDir, "metadata.json")

	// Check if index exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("index not found at %s", indexPath)
	}

	// Load index
	vecIndex := index.NewVectorIndex(0)
	if err := vecIndex.Load(indexPath); err != nil {
		return nil, nil, fmt.Errorf("loading index: %w", err)
	}

	// Load metadata
	metadata, err := loadMetadata(metadataPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading metadata: %w", err)
	}

	b.vectorIndex = vecIndex
	return vecIndex, metadata, nil
}

// GetCacheDir returns the cache directory path
func (b *Builder) GetCacheDir() string {
	return b.cacheDir
}

// GetCodeUnits returns the extracted code units
func (b *Builder) GetCodeUnits() []*CodeUnit {
	return b.codeUnits
}

// NewBuilderWithOllama creates a new builder with Ollama provider
func NewBuilderWithOllama(rootDir, model, endpoint string) (*Builder, error) {
	provider, err := embed.NewOllamaProvider(&embed.Config{
		Model:    model,
		Endpoint: endpoint,
	})
	if err != nil {
		return nil, err
	}
	return NewBuilder(rootDir, provider)
}

// NewBuilderWithHuggingFace creates a new builder with HuggingFace provider
func NewBuilderWithHuggingFace(rootDir, model, token string) (*Builder, error) {
	provider, err := embed.NewHuggingFaceProvider(&embed.Config{
		Model:  model,
		APIKey: token,
	})
	if err != nil {
		return nil, err
	}
	return NewBuilder(rootDir, provider)
}

// BuildIndex is a convenience function to build and save a semantic index
func BuildIndex(rootDir string, embedProvider embed.Provider) error {
	builder, err := NewBuilder(rootDir, embedProvider)
	if err != nil {
		return fmt.Errorf("creating builder: %w", err)
	}

	vecIndex, metadata, err := builder.Build()
	if err != nil {
		return fmt.Errorf("building index: %w", err)
	}

	if vecIndex == nil || vecIndex.Count() == 0 {
		fmt.Println("No code units found to index")
		return nil
	}

	if err := builder.Save(); err != nil {
		return fmt.Errorf("saving index: %w", err)
	}

	fmt.Printf("Indexed %d code units (dimension: %d, model: %s)\n",
		metadata.Count, metadata.Dimension, metadata.Model)
	fmt.Printf("Index saved to: %s\n", builder.GetCacheDir())

	return nil
}

// LoadIndex loads an existing semantic index
func LoadIndex(rootDir string) (*index.VectorIndex, *IndexMetadata, error) {
	cacheDir := filepath.Join(rootDir, ".gcq", "cache", "semantic")
	indexPath := filepath.Join(cacheDir, "index.msgpack")
	metadataPath := filepath.Join(cacheDir, "metadata.json")

	vecIndex := index.NewVectorIndex(0)
	if err := vecIndex.Load(indexPath); err != nil {
		return nil, nil, fmt.Errorf("loading index: %w", err)
	}

	metadata, err := loadMetadata(metadataPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading metadata: %w", err)
	}

	return vecIndex, metadata, nil
}

// saveMetadata saves index metadata to a JSON file
func saveMetadata(path string, metadata IndexMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}
	return nil
}

// loadMetadata loads index metadata from a JSON file
func loadMetadata(path string) (*IndexMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading metadata: %w", err)
	}
	var metadata IndexMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}
	return &metadata, nil
}
