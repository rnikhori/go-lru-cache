package lru

import (
	"testing"
	"time"
)

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

func BenchmarkPutParallel(b *testing.B) {
	c := New[int, int](1024)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Put(i%2048, i)
			i++
		}
	})
}

func BenchmarkGetParallel(b *testing.B) {
	c := New[int, int](1024)
	for i := 0; i < 1024; i++ {
		c.Put(i, i)
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(i % 1024)
			i++
		}
	})
}

func BenchmarkPutWithTTL(b *testing.B) {
	c := New[int, int](1024)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.PutWithTTL(i%2048, i, time.Minute)
	}
}

func BenchmarkPurgeExpired(b *testing.B) {
	c := New[int, int](1024)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		c.Clear()
		for k := 0; k < 1024; k++ {
			c.PutWithTTL(k, k, time.Nanosecond)
		}
		b.StartTimer()
		c.PurgeExpired()
	}
}
