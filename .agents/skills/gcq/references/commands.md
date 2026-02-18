# gcq CLI Commands Reference

All commands support `--json` / `-j` for JSON output unless noted otherwise.

---

## warm

Build the semantic index for a project.

**Use:** `gcq warm [path]`

**Description:**
Scans the project, extracts code units, generates embeddings, and builds a searchable semantic index. If a daemon is running, delegates to it. Otherwise runs locally. Clears dirty file tracking after a successful build.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |
| `--provider` | `-p` | `""` | Embedding provider for backward compatibility (use `--warm-provider` instead) |
| `--model` | `-m` | `""` | Embedding model name for backward compatibility (use `--warm-model` instead) |
| `--warm-provider` | | `""` | Embedding provider for indexing (ollama or huggingface). Overrides `--provider` |
| `--warm-model` | | `""` | Embedding model name for indexing. Overrides `--model` |
| `--language` | `-l` | `""` | Language to index (auto-detects all by default). Supported: python, go, typescript, javascript, java, rust, c, cpp, ruby, php, swift, kotlin, csharp |
| `--force` | `-f` | `false` | Force full rebuild, ignoring dirty tracking |

**Examples:**

```bash
# Index current directory (auto-detect languages)
gcq warm

# Index a specific project
gcq warm /path/to/project

# Index only Go files
gcq warm --language go .

# Force full rebuild with JSON output
gcq warm --force --json .

# Use a specific provider and model
gcq warm --warm-provider ollama --warm-model nomic-embed-text .
```

---

## semantic

Search the code index using semantic similarity.

**Use:** `gcq semantic <query>`

**Description:**
Performs semantic search over the indexed code to find functions, methods, and classes that match the query. Requires a pre-built index (run `gcq warm` first). Warns if the search provider's embedding dimension differs from the index dimension.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |
| `--provider` | `-p` | `""` | Embedding provider for backward compatibility (ollama or huggingface) |
| `--model` | `-m` | `""` | Embedding model name for backward compatibility |
| `--search-provider` | | `""` | Search-specific embedding provider (ollama or huggingface) |
| `--search-model` | | `""` | Search-specific embedding model name |
| `--k` | `-k` | `10` | Number of results to return |
| `--path` | | `""` | Project path to search (defaults to current directory) |

**Examples:**

```bash
# Search for authentication-related code
gcq semantic "user authentication logic"

# Get top 5 results as JSON
gcq semantic --k 5 --json "error handling"

# Search in a specific project
gcq semantic --path /path/to/project "database connection"

# Use a different search provider
gcq semantic --search-provider huggingface "parse config"
```

---

## context

Get LLM-ready context from an entry point file.

**Use:** `gcq context <entry>`

**Description:**
Analyzes an entry point file and gathers its dependencies, imports, and call graph to provide comprehensive context for LLM processing. Recursively follows imports and call graph edges to collect all related modules.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |
| `--language` | `-l` | `""` | Language of the entry point file |
| `--path` | | `""` | Project root path (defaults to directory containing entry point) |

**Examples:**

```bash
# Gather context from a Python entry point
gcq context src/main.py

# Output as JSON for LLM consumption
gcq context --json app/server.go

# Specify project root explicitly
gcq context --path /project/root src/handler.py
```

---

## calls

Build a call graph for a project.

**Use:** `gcq calls [path]`

**Description:**
Analyzes a project and builds a call graph showing function calls. The call graph includes both intra-file and cross-file edges, plus unresolved calls.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |
| `--language` | `-l` | `""` | Language to analyze (python, go, php, etc.) |

**Examples:**

```bash
# Build call graph for current directory
gcq calls

# Build call graph for a specific path
gcq calls /path/to/project

# Analyze only Go files, output JSON
gcq calls --language go --json .
```

---

## impact

Find all callers of a function.

**Use:** `gcq impact <function>`

**Description:**
Finds all functions that call the specified function. Helps you understand the impact of changing a function. Supports qualified names like `ClassName.method`. Searches through the full call graph and deduplicates results.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |
| `--language` | `-l` | `""` | Language to analyze (python, go, php, etc.) |

**Examples:**

```bash
# Find all callers of a function
gcq impact handleRequest

# Find callers of a method on a class
gcq impact UserService.authenticate

# Output as JSON, filter to Go files
gcq impact --language go --json ProcessOrder
```

---

## extract

Full file analysis.

**Use:** `gcq extract <file>`

**Description:**
Extracts complete module information from a single file, including functions, classes, imports, docstrings, and intra-file call graph edges. Only works on supported file types.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |

**Examples:**

```bash
# Extract structure from a Python file
gcq extract src/models/user.py

# Get JSON output for programmatic use
gcq extract --json pkg/handler.go
```

---

## search

Search for a regex pattern in files.

**Use:** `gcq search [pattern] [path]`

**Description:**
Searches for a regex pattern across all files in the given path. Supports file extension filtering, context lines around matches, and result limits.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |
| `--pattern` | `-p` | `""` | Regex pattern to search for |
| `--context` | `-c` | `0` | Number of context lines before and after match |
| `--ext` | `-e` | `[]` | File extensions to search (can repeat) |
| `--max` | `-m` | `0` | Maximum number of results (0 = unlimited) |

**Examples:**

```bash
# Search for a pattern in current directory
gcq search "func.*test" .

# Filter by extension with context lines
gcq search --ext .go --context 2 "TODO" .

# JSON output with result limit
gcq search --json --max 20 "error" /path/to/project

# Multiple extensions
gcq search --ext .go --ext .py "import" .
```

---

## tree

Display file tree structure.

**Use:** `gcq tree [path]`

