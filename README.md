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

## Removing entries

`Remove` deletes a single key, and `Clear` empties the whole cache. Both
fire `OnEvict` for the entries they remove:

```go
cache.Remove("a")  // EvictReasonRemoved
cache.Clear()      // EvictReasonCleared, once per remaining entry
```

## Inspecting the cache

`Peek` reads a value without affecting recency order. `Keys` and `Values`
return the current contents ordered from most- to least-recently used:

```go
v, ok := cache.Peek("a")     // doesn't promote "a"
keys := cache.Keys()         // most-recently used first
values := cache.Values()
```

## Stats

`Stats` reports cumulative hit, miss, and eviction counts, safe to read
concurrently from any goroutine:

```go
s := cache.Stats()
fmt.Printf("hits=%d misses=%d evictions=%d\n", s.Hits, s.Misses, s.Evictions)
```

## Resizing

`Resize` changes capacity at runtime. Shrinking evicts least-recently-used
entries as needed; a non-positive capacity is treated as 1, matching `New`:

```go
cache.Resize(10) // grow
cache.Resize(2)  // shrink, evicting down to 2 entries
```

## TTL / expiration

Entries can carry an optional expiry. `PutWithTTL` sets it per entry;
`WithDefaultTTL` sets one for every `Put` call. Expired entries are
treated as absent and are lazily evicted the next time they're looked up
via `Get`, `Peek`, `Len`, `Keys`, or `Values`:

```go
cache := lru.New[string, int](100, lru.WithDefaultTTL[string, int](time.Minute))

cache.Put("a", 1)                                  // expires in 1 minute
cache.PutWithTTL("b", 2, 5*time.Second)             // expires sooner
cache.PutWithTTL("c", 3, 0)                         // never expires
```

Call `PurgeExpired` to proactively sweep and evict everything past its
TTL — useful for entries that are set once and never read again:

```go
n := cache.PurgeExpired() // number of entries removed
```

## Background janitor

`StartJanitor` runs `PurgeExpired` on a ticker in its own goroutine, so
expired entries are reclaimed even if nothing ever calls `Get` on them
again. `StopJanitor` stops it and waits for the goroutine to exit:

```go
cache.StartJanitor(time.Minute)
defer cache.StopJanitor()
```

Starting a janitor while one is already running is a no-op; stopping one
that isn't running is also a no-op.

Run the included example:

```sh
go run .
```

## Testing

```sh
go test ./...
```
