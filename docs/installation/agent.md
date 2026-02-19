# Installation Guide for LLM Agents

This guide provides direct, scriptable commands for automated installation.

## Option 1: Download Standalone Binary

```bash
# Detect OS and arch, download latest release
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
  x86_64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
esac

curl -sL "https://github.com/l3aro/go-context-query/releases/latest/download/gcq-${OS}-${ARCH}" -o gcq
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
git clone https://github.com/l3aro/go-context-query.git

# Copy skill to agent skills directory
cp -r go-context-query/.agents/skills/gcq ~/.agents/skills/

# Or download just the skill file
mkdir -p ~/.agents/skills/gcq
curl -sL https://raw.githubusercontent.com/l3aro/go-context-query/main/.agents/skills/gcq/SKILL.md \
  -o ~/.agents/skills/gcq/SKILL.md
```

The skill provides:
- Command reference for all 15 gcq commands
- Workflows for common analysis tasks
- Configuration guidance

## Daemon

For persistent indexing and faster queries, use the daemon:

### Start Daemon

```bash
# Current project
./gcq start -d

# Specific project
./gcq start -d --project /path/to/project

# With verbose logging
./gcq start -d -v
```

### Daemon Commands

```bash
# Status
./gcq status
./gcq status --project /path/to/project

# Stop
./gcq stop
./gcq stop --project /path/to/project
```

### Per-Project Isolation

Each project gets isolated socket and index:

```bash
# Project A
cd /path/to/project-a
./gcq start -d
# Socket: /tmp/gcq-{md5_hash_a}.sock

# Project B (separate)
cd /path/to/project-b  
./gcq start -d
# Socket: /tmp/gcq-{md5_hash_b}.sock
```

Socket path: `/tmp/gcq-{md5(project_path[:8])}.sock`
Index path: `{project}/.gcq/index.idx`

### Notify (File Changes)

```bash
# Single file
./gcq notify ./src/auth.go

# Batch via git
git diff --name-only HEAD | xargs -I{} ./gcq notify {}

# Via socket directly
echo '{"type": "notify", "params": {"path": "./main.go"}}' | nc -U /tmp/gcq-{hash}.sock -w 2
```

Auto-reindex triggers at 20 dirty files.

## Agent Configuration Interview Pattern

When helping users configure gcq, agents should use the `question` tool to interview users:

### Questions to Ask

1. **What type of code analysis do you need?**
   - Guide user to choose: semantic search, call graph, context gathering, or code structure

2. **What embedding provider do you use?**
   - Ollama (local) or HuggingFace (cloud)

3. **Do you need dual provider?**
   - Different models for indexing vs search

4. **What's your project path?**
   - Where to run gcq commands

### Flag Mapping

After interviewing, map answers to gcq init flags:
- Ollama → `--warm-provider ollama`
- HuggingFace → `--warm-provider huggingface`
- With search → add `--search-provider` flags
- Skip health check → `--skip-health-check`
- Install skill → `--skill`

### Example

```bash
gcq init --warm-provider ollama --warm-model nomic-embed-text --skip-health-check --skill
```

## LLM Agent Interview Pattern (Init Command)

When helping users set up gcq for the first time, replicate the `gcq init` interactive flow by asking questions step-by-step. Use the `question` tool to gather configuration details.

**IMPORTANT**: NEVER assume user preferences. ALWAYS ask explicitly, even for optional settings. Do not use default values unless the user explicitly confirms them.

### Step 0: Check Existing Installation

Before starting the interview, check if gcq is already installed:

```bash
# Check if gcq exists
which gcq || echo "not found"

# Check current version
gcq --version
```

Ask: **"I found gcq is already installed. Would you like me to update it to the latest version, or can we proceed with the current version?"**

If user wants to update:
```bash
# Download latest release
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
  x86_64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
esac

curl -sL "https://github.com/l3aro/go-context-query/releases/latest/download/gcq-${OS}-${ARCH}" -o gcq
chmod +x gcq

# Replace existing binary
mv gcq ~/.local/bin/  # or wherever gcq is installed
```

### Step 1: Warm Model Provider

Ask: **"What embedding provider do you want to use for indexing/warming your codebase?"**

Options:
- Ollama (local, privacy-friendly)
- HuggingFace (cloud-based)

**Wait for user answer before proceeding.**

### Step 2: Provider-Specific Details

**If Ollama selected:**
- Ask: **"What is your Ollama base URL?"** (e.g., http://localhost:11434)
- Ask: **"Which Ollama model do you want to use?"**
- Ask: **"Do you have an Ollama API key?"** (optional)

**If HuggingFace selected:**
- Ask: **"Which HuggingFace model do you want to use?"**
- Ask: **"Do you have a HuggingFace API token?"** (optional)

**Wait for user answers before proceeding.**

### Step 3: Search Model

Ask: **"Do you want to use the same model for both indexing and search, or configure a different one for search?"**

- **Same model** → Search inherits warm settings
- **Different model** → Follow Step 2 questions again for search provider

**Wait for user answer before proceeding.**

### Step 4: Project Location

Ask: **"Where should the configuration be saved?"** (typically the current project directory)

### Step 5: Generate Command

After collecting all answers, generate the appropriate gcq init command:

```bash
# Example: Ollama only
./gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --warm-base-url http://localhost:11434 \
  --skip-health-check

# Example: Dual provider (Ollama + HuggingFace)
./gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --search-provider huggingface \
  --search-model bge-m3 \
  --skip-health-check

# Example: Full configuration
./gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --warm-base-url http://localhost:11434 \
  --search-provider huggingface \
  --search-model bge-m3 \
  --skip-health-check \
  --skill
```

### Step 6: Offer Skill Installation

After initialization, ask: **"Would you like me to install the gcq skill for your agent?"**

This enables:
- Automatic discovery of gcq capabilities
- Context-aware code analysis
- Semantic search integration

```bash
./gcq init --skill
# Or add --skill to any init command above
```

### Complete Interview Flow Example

```
Agent: Hi! I'll help you set up gcq for semantic code analysis.

Agent: First, let me check if gcq is already installed...

(which gcq output)

Agent: gcq is already installed (version X.X.X). Would you like me to update it to the latest version, or can we proceed with the current version?

User: Let's update to the latest version

Agent: Great! Updating gcq...
(download and install latest version)

Agent: Now, let's configure gcq. What embedding provider do you want to use for indexing/warming your codebase?
  - Ollama (local, privacy-friendly)
  - HuggingFace (cloud-based)

User: Ollama

Agent: What is your Ollama base URL?

User: http://localhost:11434

Agent: Which Ollama model do you want to use?

User: nomic-embed-text

Agent: Do you have an Ollama API key? (optional)

User: No thanks

Agent: Do you want to use the same model for both indexing and search, or configure a different one for search?

User: Same is fine

Agent: Great! Here's the command based on your answers:

./gcq init \
  --warm-provider ollama \
  --warm-model nomic-embed-text \
  --warm-base-url http://localhost:11434 \
  --skip-health-check

Would you like me to:
1. Run this command now?
2. Add the --skill flag to install the agent skill?
3. Customize any options?
```
