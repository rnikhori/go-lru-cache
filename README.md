# go-lru-cache

A generic, thread-safe LRU cache for Go, built on `container/list` and the standard library only.

## Usage

```go
package main

import (
	"fmt"

	"github.com/rnikhori/go-lru-cache/lru"
)

func main() {
	cache := lru.New[string, int](3)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)
	cache.Put("d", 4) // evicts "a", the least-recently-used entry

	if v, ok := cache.Get("a"); !ok {
		fmt.Println("a was evicted")
	} else {
		fmt.Println("a =", v)
	}
}
```

## Eviction callbacks

Pass `WithOnEvict` to `New` to be notified whenever an entry leaves the
cache. The callback runs synchronously while the cache's internal lock is
held, so it must not call back into the same cache:

```go
cache := lru.New[string, int](2, lru.WithOnEvict(func(key string, value int, reason lru.EvictReason) {
	fmt.Printf("evicted %q=%d (%s)\n", key, value, reason)
}))
```

Run the included example:

```sh
go run .
```

## Testing

```sh
go test ./...
```
