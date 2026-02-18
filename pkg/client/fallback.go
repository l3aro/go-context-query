// Package client provides fallback execution when daemon is unavailable.
package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/l3aro/go-context-query/internal/config"
	"github.com/l3aro/go-context-query/internal/scanner"
	"github.com/l3aro/go-context-query/pkg/callgraph"
	"github.com/l3aro/go-context-query/pkg/embed"
	"github.com/l3aro/go-context-query/pkg/extractor"
	"github.com/l3aro/go-context-query/pkg/index"
	"github.com/l3aro/go-context-query/pkg/search"
	"github.com/l3aro/go-context-query/pkg/types"
)

// Executor performs direct execution when daemon is unavailable
type Executor struct {
	index     *index.VectorIndex
	searcher  *search.Searcher
	embedder  embed.Provider
	scanner   *scanner.Scanner
	callGraph *callgraph.Builder
}

// NewExecutor creates a new fallback executor
func NewExecutor() (*Executor, error) {
	cfg := config.DefaultConfig()

	providerType := cfg.Warm.Provider
	if providerType == "" {
		providerType = cfg.Provider
	}
	if providerType == "" {
		providerType = "ollama"
	}

	model := cfg.Warm.Model
	if model == "" {
		model = cfg.OllamaModel
	}
	if model == "" {
		model = "nomic-embed-text"
	}

	endpoint := cfg.Warm.BaseURL
	if endpoint == "" {
		endpoint = cfg.OllamaBaseURL
	}
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	apiKey := cfg.Warm.Token
	if apiKey == "" {
		apiKey = cfg.OllamaAPIKey
	}

	embedCfg := &embed.Config{
		Endpoint: endpoint,
		Model:    model,
		APIKey:   apiKey,
	}

	var err error
	var embedder embed.Provider

	switch providerType {
	case config.ProviderOllama:
		embedder, err = embed.NewOllamaProvider(embedCfg)
	case config.ProviderHuggingFace:
		hfModel := cfg.Warm.Model
		if hfModel == "" {
			hfModel = cfg.HFModel
		}
		if hfModel == "" {
			hfModel = "sentence-transformers/all-MiniLM-L6-v2"
		}
		hfToken := cfg.Warm.Token
		if hfToken == "" {
			hfToken = cfg.HFToken
		}
		embedder, err = embed.NewHuggingFaceProvider(&embed.Config{
			Model:  hfModel,
			APIKey: hfToken,
		})
	default:
		embedder, err = embed.NewOllamaProvider(embedCfg)
	}

	if err != nil {
		return nil, fmt.Errorf("initializing embedder: %w", err)
	}

	dimension := 768
	if providerType == config.ProviderHuggingFace {
		modelCheck := cfg.Warm.Model
		if modelCheck == "" {
			modelCheck = cfg.HFModel
		}
		if modelCheck == "sentence-transformers/all-MiniLM-L6-v2" {
			dimension = 384
		}
	}

	idx := index.NewVectorIndex(dimension)
	indexPath := filepath.Join(os.TempDir(), "gcq.idx")
	if err := idx.Load(indexPath); err != nil {
		// Index may not exist yet, that's ok
	}

	searcher := search.NewSearcher(embedder, idx)

	return &Executor{
		index:     idx,
		searcher:  searcher,
		embedder:  embedder,
		scanner:   scanner.New(scanner.DefaultOptions()),
		callGraph: callgraph.NewBuilder(),
	}, nil
}

