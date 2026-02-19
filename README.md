# go-context-query

Semantic code indexing and analysis tool for AI-powered code understanding.

## Features

- **Semantic Search** - Find code using natural language queries
- **Call Graph Analysis** - Understand function relationships and dependencies
- **Impact Analysis** - Find all callers of a specific function
- **Code Context** - Generate LLM-ready context from entry points
- **Code Structure** - Extract functions, classes, and imports
- **Control Flow Analysis** - CFG and DFG visualization
- **Code Slicing** - Backward/forward data flow analysis

## Installation

For detailed installation instructions, see the [Installation Guide](docs/installation/human.md).

### Quick Install

```bash
# Build from source
make build

# Or install to GOPATH
make install-bin
```

### Download Binary

Visit the [Releases](https://github.com/l3aro/go-context-query/releases) page for pre-built binaries.

---

## For LLM Agents

Paste this into your LLM agent session:

```
Install and configure go-context-query by following the instructions here:
https://raw.githubusercontent.com/l3aro/go-context-query/main/docs/installation/agent.md
```

---

## Quick Start

```bash
# Initialize configuration (first time only)
./bin/gcq init

# Build semantic index
./bin/gcq warm ./your-project

# Search code semantically
./bin/gcq semantic "find authentication logic"

# Get LLM-ready context
./bin/gcq context ./your-project/main.go
```

## CLI Commands

### Semantic Search

```bash
# Build semantic index
gcq warm <paths...>

# Search indexed code
gcq semantic "find user authentication"
```

### Call Graph Analysis

```bash
# Build call graph
gcq calls ./your-project

# Find all functions that call a specific function
gcq impact ./your-project --function ValidateUser
```

### Code Context

```bash
# Get LLM-ready context from entry point
gcq context ./your-project/main.go
```

### Code Structure

```bash
# Display file tree
gcq tree ./your-project

# Show code structure (functions, classes, imports)
gcq structure ./your-project

# Full file analysis
gcq extract ./your-project/main.go
```

### Control Flow Analysis

```bash
# Control flow graph
gcq cfg ./your-project/main.go --function main

# Data flow graph
gcq dfg ./your-project/main.go --function main

# Code slicing
gcq slice ./your-project/main.go --line 42
```

### Traditional Search

```bash
# Regex search
gcq search "func.*auth" ./your-project
```

### Utilities

```bash
# Verify setup
gcq doctor

# Mark file as dirty (for tracking changes)
gcq notify ./your-project/main.go
```

## Configuration

### Config File

Create `~/.gcq/config.yaml` or `.gcq/config.yaml`:

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

### Environment Variables

```bash
export GCQ_PROVIDER=ollama
export GCQ_OLLAMA_MODEL=nomic-embed-text
export GCQ_OLLAMA_BASE_URL=http://localhost:11434
```

### Provider Flags

Override providers via CLI flags:

```bash
gcq warm --warm-provider ollama ./myproject
gcq semantic --search-provider huggingface "find auth code"
```

| Flag | Command | Description |
|------|---------|-------------|
| `--provider`, `-p` | warm, semantic | Legacy provider flag |
| `--warm-provider` | warm | Provider for indexing |
| `--search-provider` | semantic | Provider for search |

Provider priority: flag > config > default (ollama)

### Dual Provider

Use different embedding providers for indexing and search:

```yaml
warm_provider: ollama
warm_ollama_model: nomic-embed-text

search_provider: huggingface
search_hf_model: bge-m3
```

## Daemon

The daemon provides persistent indexing and faster queries by keeping the index loaded in memory.

### Quick Start

```bash
# Start daemon for current project (uses cwd)
./bin/gcq start

# Start daemon for specific project
./bin/gcq start --project /path/to/project

# Check daemon status
./bin/gcq status

# Stop daemon
./bin/gcq stop
```

### Per-Project Isolation

Each project gets its own daemon with isolated socket and index:

```bash
# Project A daemon: /tmp/gcq-{hash_a}.sock
cd /path/to/project-a
./bin/gcq start -d

# Project B daemon: /tmp/gcq-{hash_b}.sock  
cd /path/to/project-b
./bin/gcq start -d
```

Socket path is computed from project path hash: `/tmp/gcq-{md5(project_path)[:8]}.sock`

Index is stored in: `{project}/.gcq/index.idx`

### Daemon Commands

```bash
# Start daemon in background
./bin/gcq start -d
./bin/gcq start -d --project /path/to/project

# Check status
./bin/gcq status
./bin/gcq status --project /path/to/project

# Stop daemon
./bin/gcq stop
./bin/gcq stop --project /path/to/project
```

### Notify (File Change Tracking)

Tell the daemon that files have changed:

```bash
# Single file
./bin/gcq notify ./src/auth.go

# Multiple files (via git)
git diff --name-only HEAD | xargs -I{} ./bin/gcq notify {}

# Or send directly via socket
echo '{"type": "notify", "params": {"path": "./src/main.go"}}' | nc -U /tmp/gcq-{hash}.sock
```

When dirty file count reaches threshold (20), daemon automatically reindexes in background.

### Direct Daemon Binary

Run daemon directly:

```bash
# With explicit project path
./bin/gcqd -project /path/to/project

# With explicit socket
./bin/gcqd -socket /tmp/custom.sock

# Daemon stays running until stopped (Ctrl+C)
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `build` | Build gcq and gcqd binaries |
| `build-all` | Build for all platforms |
| `test` | Run tests with coverage |
| `test-no-cov` | Run tests without coverage |
| `clean` | Clean build artifacts |
| `lint` | Run linters |
| `fmt` | Format code |

## License

MIT
