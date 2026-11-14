package cache

import "testing"

func BenchmarkLRUGetHit(b *testing.B) {
	c, err := NewLRU[int, int](1000)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < 1000; i++ {
		c.Put(i, i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		value, ok := c.Get(500)
		if !ok || value != 500 {
			b.Fatal("unexpected cache miss")
		}
	}
}
