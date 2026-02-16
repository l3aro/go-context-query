// Package types defines the core data structures for code representation.
// It includes types for functions, classes, imports, call graphs, and module information.
package types

// Function represents a function definition
type Function struct {
	Name       string   `json:"name"`
	Params     string   `json:"params"`
	ReturnType string   `json:"return_type"`
	Docstring  string   `json:"docstring"`
	LineNumber int      `json:"line_number"`
	IsMethod   bool     `json:"is_method"`
	IsAsync    bool     `json:"is_async"`
	Decorators []string `json:"decorators"`
}

// Method represents a class method (alias for Function with IsMethod=true)
type Method = Function

// Class represents a class definition
type Class struct {
	Name       string   `json:"name"`
	Bases      []string `json:"bases"`
	Docstring  string   `json:"docstring"`
	Methods    []Method `json:"methods"`
	LineNumber int      `json:"line_number"`
}

// Interface represents an interface definition (e.g., Go interfaces, TypeScript interfaces)
type Interface struct {
	Name       string   `json:"name"`
	Bases      []string `json:"bases,omitempty"`
	Docstring  string   `json:"docstring"`
	Methods    []Method `json:"methods"`
	LineNumber int      `json:"line_number"`
}

// Trait represents a trait definition (e.g., Rust traits, PHP traits)
type Trait struct {
	Name       string   `json:"name"`
	Docstring  string   `json:"docstring"`
	Methods    []Method `json:"methods"`
	LineNumber int      `json:"line_number"`
}

// Protocol represents a protocol definition (e.g., Swift protocols)
type Protocol struct {
	Name       string   `json:"name"`
	Bases      []string `json:"bases,omitempty"`
	Docstring  string   `json:"docstring"`
	Methods    []Method `json:"methods"`
	LineNumber int      `json:"line_number"`
}

// Enum represents an enum definition (e.g., Rust enums, TypeScript enums, Go iota)
type Enum struct {
	Name       string   `json:"name"`
	Variants   []string `json:"variants,omitempty"`
	Docstring  string   `json:"docstring"`
	LineNumber int      `json:"line_number"`
}

// Struct represents a struct definition (e.g., Go structs, C structs)
type Struct struct {
	Name       string   `json:"name"`
	Docstring  string   `json:"docstring"`
	Fields     []string `json:"fields,omitempty"`
	LineNumber int      `json:"line_number"`
}

// Import represents an import statement
type Import struct {
	Module     string   `json:"module"`
	Names      []string `json:"names"`
	IsFrom     bool     `json:"is_from"`
	LineNumber int      `json:"line_number"`
}

// CallGraphEdge represents a single edge in the call graph
// (caller_file, caller_func, callee_file, callee_func)
type CallGraphEdge struct {
	SourceFile string `json:"src_file"`
	SourceFunc string `json:"src_func"`
	DestFile   string `json:"dst_file"`
	DestFunc   string `json:"dst_func"`
}

// CallGraph represents the call graph of a module
type CallGraph struct {
	Edges []CallGraphEdge `json:"edges"`
}

// ModuleInfo contains all extracted information about a module
type ModuleInfo struct {
	Path       string      `json:"path"`
	Functions  []Function  `json:"functions"`
	Classes    []Class     `json:"classes"`
	Imports    []Import    `json:"imports"`
	CallGraph  CallGraph   `json:"call_graph"`
	LineNumber int         `json:"line_number,omitempty"`
	Signature  string      `json:"signature,omitempty"`
	Docstring  string      `json:"docstring,omitempty"`
	Type       string      `json:"type,omitempty"`
	Language   string      `json:"language,omitempty"`
	Interfaces []Interface `json:"interfaces,omitempty"`
	Traits     []Trait     `json:"traits,omitempty"`
	Protocols  []Protocol  `json:"protocols,omitempty"`
	Enums      []Enum      `json:"enums,omitempty"`
	Structs    []Struct    `json:"structs,omitempty"`
}

// EmbeddingUnit combines L1 (local) and L2 (cross-file) context data
type EmbeddingUnit struct {
	L1Data ModuleInfo      `json:"l1_data"`
	L2Data []CallGraphEdge `json:"l2_data"`
}

// Config holds application configuration
type Config struct {
	// Embedding provider (e.g., "openai", "anthropic", "local")
	Provider string `json:"provider"`
	// Model names for embeddings and completion
	EmbeddingModel  string `json:"embedding_model"`
	CompletionModel string `json:"completion_model"`
	// Thresholds for similarity and context selection
	SimilarityThreshold float64 `json:"similarity_threshold"`
	ContextThreshold    float64 `json:"context_threshold"`
	// Max tokens for context
	MaxContextTokens int `json:"max_context_tokens"`
}