// Search performs a semantic search using direct execution
func (e *Executor) Search(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}

	results, err := e.searcher.Search(params.Query, params.Limit)
	if err != nil {
		return nil, fmt.Errorf("search error: %w", err)
	}

	if params.Threshold > 0 {
		filtered := make([]search.SearchResult, 0)
		for _, r := range results {
			if float64(r.Score) >= params.Threshold {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	searchResults := make([]SearchResult, len(results))
	for i, r := range results {
		searchResults[i] = SearchResult{
			FilePath:   r.FilePath,
			LineNumber: r.LineNumber,
			Name:       r.Name,
			Signature:  r.Signature,
			Docstring:  r.Docstring,
			Type:       r.Type,
			Score:      float64(r.Score),
		}
	}

	return searchResults, nil
}

// Extract extracts code context from a path
func (e *Executor) Extract(ctx context.Context, params ExtractParams) (*ExtractResult, error) {
	files, err := e.scanner.Scan(params.Path)
	if err != nil {
		return nil, fmt.Errorf("scan error: %w", err)
	}

	var extractedCount int
	for _, file := range files {
		filePath := file.FullPath

		moduleInfo, err := extractor.ExtractFile(filePath)
		if err != nil {
			continue
		}

		cg, err := e.callGraph.BuildFromFile(filePath, moduleInfo)
		if err != nil {
			continue
		}

		moduleInfo.CallGraph = cg.ToCallGraph()

		unit := types.EmbeddingUnit{
			L1Data: *moduleInfo,
			L2Data: moduleInfo.CallGraph.Edges,
		}

		text := moduleInfoToText(moduleInfo)
		embeddings, err := e.embedder.Embed([]string{text})
		if err != nil {
			continue
		}

		if err := e.index.Add(filePath, embeddings[0], unit); err != nil {
			continue
		}

		extractedCount++
	}

	indexPath := filepath.Join(os.TempDir(), "gcq.idx")
	if err := e.index.Save(indexPath); err != nil {
		return nil, fmt.Errorf("saving index: %w", err)
	}

	return &ExtractResult{
		Extracted: extractedCount,
		Total:     len(files),
	}, nil
}

// Context gets LLM-ready context from entry point
func (e *Executor) Context(ctx context.Context, params ContextParams) (*ContextResult, error) {
	if params.Limit <= 0 {
		params.Limit = 5
	}

	results, err := e.searcher.Search(params.Query, params.Limit)
	if err != nil {
		return nil, fmt.Errorf("search error: %w", err)
	}

	contextResults := make([]map[string]interface{}, len(results))
	for i, r := range results {
		contextResults[i] = map[string]interface{}{
			"file":      r.FilePath,
			"line":      r.LineNumber,
			"name":      r.Name,
			"signature": r.Signature,
			"docstring": r.Docstring,
			"type":      r.Type,
			"score":     r.Score,
		}
	}

	return &ContextResult{
		Query:   params.Query,
		Context: contextResults,
	}, nil
}

// Calls gets call graph information for a function
func (e *Executor) Calls(ctx context.Context, params CallsParams) (*CallsResult, error) {
	if params.File == "" || params.Func == "" {
		return nil, fmt.Errorf("file and func are required")
	}

	content, err := os.ReadFile(params.File)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	moduleInfo, err := extractor.ExtractFile(params.File)
	if err != nil {
		return nil, fmt.Errorf("extract error: %w", err)
	}

	cg, err := e.callGraph.BuildFromBytes(content, params.File, moduleInfo)
	if err != nil {
		return nil, fmt.Errorf("call graph error: %w", err)
	}

	var calls []callgraph.CalledFunction
	callType := params.Type
	if callType == "" || callType == "all" {
		calls = cg.GetCalls(params.Func)
	} else {
		switch callType {
		case "local":
			calls = cg.GetLocalCalls(params.Func)
		case "external":
			calls = cg.GetExternalCalls(params.Func)
		case "method":
			calls = cg.GetMethodCalls(params.Func)
		default:
			return nil, fmt.Errorf("invalid call type: %s", callType)
		}
	}

	calledFuncs := make([]CalledFunction, len(calls))
	for i, c := range calls {
		calledFuncs[i] = CalledFunction{
			Name: c.Name,
			File: c.Base,
			Line: c.LineNumber,
			Type: string(c.Type),
		}
	}

	return &CallsResult{
		Function: params.Func,
		File:     params.File,
		Calls:    calledFuncs,
		Count:    len(calls),
	}, nil
}

// Warm builds the semantic index for specified paths
func (e *Executor) Warm(ctx context.Context, params WarmParams) (*WarmResult, error) {
	if len(params.Paths) == 0 {
		return nil, fmt.Errorf("paths are required")
	}

	var totalExtracted int
	for _, path := range params.Paths {
		files, err := e.scanner.Scan(path)
		if err != nil {
			continue
		}

		for _, file := range files {
			filePath := file.FullPath

			moduleInfo, err := extractor.ExtractFile(filePath)
			if err != nil {
				continue
			}

			cg, err := e.callGraph.BuildFromFile(filePath, moduleInfo)
			if err == nil {
				moduleInfo.CallGraph = cg.ToCallGraph()
			}

			unit := types.EmbeddingUnit{
				L1Data: *moduleInfo,
				L2Data: moduleInfo.CallGraph.Edges,
			}

			text := moduleInfoToText(moduleInfo)
			embeddings, err := e.embedder.Embed([]string{text})
			if err != nil {
				continue
			}

			if err := e.index.Add(filePath, embeddings[0], unit); err != nil {
				continue
			}

			totalExtracted++
		}
	}

	indexPath := filepath.Join(os.TempDir(), "gcq.idx")
	if err := e.index.Save(indexPath); err != nil {
		return nil, fmt.Errorf("saving index: %w", err)
	}

	return &WarmResult{
		Extracted: totalExtracted,
		Paths:     params.Paths,
	}, nil
}

// moduleInfoToText converts module info to text for embedding
func moduleInfoToText(m *types.ModuleInfo) string {
	var sb strings.Builder
	sb.WriteString(m.Path)
	sb.WriteString("\n")

	for _, fn := range m.Functions {
		sb.WriteString("def ")
		sb.WriteString(fn.Name)
		sb.WriteString("(")
		sb.WriteString(fn.Params)
		sb.WriteString(")")
		if fn.ReturnType != "" {
			sb.WriteString(" -> ")
			sb.WriteString(fn.ReturnType)
		}
		sb.WriteString("\n")
		if fn.Docstring != "" {
			sb.WriteString(fn.Docstring)
			sb.WriteString("\n")
		}
	}

	for _, cls := range m.Classes {
		sb.WriteString("class ")
		sb.WriteString(cls.Name)
		if len(cls.Bases) > 0 {
			sb.WriteString("(")
			sb.WriteString(strings.Join(cls.Bases, ", "))
			sb.WriteString(")")
		}
		sb.WriteString("\n")
		for _, method := range cls.Methods {
			sb.WriteString("  def ")
			sb.WriteString(method.Name)
			sb.WriteString("(")
			sb.WriteString(method.Params)
			sb.WriteString(")")
			if method.ReturnType != "" {
				sb.WriteString(" -> ")
				sb.WriteString(method.ReturnType)
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
