# Configuration Reference

## Config File Format

GCQ uses YAML format for configuration files.

## Config File Locations

GCQ looks for configuration in the following locations (in order of precedence):

1. `.gcq/config.yaml` - Project-level config (checked first)
2. `~/.gcq/config.yaml` - User-level config (fallback)

Project-level config takes precedence over user-level config. If no config file is found, GCQ will prompt you to run `gcq init` to create one.

## Environment Variables

All configuration options can be overridden using environment variables. Environment variables take precedence over config file values.

### General Settings

| Environment Variable | Description | Default |
|----------------------|-------------|---------|
| `GCQ_SOCKET_PATH` | Socket path for IPC communication | `/tmp/gcq.sock` |
| `GCQ_THRESHOLD_SIMILARITY` | Minimum similarity score for results (0-1) | `0.7` |
| `GCQ_THRESHOLD_MIN_SCORE` | Minimum score threshold (0-1) | `0.5` |
| `GCQ_MAX_CONTEXT_CHUNKS` | Maximum number of context chunks to return | `10` |
| `GCQ_CHUNK_OVERLAP` | Number of overlapping tokens between chunks | `100` |
| `GCQ_CHUNK_SIZE` | Size of each text chunk in tokens | `512` |
| `GCQ_VERBOSE` | Enable verbose logging | `false` |

### Dual Provider Settings (Warm/Search)

When using dual provider mode, you can set separate providers for indexing (warm) and search operations:

| Environment Variable | Description |
|----------------------|-------------|
| `GCQ_WARM_PROVIDER` | Provider for indexing operations (ollama/huggingface) |
| `GCQ_WARM_HF_MODEL` | HuggingFace model for warm provider |
| `GCQ_WARM_HF_TOKEN` | HuggingFace API token for warm provider |
| `GCQ_WARM_OLLAMA_MODEL` | Ollama model for warm provider |
| `GCQ_WARM_OLLAMA_BASE_URL` | Ollama base URL for warm provider |
| `GCQ_WARM_OLLAMA_API_KEY` | Ollama API key for warm provider |
| `GCQ_SEARCH_PROVIDER` | Provider for search operations (ollama/huggingface) |
| `GCQ_SEARCH_HF_MODEL` | HuggingFace model for search provider |
| `GCQ_SEARCH_HF_TOKEN` | HuggingFace API token for search provider |
| `GCQ_SEARCH_OLLAMA_MODEL` | Ollama model for search provider |
| `GCQ_SEARCH_OLLAMA_BASE_URL` | Ollama base URL for search provider |
| `GCQ_SEARCH_OLLAMA_API_KEY` | Ollama API key for search provider |

### Legacy Settings (Single Provider)

For backward compatibility, you can use a single provider for both operations:

| Environment Variable | Description |
|----------------------|-------------|
| `GCQ_PROVIDER` | Provider type (ollama/huggingface) |
| `GCQ_HF_MODEL` | HuggingFace model name |
| `GCQ_HF_TOKEN` | HuggingFace API token |
| `GCQ_OLLAMA_MODEL` | Ollama model name |
| `GCQ_OLLAMA_BASE_URL` | Ollama server URL |
| `GCQ_OLLAMA_API_KEY` | Ollama API key |

## Config Options

### Warm Provider (Indexing)

Configuration for the embedding provider used during indexing operations.

| Option | Type | Description | Required |
|--------|------|-------------|----------|
| `warm.provider` | string | Provider type: `ollama` or `huggingface` | Yes* |
| `warm.model` | string | Model identifier | Yes* |
| `warm.base_url` | string | Server base URL | For Ollama |
| `warm.token` | string | API token or key | For authenticated endpoints |

*Required when using that specific provider.

### Search Provider

Configuration for the embedding provider used during search operations.

| Option | Type | Description | Required |
|--------|------|-------------|----------|
| `search.provider` | string | Provider type: `ollama` or `huggingface` | Yes* |
| `search.model` | string | Model identifier | Yes* |
| `search.base_url` | string | Server base URL | For Ollama |
| `search.token` | string | API token or key | For authenticated endpoints |

