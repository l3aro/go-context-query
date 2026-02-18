package index

import (
	"fmt"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

func BenchmarkIndexSearch(b *testing.B) {
	idx := NewVectorIndex(384)
	for i := 0; i < 1000; i++ {
		vec := make([]float32, 384)
		for j := range vec {
			vec[j] = float32(i%100) / 100.0
		}
		idx.Add(fmt.Sprintf("id%d", i), vec, types.EmbeddingUnit{L1Data: types.ModuleInfo{Path: fmt.Sprintf("file%d.go", i)}})
	}
	query := make([]float32, 384)
	for i := range query {
		query[i] = float32(i%100) / 100.0
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search(query, 10)
	}
}
