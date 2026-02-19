# Installation Guide

## For Humans

Paste this into your LLM agent session:

```
Install and configure go-context-query by following the instructions here:
https://raw.githubusercontent.com/l3aro/go-context-query/main/docs/installation/agent.md
```

---

Welcome! This guide will help you get go-context-query up and running on your machine.

## Download Standalone Binary

### Step 1: Download

Visit the [Releases](https://github.com/l3aro/go-context-query/releases) page and download the binary for your OS:

| OS      | Architecture | Download                     |
| ------- | ------------ | ------------------------------|
| Linux   | amd64        | `gcq-linux-amd64`             |
| macOS   | amd64        | `gcq-darwin-amd64`            |
| macOS   | arm64        | `gcq-darwin-arm64`            |
| Windows | amd64        | `gcq-windows-amd64`           |

Or use curl:

```bash
# Example for Linux amd64
curl -sL https://github.com/l3aro/go-context-query/releases/latest/download/gcq-linux-amd64 -o gcq
chmod +x gcq

# Example for macOS ARM64
curl -sL https://github.com/l3aro/go-context-query/releases/latest/download/gcq-darwin-arm64 -o gcq
chmod +x gcq

# Example for Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/l3aro/go-context-query/releases/latest/download/gcq-windows-amd64" -OutFile "gcq.exe"
```

### Step 2: Make Executable (Linux/macOS)

```bash
chmod +x gcq
```

### Step 2: Move to PATH

```bash
# Create local bin directory
mkdir -p ~/.local/bin

# Move the binary
mv gcq ~/.local/bin/

# Add to your PATH (add this to your ~/.bashrc or ~/.zshrc)
export PATH="$HOME/.local/bin:$PATH"
```

---

## Alternative: Build from Source

If you have Go installed and want the latest development version:

```bash
git clone https://github.com/l3aro/go-context-query.git
cd go-context-query
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
