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

Run the included example:

```sh
go run .
```

## Testing

```sh
go test ./...
```
