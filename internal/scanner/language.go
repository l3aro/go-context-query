package scanner

import (
	"strings"
)

// languageMap maps file extensions to programming languages.
var languageMap = map[string]string{
	// Python
	".py":  "python",
	".pyw": "python",
	".pyi": "python",
	".pyc": "python",
	".pyd": "python",
	".pyo": "python",

	// Go
	".go":  "go",
	".mod": "go",
	".sum": "go",

	// JavaScript/TypeScript
	".js":  "javascript",
	".jsx": "javascript",
	".mjs": "javascript",
	".cjs": "javascript",
	".ts":  "typescript",
	".tsx": "typescript",
	".mts": "typescript",
	".cts": "typescript",

	// Web
	".html":   "html",
	".htm":    "html",
	".css":    "css",
	".scss":   "scss",
	".sass":   "sass",
	".less":   "less",
	".vue":    "vue",
	".svelte": "svelte",

	// Java
	".java":   "java",
	".class":  "java",
	".jar":    "java",
	".gradle": "groovy",

	// Kotlin
	".kt":  "kotlin",
	".kts": "kotlin",

	// Scala
	".scala": "scala",
	".sc":    "scala",

	// C/C++
	".c":   "c",
	".h":   "c",
	".cpp": "cpp",
	".hpp": "cpp",
	".cc":  "cpp",
	".hh":  "cpp",
	".cxx": "cpp",
	".hxx": "cpp",

	// C#
	".cs":  "csharp",
	".csx": "csharp",

	// Rust
	".rs":   "rust",
	".rlib": "rust",

	// Ruby
	".rb":      "ruby",
	".erb":     "ruby",
	".gemspec": "ruby",

	// PHP
	".php":   "php",
	".phtml": "php",

	// Swift
	".swift": "swift",

	// Objective-C
	".m":  "objective-c",
	".mm": "objective-cpp",

	// Shell/Bash
	".sh":   "shell",
	".bash": "shell",
	".zsh":  "shell",
	".fish": "shell",
	".ps1":  "powershell",

	// SQL
	".sql": "sql",

	// R
	".r":   "r",
	".rmd": "r",

	// Julia
	".jl": "julia",

	// Dart
	".dart": "dart",

	// Elixir
	".ex":  "elixir",
	".exs": "elixir",

	// Erlang
	".erl": "erlang",
	".hrl": "erlang",

	// Haskell
	".hs":  "haskell",
	".lhs": "haskell",

	// Lua
	".lua": "lua",

	// Perl
	".pl": "perl",
	".pm": "perl",

	// Clojure
	".clj":  "clojure",
	".cljs": "clojure",
	".cljc": "clojure",

	// Lisp
	".lisp": "lisp",
	".lsp":  "lisp",
	".cl":   "lisp",

	// Markdown
	".md":       "markdown",
	".mdx":      "markdown",
	".markdown": "markdown",

	// JSON
	".json":  "json",
	".jsonc": "json",
	".json5": "json",

	// YAML
	".yml":  "yaml",
	".yaml": "yaml",

	// XML
	".xml": "xml",
	".svg": "xml",

	// TOML
	".toml": "toml",

	// INI/Config
	".ini":    "ini",
	".cfg":    "ini",
	".conf":   "ini",
	".config": "ini",

	// Makefile
	".mk":      "makefile",
	"Makefile": "makefile",
	"makefile": "makefile",

	// Docker
	"Dockerfile": "dockerfile",

	// Git
	".gitignore":     "gitignore",
	".gitattributes": "gitattributes",

	// Documentation
	".rst": "rst",
	".txt": "text",

	// Vim
	".vim": "vim",

	// Emacs
	".el": "elisp",

	// Assembly
	".asm": "assembly",
	".s":   "assembly",
	".S":   "assembly",

	// Protocol Buffers
	".proto": "protobuf",

	// GraphQL
	".graphql": "graphql",
	".gql":     "graphql",

	// Terraform
	".tf":     "terraform",
	".tfvars": "terraform",

	// Dockerfile variations
	"docker-compose.yml":  "dockerfile",
	"docker-compose.yaml": "dockerfile",

	// Common config files
	".env":             "dotenv",
	".env.local":       "dotenv",
	".env.production":  "dotenv",
	".env.development": "dotenv",
}

// DetectLanguage returns the programming language for a given file extension.
// Returns empty string if the extension is not recognized.
func DetectLanguage(ext string) string {
	// Normalize extension to lowercase
	ext = strings.ToLower(ext)

	if lang, ok := languageMap[ext]; ok {
		return lang
	}

	return ""
}
