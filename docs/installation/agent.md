# Installation Guide for LLM Agents

This guide provides direct, scriptable commands for automated installation.

## Option 1: Download Standalone Binary

```bash
# Detect OS and arch, download latest release
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
  x86_64) ARCH="x86_64" ;;
  arm64|aarch64) ARCH="arm64" ;;
esac

curl -sL "https://github.com/vectordotdev/go-context-query/releases/latest/download/gcq_${OS}_${ARCH}.tar.gz" | tar xz

chmod +x gcq

# Setup binary location
if [ ! -d "$HOME/.local/bin" ]; then
    mkdir -p "$HOME/.local/bin"
fi
mv gcq "$HOME/.local/bin/"

# Add to PATH if not already present
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    export PATH="$HOME/.local/bin:$PATH"
fi
```

Binary path: `./gcq` (use this instead of `./bin/gcq` in examples below)

## Option 2: Build from Source

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
./gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --location project \
  --skip-health-check

# Or with custom Ollama URL
./gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --warm-base-url http://localhost:11434 \
  --location project \
  --yes

# Dual provider: Ollama for indexing, HuggingFace for search
./gcq init \
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
./gcq doctor

# Test indexing
./gcq warm ./path/to/project

# Test search
./gcq semantic "find auth function"
```

## Common Configurations

### Local Ollama Only

```bash
./gcq init --warm-provider ollama --location project --skip-health-check
```

### Dual Provider (Ollama + HuggingFace)

```bash
./gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --search-provider huggingface \
  --search-model bge-m3 \
  --location project \
  --skip-health-check
```

### Global Config

```bash
./gcq init --warm-provider ollama --location global --skip-health-check
```

## Using gcq with Agents

gcq includes a skill definition for AI agents. When installed, agents can automatically discover and use gcq for:
- Semantic code search and indexing
- Call graph analysis
- Context gathering for LLM processing
- Code structure extraction

Agents with skill support will automatically use gcq when you mention:
- "semantic search"
- "find related code"
- "call graph"
- "code context"
- "understand codebase"

## Installing gcq Skill

For agents that support skill loading, install the gcq skill:

```bash
# Clone the repository with skills
git clone https://github.com/vectordotdev/go-context-query.git

# Copy skill to agent skills directory
cp -r go-context-query/.agents/skills/gcq ~/.agents/skills/

# Or download just the skill file
mkdir -p ~/.agents/skills/gcq
curl -sL https://raw.githubusercontent.com/vectordotdev/go-context-query/main/.agents/skills/gcq/SKILL.md \
  -o ~/.agents/skills/gcq/SKILL.md
```

The skill provides:
- Command reference for all 15 gcq commands
- Workflows for common analysis tasks
- Configuration guidance
