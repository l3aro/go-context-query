// Package main implements the go-context-query daemon (gcqd).
// It provides a Unix domain socket server (with TCP fallback on Windows)
// for indexing and searching code context.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/l3aro/go-context-query/internal/config"
	"github.com/l3aro/go-context-query/internal/scanner"
	"github.com/l3aro/go-context-query/pkg/callgraph"
	"github.com/l3aro/go-context-query/pkg/embed"
	"github.com/l3aro/go-context-query/pkg/extractor"
	"github.com/l3aro/go-context-query/pkg/index"
	"github.com/l3aro/go-context-query/pkg/search"
	"github.com/l3aro/go-context-query/pkg/types"
)

var version = "dev"
var buildTime = ""

const DefaultSocketPath = "/tmp/gcq.sock"
const DefaultTCPPort = "9847"

func isWindows() bool {
	return runtime.GOOS == "windows"
}

type Daemon struct {
	config       *config.Config
	index        *index.VectorIndex
	searcher     *search.Searcher
	textSearcher *search.TextSearcher
	embedder     embed.Provider
	scanner      *scanner.Scanner
	callGraph    *callgraph.Builder
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	indexPath    string
}

func NewDaemon(cfg *config.Config) (*Daemon, error) {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Daemon{
		config:    cfg,
		ctx:       ctx,
		cancel:    cancel,
		indexPath: filepath.Join(os.TempDir(), "gcq.idx"),
	}

	var err error
	d.embedder, err = d.initEmbedder(cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("initializing embedder: %w", err)
	}

	dimension := d.getEmbeddingDimension()
	d.index = index.NewVectorIndex(dimension)

	if err := d.index.Load(d.indexPath); err != nil {
		log.Printf("No existing index found or error loading: %v", err)
	}

	d.searcher = search.NewSearcher(d.embedder, d.index)
	d.textSearcher = search.NewTextSearcher(search.TextSearchOptions{
		ContextLines: 2,
		MaxResults:   100,
	})
	d.scanner = scanner.New(scanner.DefaultOptions())
	d.callGraph = callgraph.NewBuilder()

	return d, nil
}

func (d *Daemon) initEmbedder(cfg *config.Config) (embed.Provider, error) {
	providerType := cfg.WarmProvider
	if providerType == "" {
		providerType = cfg.Provider
	}
	if providerType == "" {
		providerType = "ollama"
	}

	model := cfg.WarmOllamaModel
	if model == "" {
		model = cfg.OllamaModel
	}
	if model == "" {
		model = "nomic-embed-text"
	}

	endpoint := cfg.WarmOllamaBaseURL
	if endpoint == "" {
		endpoint = cfg.OllamaBaseURL
	}
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	apiKey := cfg.WarmOllamaAPIKey
	if apiKey == "" {
		apiKey = cfg.OllamaAPIKey
	}

	embedCfg := &embed.Config{
		Endpoint: endpoint,
		Model:    model,
		APIKey:   apiKey,
	}

	switch providerType {
	case config.ProviderOllama:
		return embed.NewOllamaProvider(embedCfg)
	case config.ProviderHuggingFace:
		hfModel := cfg.WarmHFModel
		if hfModel == "" {
			hfModel = cfg.HFModel
		}
		if hfModel == "" {
			hfModel = "sentence-transformers/all-MiniLM-L6-v2"
		}
		hfToken := cfg.WarmHFToken
		if hfToken == "" {
			hfToken = cfg.HFToken
		}
		return embed.NewHuggingFaceProvider(&embed.Config{
			Model:  hfModel,
			APIKey: hfToken,
		})
	default:
		return embed.NewOllamaProvider(embedCfg)
	}
}

func (d *Daemon) getEmbeddingDimension() int {
	providerType := d.config.WarmProvider
	if providerType == "" {
		providerType = d.config.Provider
	}
	if providerType == "" {
		providerType = "ollama"
	}

	switch providerType {
	case config.ProviderOllama:
		return 768
	case config.ProviderHuggingFace:
		model := d.config.WarmHFModel
		if model == "" {
			model = d.config.HFModel
		}
		if model == "sentence-transformers/all-MiniLM-L6-v2" {
			return 384
		}
		return 384
	default:
		return 768
	}
}

