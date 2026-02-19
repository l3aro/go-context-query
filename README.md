# go-context-query

Semantic code indexing and analysis tool.

## Installation

For detailed installation instructions, see the [Installation Guide](docs/installation/human.md).

### Quick Install

```bash
# Build from source
make build

# Or install to GOPATH
make install-bin
```

---

## For LLM Agents

Paste this into your LLM agent session:

```
Install and configure go-context-query by following the instructions here:
https://raw.githubusercontent.com/l3aro/go-context-query/main/docs/installation/agent.md
```

## Configuration

Create `~/.gcq/config.yaml`:

```yaml
# Warm (indexing) provider settings
warm_provider: ollama
warm_ollama_model: nomic-embed-text
warm_ollama_base_url: http://localhost:11434

# Search provider settings (optional - inherits from warm if not set)
search_provider: ollama
search_ollama_model: nomic-embed-text
search_ollama_base_url: http://localhost:11434

# Shared settings
socket_path: /tmp/gcq.sock
threshold_similarity: 0.7
threshold_min_score: 0.5
max_context_chunks: 10
chunk_overlap: 100
chunk_size: 512
verbose: false
```

Or use environment variables (see `internal/config/config.go`).

### Dual Provider Configuration

You can use different embedding providers for indexing (warming) and search operations:

```yaml
# Dual provider: use Ollama for indexing, HuggingFace for search
warm_provider: ollama
warm_ollama_model: nomic-embed-text
warm_ollama_base_url: http://localhost:11434

search_provider: huggingface
search_hf_model: sentence-transformers/all-MiniLM-L6-v2

# Shared settings
socket_path: /tmp/gcq.sock
threshold_similarity: 0.7
max_context_chunks: 10
chunk_size: 512
```

Or use different Ollama endpoints:

```yaml
# Dual Ollama: fast local model for indexing, cloud endpoint for search
warm_provider: ollama
warm_ollama_model: nomic-embed-text
warm_ollama_base_url: http://localhost:11434

search_provider: ollama
search_ollama_model: bge-m3
search_ollama_base_url: https://ollama.example.com
```

## Usage

### CLI Commands

```bash
# Build semantic index (uses warm_provider or provider)
gcq warm <paths...>

# Semantic search (uses search_provider or provider)
gcq semantic <query>

# Override provider via flags
gcq warm --warm-provider ollama ./myproject
gcq semantic --search-provider huggingface "find auth code"
```

### Provider Flags

| Flag | Command | Description |
|------|---------|-------------|
| `--provider`, `-p` | warm, semantic | Legacy provider flag (ollama or huggingface) |
| `--warm-provider` | warm | Provider for indexing operations |
| `--search-provider` | semantic | Provider for search queries |

Provider priority: flag > config > default (ollama)

### Daemon

Start the daemon for persistent indexing:

```bash
./bin/gcqd
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `build` | Build gcq and gcqd binaries |
| `build-all` | Build for all platforms |
| `test` | Run tests with coverage |
| `clean` | Clean build artifacts |
| `install` | Install Go dependencies |
| `install-bin` | Install binaries to GOPATH/bin |
