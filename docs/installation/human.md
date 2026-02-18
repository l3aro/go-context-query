# Installation Guide

Welcome! This guide will help you get go-context-query up and running on your machine.

## Prerequisites

Before you begin, make sure you have:

- **Go 1.21+** installed on your system

## Quick Start

### Step 1: Clone and Build

```bash
# Clone the repository
git clone https://github.com/l3aro/go-context-query.git
cd go-context-query

# Build the binary
make build
```

This creates the `gcq` binary in the `bin/` directory.

### Step 2: Initialize Configuration

Run the interactive setup:

```bash
./bin/gcq init
```

You'll be prompted to choose:
- **Warm Provider**: Ollama or HuggingFace (for indexing)
- **Search Provider**: Same as warm or a different one
- **Save Location**: Global (`~/.gcq/config.yaml`) or Project (`.gcq/config.yaml`)

### Step 3: Verify Installation

Run the doctor command to check everything works:

```bash
./bin/gcq doctor
```

You should see green checkmarks if your setup is correct.

## Alternative: Use Environment Variables

If you prefer not to use the config file, you can set environment variables:

```bash
export GCQ_PROVIDER=ollama
export GCQ_OLLAMA_MODEL=nomic-embed-text
export GCQ_OLLAMA_BASE_URL=http://localhost:11434
```

See `internal/config/config.go` for all available options.

## Installing to PATH

To use `gcq` from anywhere:

```bash
# Copy to a directory in your PATH
cp bin/gcq ~/local/bin/

# Or add the bin directory to your PATH
export PATH="$PATH:$(pwd)/bin"
```

## Next Steps

Check out the [README](../README.md) for usage examples:

- `gcq warm ./myproject` - Build semantic index
- `gcq semantic "find auth code"` - Search your codebase
- `gcq context main.py` - Get LLM-ready context

## Troubleshooting

### "Command not found"

Make sure the `gcq` binary is in your PATH, or use the full path `./bin/gcq`.

### "Ollama not running"

Start Ollama with: `ollama serve`

### "No configuration found"

Run: `gcq init` to create a configuration file.

## Need Help?

- Check the [main README](../README.md) for detailed usage
- Open an issue on GitHub if you run into problems
