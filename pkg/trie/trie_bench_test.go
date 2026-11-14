package trie

import (
	"fmt"
	"testing"
)

func BenchmarkTrieWithPrefix(b *testing.B) {
	tr := New()

	for i := 0; i < 10000; i++ {
		tr.Insert(fmt.Sprintf("link-%05d", i))
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		result := tr.WithPrefix("link-00", 10)
		if len(result) != 10 {
			b.Fatalf("expected 10 results, got %d", len(result))
		}
	}
}
