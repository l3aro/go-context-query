# Installation Guide for LLM Agents

This guide provides direct, scriptable commands for automated installation.

## Build from Source

```bash
# Clone repository
git clone https://github.com/l3aro/go-context-query.git
cd go-context-query

# Build binary
make build
# Output: bin/gcq, bin/gcqd
```

## Non-Interactive Configuration

Use CLI flags to configure without prompts:

```bash
# Initialize config (non-interactive)
./bin/gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --location project \
  --skip-health-check

# Or with custom Ollama URL
./bin/gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --warm-base-url http://localhost:11434 \
  --location project \
  --yes

# Dual provider: Ollama for indexing, HuggingFace for search
./bin/gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --search-provider huggingface \
  --search-model bge-m3 \
  --location project \
  --skip-health-check
```

## Available Flags

| Flag | Description | Required |
|------|-------------|----------|
| `--warm-provider` | Provider for indexing: `ollama` or `huggingface` | Yes (non-interactive) |
| `--warm-model` | Model name for indexing | No (has defaults) |
| `--warm-base-url` | Ollama base URL | No (default: http://localhost:11434) |
| `--warm-api-key` | API key for warm provider | No |
| `--search-provider` | Provider for search | No (defaults to warm) |
| `--search-model` | Model name for search | No |
| `--search-base-url` | Search provider base URL | No |
| `--search-api-key` | API key for search | No |
| `--location` | `global` or `project` | No (default: project) |
| `--yes` / `-y` | Overwrite existing config | No |
| `--skip-health-check` | Skip health check | No |

## Environment Variables Alternative

Create config file manually:

```bash
mkdir -p .gcq
cat > .gcq/config.yaml << 'EOF'
warm_provider: ollama
warm_ollama_model: nomic-embed-text
warm_ollama_base_url: http://localhost:11434
search_provider: ollama
search_ollama_model: nomic-embed-text
search_ollama_base_url: http://localhost:11434
socket_path: /tmp/gcq.sock
threshold_similarity: 0.7
max_context_chunks: 10
chunk_size: 512
EOF
```

Or use environment variables directly:

```bash
export GCQ_PROVIDER=ollama
export GCQ_OLLAMA_MODEL=nomic-embed-text
export GCQ_OLLAMA_BASE_URL=http://localhost:11434
```

## Verify Installation

```bash
# Check doctor output
./bin/gcq doctor

# Test indexing
./bin/gcq warm ./path/to/project

# Test search
./bin/gcq semantic "find auth function"
```

## Common Configurations

### Local Ollama Only

```bash
./bin/gcq init --warm-provider ollama --location project --skip-health-check
```

### Dual Provider (Ollama + HuggingFace)

```bash
./bin/gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --search-provider huggingface \
  --search-model bge-m3 \
  --location project \
  --skip-health-check
```

### Global Config

```bash
./bin/gcq init --warm-provider ollama --location global --skip-health-check
```