*Required when using that specific provider.

### Legacy Provider (Fallback)

If warm/search providers are not specified, these legacy options are used:

| Option | Type | Description | Required |
|--------|------|-------------|----------|
| `provider` | string | Provider type: `ollama` or `huggingface` | Yes |
| `hf_model` | string | HuggingFace model name | When provider is huggingface |
| `hf_token` | string | HuggingFace API token | When using private models |
| `ollama_model` | string | Ollama model name | When provider is ollama |
| `ollama_base_url` | string | Ollama server URL | When provider is ollama |
| `ollama_api_key` | string | Ollama API key | For authenticated endpoints |

### Context Gathering

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `socket_path` | string | `/tmp/gcq.sock` | Unix socket path for daemon IPC |
| `threshold_similarity` | float | `0.7` | Minimum similarity score (0.0-1.0) |
| `threshold_min_score` | float | `0.5` | Minimum score threshold (0.0-1.0) |
| `max_context_chunks` | int | `10` | Maximum chunks to include in context |
| `chunk_overlap` | int | `100` | Overlapping tokens between chunks |
| `chunk_size` | int | `512` | Size of each chunk in tokens |
| `verbose` | bool | `false` | Enable detailed logging |

## Provider Setup

### Ollama

Ollama provides local embedding model hosting. Download from [ollama.ai](https://ollama.ai).

#### Local Ollama

```yaml
warm:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:11434
```

#### Cloud Ollama Endpoint

```yaml
warm:
  provider: ollama
  model: bge-m3
  base_url: https://ollama.example.com
  token: your-api-key
```

#### Recommended Ollama Models

- `nomic-embed-text` - Fast, general purpose
- `bge-m3` - High-quality multilingual embeddings
- `mxbai-embed-large` - Large model for better accuracy

### HuggingFace

HuggingFace provides cloud-hosted embedding models via their Inference API.

```yaml
warm:
  provider: huggingface
  model: sentence-transformers/all-MiniLM-L6-v2
  token: hf_your-token-here
```

#### Recommended HuggingFace Models

- `sentence-transformers/all-MiniLM-L6-v2` - Fast, 384 dimensions
- `sentence-transformers/all-mpnet-base-v2` - Higher quality, 768 dimensions
- `BAAI/bge-large-en-v1.5` - State-of-the-art English embeddings

## Example Configs

### Single Ollama Provider

Use one Ollama instance for both indexing and search:

```yaml
warm:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:11434

socket_path: /tmp/gcq.sock
threshold_similarity: 0.7
threshold_min_score: 0.5
max_context_chunks: 10
chunk_overlap: 100
chunk_size: 512
verbose: false
```

### Dual Providers: Local and Cloud Ollama

Use a fast local model for indexing, cloud endpoint for search:

```yaml
warm:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:11434

search:
  provider: ollama
  model: bge-m3
  base_url: https://ollama.example.com
  token: your-cloud-api-key

socket_path: /tmp/gcq.sock
threshold_similarity: 0.7
max_context_chunks: 10
chunk_size: 512
```

### Dual Providers: Ollama and HuggingFace

Use local Ollama for indexing, HuggingFace for search:

```yaml
warm:
  provider: ollama
  model: nomic-embed-text
  base_url: http://localhost:11434

search:
  provider: huggingface
  model: BAAI/bge-large-en-v1.5
  token: hf_your-token-here

socket_path: /tmp/gcq.sock
threshold_similarity: 0.7
max_context_chunks: 10
chunk_size: 512
```

### Legacy Single Provider Config

For backward compatibility with older config format:

```yaml
provider: ollama
ollama_model: nomic-embed-text
ollama_base_url: http://localhost:11434

socket_path: /tmp/gcq.sock
threshold_similarity: 0.7
```

This legacy format is automatically migrated to the dual provider format internally.