func (d *Daemon) StartSocketServer() error {
	var listener net.Listener
	var err error

	socketPath := d.config.SocketPath
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}

	if isWindows() {
		port := os.Getenv("GCQ_TCP_PORT")
		if port == "" {
			port = DefaultTCPPort
		}
		listener, err = net.Listen("tcp", "localhost:"+port)
		log.Printf("Started TCP server on localhost:%s", port)
	} else {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing existing socket: %w", err)
		}

		listener, err = net.Listen("unix", socketPath)
		if err != nil {
			return fmt.Errorf("listening on socket: %w", err)
		}

		if err := os.Chmod(socketPath, 0777); err != nil {
			return fmt.Errorf("setting socket permissions: %w", err)
		}

		log.Printf("Started Unix socket server on %s", socketPath)
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down server...")
		d.Stop()
		listener.Close()
	}()

	var tempDelay time.Duration
	for {
		conn, err := listener.Accept()
		if err != nil {
			if tempDelay == 0 {
				tempDelay = time.Millisecond
			} else {
				tempDelay *= 2
			}
			if tempDelay > time.Second {
				tempDelay = time.Second
			}
			select {
			case <-time.After(tempDelay):
				if d.ctx.Err() != nil {
					return nil
				}
				continue
			case <-d.ctx.Done():
				return nil
			}
		}
		tempDelay = 0

		go d.handleConnection(conn)
	}
}

func (d *Daemon) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		select {
		case <-d.ctx.Done():
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		var cmd Command
		if err := decoder.Decode(&cmd); err != nil {
			if err == io.EOF {
				return
			}
			encoder.Encode(Response{
				Error: fmt.Sprintf("decode error: %v", err),
			})
			continue
		}

		resp := d.handleCommand(cmd)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Encode error: %v", err)
			return
		}

		conn.SetReadDeadline(time.Time{})
	}
}

type Command struct {
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params,omitempty"`
	ID     string          `json:"id,omitempty"`
}

type Response struct {
	ID     string          `json:"id,omitempty"`
	Type   string          `json:"type,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (d *Daemon) handleCommand(cmd Command) Response {
	switch cmd.Type {
	case "status":
		return d.handleStatus(cmd)
	case "search":
		return d.handleSearch(cmd)
	case "extract":
		return d.handleExtract(cmd)
	case "context":
		return d.handleContext(cmd)
	case "calls":
		return d.handleCalls(cmd)
	case "warm":
		return d.handleWarm(cmd)
	case "stop":
		return d.handleStop(cmd)
	default:
		return Response{
			ID:    cmd.ID,
			Error: fmt.Sprintf("unknown command: %s", cmd.Type),
		}
	}
}

func (d *Daemon) handleStatus(cmd Command) Response {
	d.mu.RLock()
	defer d.mu.RUnlock()

	count, dim := d.searcher.IndexStats()

	result := map[string]interface{}{
		"version":     version,
		"status":      "running",
		"index_count": count,
		"dimension":   dim,
		"provider":    d.config.Provider,
		"model":       d.getModelName(),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("marshal error: %v", err)}
	}

	return Response{
		ID:     cmd.ID,
		Type:   "status",
		Result: resultJSON,
	}
}

func (d *Daemon) getModelName() string {
	switch d.config.Provider {
	case config.ProviderOllama:
		return d.config.OllamaModel
	case config.ProviderHuggingFace:
		return d.config.HFModel
	default:
		return "unknown"
	}
}

type SearchParams struct {
	Query     string  `json:"query"`
	Limit     int     `json:"limit,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
	Mode      string  `json:"mode,omitempty"` // "semantic" (default) or "text"
	Root      string  `json:"root,omitempty"` // root directory for text search
}

