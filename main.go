package main

import (
	"fmt"
	"time"

	"github.com/rnikhori/go-lru-cache/lru"
)

func main() {
	cache := lru.New[string, int](3, lru.WithOnEvict(func(key string, value int, reason lru.EvictReason) {
		fmt.Printf("evicted %q=%d (%s)\n", key, value, reason)
	}))

	keys := []string{"a", "b", "c", "d", "e"}
	for i, k := range keys {
		cache.Put(k, i)
		fmt.Printf("put %q -> %d (len=%d)\n", k, i, cache.Len())
	}

	fmt.Println()
	for _, k := range keys {
		if v, ok := cache.Get(k); ok {
			fmt.Printf("%q present, value=%d\n", k, v)
		} else {
			fmt.Printf("%q evicted\n", k)
		}
	}

	fmt.Println()
	fmt.Println("keys in most-to-least-recently-used order:", cache.Keys())

	fmt.Println()
	cache.PutWithTTL("short-lived", 100, 10*time.Millisecond)
	fmt.Println("short-lived present immediately:", presentIn(cache, "short-lived"))
	time.Sleep(20 * time.Millisecond)
	fmt.Println("short-lived present after TTL:", presentIn(cache, "short-lived"))

	fmt.Println()
	s := cache.Stats()
	fmt.Printf("stats: hits=%d misses=%d evictions=%d\n", s.Hits, s.Misses, s.Evictions)
}

func presentIn(cache *lru.Cache[string, int], key string) bool {
	_, ok := cache.Get(key)
	return ok
}
