package lru

import (
	"fmt"
	"sync"
	"testing"
)

// TestConcurrentGetPutRemove hammers a single cache from many goroutines
// performing Get, Put, and Remove simultaneously. Run with -race to catch
// any data races in the locking around the shared list/map state.
func TestConcurrentGetPutRemove(t *testing.T) {
	const (
		goroutines = 32
		opsPerG    = 2000
		keyspace   = 64
	)

	c := New[int, int](16)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				key := (seed*opsPerG + i) % keyspace
				switch i % 3 {
				case 0:
					c.Put(key, key)
				case 1:
					c.Get(key)
				case 2:
					c.Remove(key)
				}
			}
		}(g)
	}
	wg.Wait()

	if got := c.Len(); got < 0 || got > c.Cap() {
		t.Fatalf("Len() = %d; want between 0 and Cap()=%d", got, c.Cap())
	}
}

// TestConcurrentGetPutWithOnEvict exercises eviction under concurrent load
// to ensure OnEvict and the Stats counters never race with the rest of the
// cache's bookkeeping.
func TestConcurrentGetPutWithOnEvict(t *testing.T) {
	const (
		goroutines = 16
		opsPerG    = 1000
	)

	var evictions int64
	var mu sync.Mutex
	c := New[string, int](8, WithOnEvict(func(key string, value int, reason EvictReason) {
		mu.Lock()
		evictions++
		mu.Unlock()
	}))

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				key := fmt.Sprintf("k%d", (seed+i)%100)
				c.Put(key, i)
				c.Get(key)
			}
		}(g)
	}
	wg.Wait()

	stats := c.Stats()
	mu.Lock()
	defer mu.Unlock()
	if stats.Evictions != uint64(evictions) {
		t.Fatalf("Stats().Evictions = %d; want %d (matching OnEvict call count)", stats.Evictions, evictions)
	}
}
