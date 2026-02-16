# go-context-query

Semantic code indexing and analysis tool.

## Installation

```bash
# Build from source
make build

# Or install to GOPATH
make install-bin
```

## Configuration

Create `~/.gcq/config.yaml`:

```yaml
provider: ollama
ollama_model: nomic-embed-text
ollama_base_url: http://localhost:11434
socket_path: /tmp/gcq.sock
threshold_similarity: 0.7
max_context_chunks: 10
chunk_size: 512
```

Or use environment variables (see `internal/config/config.go`).

## Usage

### CLI Commands

```bash
# Display file tree structure
gcq tree <path>

# Show code structure (functions, classes, imports)
gcq structure <path>

# Full file analysis
gcq extract <path>

# Get LLM-ready context from entry point
gcq context <path>

# Build call graph for a project
gcq calls <path>

# Find callers of a function
gcq impact <path>

# Build semantic index for a project
gcq warm <paths...>

# Semantic search over indexed code
gcq semantic <query>
```

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
