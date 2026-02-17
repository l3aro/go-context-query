# Extractor Benchmark Results

## Test Environment
- **CPU**: Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz
- **Go**: linux/amd64

## Performance Metrics

| Language | ns/op | B/op | allocs/op |
|----------|-------|------|-----------|
| JavaScript | 438,959 | 35,000 | 447 |
| Ruby | 686,833 | 41,488 | 831 |
| Go | 889,129 | 70,920 | 1,351 |
| Java | 827,026 | 76,208 | 1,268 |
| C | 1,045,906 | 78,336 | 1,476 |
| C# | 1,151,117 | 76,456 | 1,325 |
| Rust | 1,276,993 | 82,408 | 2,082 |
| TypeScript | 1,254,212 | 130,992 | 1,791 |
| Python | 1,455,872 | 134,008 | 1,716 |
| PHP | 1,354,883 | 80,792 | 2,244 |
| C++ | 1,486,418 | 94,464 | 2,235 |
| Kotlin | 1,513,555 | 94,408 | 2,326 |

## Summary

- **Fastest**: JavaScript (438,959 ns/op)
- **Slowest**: Kotlin (1,513,555 ns/op)
- **Most Memory Efficient**: JavaScript (35,000 B/op)
- **Least Memory Efficient**: Python (134,008 B/op)
- **Fewest Allocations**: JavaScript (447 allocs/op)
- **Most Allocations**: Kotlin (2,326 allocs/op)

## Notes

- Benchmarks run on sample files (~50-60 lines of code with classes, functions, interfaces)
- Results may vary based on file size and complexity
- JavaScript extractor shows best overall performance across all metrics
- Interpreted languages (Python, PHP, Ruby) show moderate performance
- Statically typed compiled languages (Rust, C++, Kotlin) show higher allocation counts
