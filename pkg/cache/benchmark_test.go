package cache

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkCacheGet(b *testing.B) {
	c := New(Options{MaxSize: 10000})
	for i := 0; i < 1000; i++ {
		c.Set(fmt.Sprintf("key%d", i), strings.Repeat("x", 100))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("key999")
	}
}

func BenchmarkCacheSet(b *testing.B) {
	c := New(Options{MaxSize: 10000})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key%d", i), strings.Repeat("x", 100))
	}
}
