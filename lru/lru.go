// Package lru implements a thread-safe, generic LRU (least-recently-used) cache.
package lru

import (
	"container/list"
	"sync"
)

// Cache is a fixed-capacity LRU cache. The zero value is not usable;
// create one with New.
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List
	items    map[K]*list.Element
}

type entry[K comparable, V any] struct {
	key   K
	value V
}

// New creates an LRU cache that holds at most capacity entries. A
// non-positive capacity is treated as 1.
func New[K comparable, V any](capacity int) *Cache[K, V] {
	if capacity < 1 {
		capacity = 1
	}
	return &Cache[K, V]{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[K]*list.Element, capacity),
	}
}

// Get returns the value stored for key and marks it as recently used.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*entry[K, V]).value, true
}

// Put inserts or updates the value for key, marking it as recently used.
// If the cache is at capacity and key is new, the least-recently-used
// entry is evicted.
func (c *Cache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		el.Value.(*entry[K, V]).value = value
		c.ll.MoveToFront(el)
		return
	}

	el := c.ll.PushFront(&entry[K, V]{key: key, value: value})
	c.items[key] = el

	if c.ll.Len() > c.capacity {
		c.evictOldest()
	}
}

// Len returns the number of entries currently stored in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}

func (c *Cache[K, V]) evictOldest() {
	oldest := c.ll.Back()
	if oldest == nil {
		return
	}
	c.ll.Remove(oldest)
	delete(c.items, oldest.Value.(*entry[K, V]).key)
}