func (d *Daemon) handleSearch(cmd Command) Response {
	var params SearchParams
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("invalid params: %v", err)}
	}

	if params.Query == "" {
		return Response{ID: cmd.ID, Error: "query is required"}
	}

	if params.Limit <= 0 {
		params.Limit = 10
	}

	// Default to semantic mode if not specified
	if params.Mode == "" {
		params.Mode = "semantic"
	}

	if params.Mode == "text" {
		return d.handleTextSearch(cmd, params)
	}

	// Semantic search (existing behavior)
	results, err := d.searcher.Search(params.Query, params.Limit)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("search error: %v", err)}
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

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("marshal error: %v", err)}
	}

	return Response{
		ID:     cmd.ID,
		Type:   "search",
		Result: resultJSON,
	}
}

func (d *Daemon) handleTextSearch(cmd Command, params SearchParams) Response {
	if params.Root == "" {
		return Response{ID: cmd.ID, Error: "root is required for text search"}
	}

	ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
	defer cancel()

	matches, err := d.textSearcher.Search(ctx, params.Query, params.Root)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("text search error: %v", err)}
	}

	// Apply limit
	if params.Limit > 0 && len(matches) > params.Limit {
		matches = matches[:params.Limit]
	}

	result := map[string]interface{}{
		"mode":    "text",
		"query":   params.Query,
		"root":    params.Root,
		"matches": matches,
		"count":   len(matches),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("marshal error: %v", err)}
	}

	return Response{
		ID:     cmd.ID,
		Type:   "search",
		Result: resultJSON,
	}
}

type ExtractParams struct {
	Path string `json:"path"`
}

func (d *Daemon) handleExtract(cmd Command) Response {
	var params ExtractParams
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("invalid params: %v", err)}
	}

	if params.Path == "" {
		return Response{ID: cmd.ID, Error: "path is required"}
	}

	files, err := d.scanner.Scan(params.Path)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("scan error: %v", err)}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	var extractedCount int
	for _, file := range files {
		filePath := file.FullPath

		moduleInfo, err := extractor.ExtractFile(filePath)
		if err != nil {
			log.Printf("Error extracting %s: %v", filePath, err)
			continue
		}

		cg, err := d.callGraph.BuildFromFile(filePath, moduleInfo)
		if err != nil {
			log.Printf("Error building call graph for %s: %v", filePath, err)
		} else {
			moduleInfo.CallGraph = cg.ToCallGraph()
		}

		unit := types.EmbeddingUnit{
			L1Data: *moduleInfo,
			L2Data: moduleInfo.CallGraph.Edges,
		}

		text := moduleInfoToText(moduleInfo)
		embeddings, err := d.embedder.Embed([]string{text})
		if err != nil {
			log.Printf("Error embedding %s: %v", filePath, err)
			continue
		}

		if err := d.index.Add(filePath, embeddings[0], unit); err != nil {
			log.Printf("Error adding to index: %v", err)
			continue
		}

		extractedCount++
	}

	if err := d.index.Save(d.indexPath); err != nil {
		log.Printf("Error saving index: %v", err)
	}

	result := map[string]interface{}{
		"extracted": extractedCount,
		"total":     len(files),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("marshal error: %v", err)}
	}

	return Response{
		ID:     cmd.ID,
		Type:   "extract",
		Result: resultJSON,
	}
}

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

type ContextParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

func (d *Daemon) handleContext(cmd Command) Response {
	var params ContextParams
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("invalid params: %v", err)}
	}

	if params.Query == "" {
		return Response{ID: cmd.ID, Error: "query is required"}
	}

	if params.Limit <= 0 {
		params.Limit = 5
	}

	results, err := d.searcher.Search(params.Query, params.Limit)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("search error: %v", err)}
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

	result := map[string]interface{}{
		"context": contextResults,
		"query":   params.Query,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("marshal error: %v", err)}
	}

	return Response{
		ID:     cmd.ID,
		Type:   "context",
		Result: resultJSON,
	}
}

type CallsParams struct {
	File string `json:"file"`
	Func string `json:"func"`
	Type string `json:"type,omitempty"`
}

