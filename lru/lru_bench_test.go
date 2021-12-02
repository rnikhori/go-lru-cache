package lru

import "testing"

func BenchmarkPut(b *testing.B) {
	c := New[int, int](1024)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Put(i%2048, i)
	}
}

func BenchmarkGetHit(b *testing.B) {
	c := New[int, int](1024)
	for i := 0; i < 1024; i++ {
		c.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(i % 1024)
	}
}

func BenchmarkGetMiss(b *testing.B) {
	c := New[int, int](1024)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Get(i)
	}
}