**Description:**
Shows a tree view of the directory structure starting from the given path. Directories are listed first, then files, both sorted alphabetically. File language is detected and shown in text output.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |

**Examples:**

```bash
# Show tree for current directory
gcq tree

# Show tree for a specific path
gcq tree /path/to/project

# JSON output for programmatic use
gcq tree --json .
```

---

## structure

Show code structure (functions, classes, imports).

**Use:** `gcq structure [path]`

**Description:**
Analyzes all supported files in the given path and shows their structure including functions, classes, and imports. Works on both single files and directories.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |

**Examples:**

```bash
# Show structure for current directory
gcq structure

# Show structure for a single file
gcq structure src/main.py

# JSON output for a directory
gcq structure --json /path/to/project
```

---

## notify

Mark a file as dirty for tracking.

**Use:** `gcq notify [path]`

**Description:**
Marks a file as dirty by computing its content hash. Used to track which files have changed and need reprocessing during the next `gcq warm` run.

**Flags:**

None beyond the positional path argument. Does not support `--json`.

**Examples:**

```bash
# Mark a file as dirty
gcq notify /path/to/file.go

# Mark a relative path
gcq notify ./src/main.go
```

---

## cfg

Extract control flow graph for a function.

**Use:** `gcq cfg <file> <function>`

**Description:**
Extracts the Control Flow Graph (CFG) for a specific function. Supports Python, Go, TypeScript, Rust, Java, C, C++, Ruby, and PHP. Outputs blocks, edges, and cyclomatic complexity. If the function isn't found, suggests similar names when possible.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |
| `--all` | | `false` | Show all matches if function name is ambiguous |

**Examples:**

```bash
# Extract CFG for a Python function
gcq cfg src/utils.py parse_config

# JSON output for a Go function
gcq cfg --json pkg/server.go HandleRequest

# Show all matches for ambiguous names
gcq cfg --all src/math.py calculate
```

---

## dfg

Extract data flow graph for a function.

**Use:** `gcq dfg <file> <function>`

**Description:**
Extracts the Data Flow Graph (DFG) for a specific function. Supports Python, Go, TypeScript, Rust, Java, C, C++, Ruby, and PHP. Outputs variable references, data flow edges, and variable definitions/uses.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |

**Examples:**

```bash
# Extract DFG for a function
gcq dfg src/handler.py process_request

# JSON output
gcq dfg --json pkg/parser.go ParseTokens
```

---

## slice

Perform backward or forward slice analysis on a function.

**Use:** `gcq slice <file> <function> --line N [--backward|--forward] [--var NAME] [--json]`

**Description:**
Performs program slicing on a specific function to find data and control dependencies. Backward slice finds all lines that may affect the value at the target line. Forward slice finds all lines that may be affected by the value at the source line. Defaults to backward if neither direction is specified. Supports Python, Go, TypeScript, Rust, Java, C, C++, Ruby, and PHP.

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output as JSON |
| `--line` | `-l` | `0` | Line number to slice from (required) |
| `--backward` | `-b` | `false` | Backward slice (default if neither specified) |
| `--forward` | `-f` | `false` | Forward slice |
| `--var` | `-v` | `""` | Variable name to filter (optional) |

**Examples:**

```bash
# Backward slice from line 42
gcq slice src/calc.py compute --line 42

# Forward slice from line 10, filtering by variable
gcq slice src/handler.go Process --line 10 --forward --var result

# JSON output for backward slice
gcq slice --json src/utils.py transform --line 25 --backward
```

---

## init

Initialize gcq configuration.

**Use:** `gcq init`

**Description:**
Guides you through setting up gcq configuration step by step. Creates a config file with warm model (for indexing) and search model settings. Runs interactively by default. Provide flags for non-interactive mode. Runs a health check after saving the config (skippable with `--skip-health-check`).

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--warm-provider` | | `""` | Warm provider: ollama or huggingface (required in non-interactive mode) |
| `--warm-model` | | `""` | Warm model name (optional, has sensible defaults) |
| `--warm-base-url` | | `""` | Ollama base URL (default: http://localhost:11434) |
| `--warm-api-key` | | `""` | Ollama/HuggingFace API key (optional) |
| `--search-provider` | | `""` | Search provider: ollama or huggingface (optional, defaults to warm) |
| `--search-model` | | `""` | Search model name (optional, defaults to warm) |
| `--search-base-url` | | `""` | Search base URL for Ollama (optional) |
| `--search-api-key` | | `""` | Search API key (optional) |
| `--location` | | `""` | Config location: project (default: project) |
| `--yes` | `-y` | `false` | Skip all confirmations, overwrite if exists |
| `--skip-health-check` | | `false` | Skip health check after initialization |

**Examples:**

```bash
# Interactive setup
gcq init

# Non-interactive with Ollama
gcq init --warm-provider ollama --warm-model nomic-embed-text --yes

# Non-interactive with HuggingFace, skip health check
gcq init --warm-provider huggingface --warm-model google/embeddinggemma-300m --skip-health-check --yes

# Separate warm and search providers
gcq init --warm-provider ollama --warm-model nomic-embed-text \
         --search-provider huggingface --search-model sentence-transformers/all-MiniLM-L6-v2 --yes
```

---

## doctor

Run health checks on configuration and models.

**Use:** `gcq doctor`

**Description:**
Checks the configuration and verifies that embedding models are accessible and working properly. Loads config from `.gcq/config.yaml` in the current project. Reports status for both warm and search models. Returns a non-zero exit code if any model is inaccessible.

**Flags:**

None. Does not accept arguments or flags beyond the default help.

**Examples:**

```bash
# Run health check
gcq doctor
```