func (d *Daemon) handleCalls(cmd Command) Response {
	var params CallsParams
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("invalid params: %v", err)}
	}

	if params.File == "" || params.Func == "" {
		return Response{ID: cmd.ID, Error: "file and func are required"}
	}

	content, err := os.ReadFile(params.File)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("read error: %v", err)}
	}

	moduleInfo, err := extractor.ExtractFile(params.File)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("extract error: %v", err)}
	}

	cg, err := d.callGraph.BuildFromBytes(content, params.File, moduleInfo)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("call graph error: %v", err)}
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
			return Response{ID: cmd.ID, Error: fmt.Sprintf("invalid call type: %s", callType)}
		}
	}

	result := map[string]interface{}{
		"function": params.Func,
		"file":     params.File,
		"calls":    calls,
		"count":    len(calls),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("marshal error: %v", err)}
	}

	return Response{
		ID:     cmd.ID,
		Type:   "calls",
		Result: resultJSON,
	}
}

type WarmParams struct {
	Paths []string `json:"paths,omitempty"`
}

func (d *Daemon) handleWarm(cmd Command) Response {
	var params WarmParams
	if err := json.Unmarshal(cmd.Params, &params); err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("invalid params: %v", err)}
	}

	if len(params.Paths) == 0 {
		return Response{ID: cmd.ID, Error: "paths are required"}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	var totalExtracted int
	for _, path := range params.Paths {
		files, err := d.scanner.Scan(path)
		if err != nil {
			log.Printf("Error scanning %s: %v", path, err)
			continue
		}

		for _, file := range files {
			filePath := file.FullPath

			moduleInfo, err := extractor.ExtractFile(filePath)
			if err != nil {
				continue
			}

			cg, err := d.callGraph.BuildFromFile(filePath, moduleInfo)
			if err == nil {
				moduleInfo.CallGraph = cg.ToCallGraph()
			}

			unit := types.EmbeddingUnit{
				L1Data: *moduleInfo,
				L2Data: moduleInfo.CallGraph.Edges,
			}

			text := moduleInfoToText(moduleInfo)
			embeddings, err := d.embedder.Embed([]string{text})
			if err != nil {
				continue
			}

			if err := d.index.Add(filePath, embeddings[0], unit); err != nil {
				continue
			}

			totalExtracted++
		}
	}

	if err := d.index.Save(d.indexPath); err != nil {
		log.Printf("Error saving index: %v", err)
	}

	result := map[string]interface{}{
		"extracted": totalExtracted,
		"paths":     params.Paths,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("marshal error: %v", err)}
	}

	return Response{
		ID:     cmd.ID,
		Type:   "warm",
		Result: resultJSON,
	}
}

func (d *Daemon) handleStop(cmd Command) Response {
	d.Stop()

	result := map[string]interface{}{
		"status": "stopped",
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return Response{ID: cmd.ID, Error: fmt.Sprintf("marshal error: %v", err)}
	}

	return Response{
		ID:     cmd.ID,
		Type:   "stop",
		Result: resultJSON,
	}
}

func (d *Daemon) Stop() {
	d.cancel()
}

func main() {
	socketPath := ""
	configPath := ""
	verbose := false

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-socket", "--socket":
			if i+1 < len(os.Args) {
				socketPath = os.Args[i+1]
				i++
			}
		case "-config", "--config":
			if i+1 < len(os.Args) {
				configPath = os.Args[i+1]
				i++
			}
		case "-v", "--verbose", "-verbose":
			verbose = true
		case "-version", "--version":
			fmt.Printf("gcqd version %s\n", version)
			os.Exit(0)
		case "-h", "--help", "-help":
			fmt.Println("Usage: gcqd [options]")
			fmt.Println("Options:")
			fmt.Println("  -socket PATH   Unix socket path (default: /tmp/gcq.sock)")
			fmt.Println("  -config PATH   Config file path")
			fmt.Println("  -v, -verbose  Verbose logging")
			fmt.Println("  -h, -help     Show this help")
			os.Exit(0)
		}
	}

	var cfg *config.Config
	var err error
	if configPath != "" {
		cfg, err = config.LoadFromFile(configPath)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if socketPath != "" {
		cfg.SocketPath = socketPath
	}

	if verbose || cfg.Verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetFlags(0)
		log.SetOutput(io.Discard)
	}

	daemon, err := NewDaemon(cfg)
	if err != nil {
		log.Fatalf("Failed to create daemon: %v", err)
	}

	log.Printf("Starting gcqd v%s", version)

	if err := daemon.StartSocketServer(); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("gcqd stopped")
}
