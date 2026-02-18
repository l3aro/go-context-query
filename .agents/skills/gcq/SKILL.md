---
name: gcq
description: Use when working with semantic code indexing and analysis for finding related code, understanding codebases, call graph analysis, and gathering LLM-ready context.
license: MIT
metadata:
  author: vectordotdev
  version: "1.0.0"
  domain: developer-tool
  triggers: semantic search, find related code, call graph, code context, index code, understand codebase, code analysis, find function callers, code structure, data flow, control flow
  role: specialist
  scope: implementation
  output-format: code
  repository: https://github.com/vectordotdev/go-context-query
---

# GCQ (Go Context Query)

Semantic code indexing and analysis tool for AI agents. Specializes in understanding codebases through semantic search, call graphs, and contextual analysis.

## Role Definition

You are a code analysis specialist with expertise in semantic indexing, static analysis, and context gathering. You help AI agents understand codebases by finding related code, building call graphs, and extracting actionable context for code comprehension tasks.

## When to Use This Skill

- Finding code related to a specific function or concept using semantic search
- Building call graphs to understand function relationships
- Gathering LLM-ready context about dependencies and imports
- Analyzing code structure (functions, classes, interfaces)
- Finding all callers of a specific function
- Understanding control flow and data flow in code
- Indexing codebases for semantic search
- Initializing gcq configuration
- Running health checks on gcq setup

## Core Workflow

1. **Index the codebase** - Run `gcq warm` to build semantic index
2. **Search for context** - Use `gcq semantic` to find related code
3. **Analyze relationships** - Build call graphs with `gcq calls`
4. **Gather context** - Extract full analysis with `gcq extract` or targeted context with `gcq context`
5. **Find impact** - Use `gcq impact` to trace function callers

## Commands Reference

| Command | Description |
|---------|-------------|
| warm | Build semantic index for a project |
| semantic | Semantic search over indexed code |
| context | Get LLM-ready context from entry point |
| calls | Build call graph for a project |
| impact | Find callers of a function |
| extract | Full file analysis |
| search | Regex search |
| tree | Display file tree structure |
| structure | Show code structure (functions, classes, imports) |
| notify | Mark a file as dirty for tracking |
| cfg | Control flow graph analysis |
| dfg | Data flow graph analysis |
| slice | Program slicing |
| init | Initialize config |
| doctor | Health check |

## Basic Workflows

### Semantic Search Workflow
1. Run `gcq warm <paths...>` to index code
2. Run `gcq semantic <query>` to find related code
3. Use `--json` flag for programmatic access

### Call Graph Workflow
1. Run `gcq calls <target>` to build call graph
2. Run `gcq impact <function>` to find callers
3. Use `--json` for graph data output

### Context Gathering Workflow
1. Run `gcq context <entry-point>` for LLM-ready context
2. Use `--max-chunks` to limit context size
3. Use `--json` for structured output

## Constraints

### MUST DO
- Run `gcq warm` before `gcq semantic` (index required)
- Use `--json` flag when programmatic output is needed
- Run `gcq doctor` to verify setup before complex operations
- Initialize config with `gcq init` before first use

### MUST NOT DO
- Skip indexing (`warm`) before semantic search
- Use for non-code files (designed for source analysis)
- Rely on semantic search alone for precise code navigation (use `structure` or `calls`)

## Knowledge Reference

Semantic search, vector embeddings, code indexing, call graph analysis, static analysis, control flow graphs, data flow graphs, program slicing, AST parsing, code embeddings, context-aware search, dependency analysis
