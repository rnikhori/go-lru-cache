// Package lru implements a thread-safe, generic LRU (least-recently-used) cache.
package lru

import (
	"container/list"
	"sync"
)

// EvictReason describes why an entry left the cache.
type EvictReason int

const (
	// EvictReasonCapacity means the entry was evicted to make room for a
	// new entry after the cache reached its capacity.
	EvictReasonCapacity EvictReason = iota
	// EvictReasonRemoved means the entry was removed by an explicit call
	// to Remove.
	EvictReasonRemoved
)

// String returns a human-readable name for the eviction reason.
func (r EvictReason) String() string {
	switch r {
	case EvictReasonCapacity:
		return "capacity"
	case EvictReasonRemoved:
		return "removed"
	default:
		return "unknown"
	}
}

// OnEvictFunc is invoked whenever an entry leaves the cache. It runs
// synchronously while the cache's internal lock is held, so it must not
// call back into the same Cache.
type OnEvictFunc[K comparable, V any] func(key K, value V, reason EvictReason)

// Option configures a Cache at construction time. Options are applied in
// the order they are passed to New.
type Option[K comparable, V any] func(*Cache[K, V])

// WithOnEvict registers a callback invoked whenever an entry is evicted
// from the cache.
func WithOnEvict[K comparable, V any](fn OnEvictFunc[K, V]) Option[K, V] {
	return func(c *Cache[K, V]) {
		c.onEvict = fn
	}
}

// Cache is a fixed-capacity LRU cache. The zero value is not usable;
// create one with New.
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List
	items    map[K]*list.Element
	onEvict  OnEvictFunc[K, V]
}

type entry[K comparable, V any] struct {
	key   K
	value V
}

// New creates an LRU cache that holds at most capacity entries. A
// non-positive capacity is treated as 1. Options can be supplied to
// customize cache behavior.
func New[K comparable, V any](capacity int, opts ...Option[K, V]) *Cache[K, V] {
	if capacity < 1 {
		capacity = 1
	}
	c := &Cache[K, V]{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[K]*list.Element, capacity),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
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

// Remove deletes key from the cache, if present, invoking the OnEvict
// callback with EvictReasonRemoved. It reports whether key was found.
func (c *Cache[K, V]) Remove(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return false
	}
	c.removeElement(el, EvictReasonRemoved)
	return true
}

func (c *Cache[K, V]) evictOldest() {
	oldest := c.ll.Back()
	if oldest == nil {
		return
	}
	c.removeElement(oldest, EvictReasonCapacity)
}

// removeElement unlinks el from the list and map and invokes the OnEvict
// callback, if any. Callers must hold c.mu.
func (c *Cache[K, V]) removeElement(el *list.Element, reason EvictReason) {
	en := el.Value.(*entry[K, V])
	c.ll.Remove(el)
	delete(c.items, en.key)
	if c.onEvict != nil {
		c.onEvict(en.key, en.value, reason)
	}
}
