# Extractor Performance Benchmarks

**Date:** February 17, 2026  
**Environment:** Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz  
**Go Version:** 1.21+

## Summary

This document contains performance benchmarks for each language extractor in the go-context-query project. Benchmarks measure the extraction speed (nanoseconds per operation) for parsing sample code files using tree-sitter.

## Benchmark Results

| Language    | Time per Extraction (ns/op) | Relative Speed |
|-------------|-----------------------------|----------------|
| JavaScript  | ~430,000                   | Fastest (1.0x) |
| Ruby        | ~690,000                   | 1.6x slower    |
| Java        | ~800,000                   | 1.9x slower    |
| Go          | ~870,000                   | 2.0x slower    |
| C           | ~1,030,000                 | 2.4x slower    |
| C#          | ~1,130,000                 | 2.6x slower    |
| TypeScript  | ~1,240,000                 | 2.9x slower    |
| Python      | ~1,290,000                 | 3.0x slower    |
| Rust        | ~1,290,000                 | 3.0x slower    |
| PHP         | ~1,320,000                 | 3.1x slower    |
| C++         | ~1,460,000                 | 3.4x slower    |
| Kotlin      | ~1,500,000                 | 3.5x slower    |

### Detailed Results

```
goos: linux
goarch: amd64
pkg: github.com/l3aro/go-context-query/pkg/extractor
cpu: Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz
BenchmarkGoExtractor/sample_file-32         	    4051	    871461 ns/op
BenchmarkTypeScriptExtractor/sample_file-32 	    2724	   1240864 ns/op
BenchmarkPythonExtractor/sample_file-32     	    2737	   1295550 ns/op
BenchmarkJavaExtractor/sample_file-32       	    4408	    794910 ns/op
BenchmarkRustExtractor/sample_file-32       	    2804	   1290493 ns/op
BenchmarkJavaScriptExtractor/sample_file-32 	    7711	    429628 ns/op
BenchmarkCExtractor/sample_file-32          	    3522	   1029092 ns/op
BenchmarkCPPExtractor/sample_file-32        	    2455	   1461069 ns/op
BenchmarkRubyExtractor/sample_file-32       	    5140	    689917 ns/op
BenchmarkPHPExtractor/sample_file-32        	    2625	   1317389 ns/op
BenchmarkKotlinExtractor/sample_file-32     	    2376	   1504881 ns/op
BenchmarkCSharpExtractor/sample_file-32     	    3162	   1126142 ns/op
```

## Notes

- **JavaScript** is the fastest extractor, likely due to simpler AST structure
- **Kotlin** and **C++** are the slowest, likely due to more complex language features
- Swift extractor is not implemented yet (benchmark skipped)
- All benchmarks use similar sample code with:
  - A class with constructor
  - Methods with parameters and return types
  - Data structure processing (loops, maps/dictionaries)
  - An interface/trait definition
- Benchmarks measure full extraction time including parsing and AST traversal

## Running Benchmarks

To run the benchmarks:

```bash
# Run all benchmarks
go test -bench=. -benchtime=3s ./pkg/extractor/...

# Run specific language benchmark
go test -bench=BenchmarkGoExtractor -benchtime=3s ./pkg/extractor/...

# Run with memory allocation stats
go test -bench=BenchmarkGoExtractor -benchmem ./pkg/extractor/...
```
