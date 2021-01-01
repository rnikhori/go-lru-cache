package main

import (
	"fmt"

	"github.com/rnikhori/go-lru-cache/lru"
)

func main() {
	cache := lru.New[string, int](3)

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
}
