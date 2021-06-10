// Package lru implements a thread-safe, generic LRU (least-recently-used) cache.
package lru

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
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
	// EvictReasonCleared means the entry was removed by a call to Clear.
	EvictReasonCleared
	// EvictReasonExpired means the entry's TTL had passed when it was
	// discovered, either lazily on access or by a purge.
	EvictReasonExpired
)

// String returns a human-readable name for the eviction reason.
func (r EvictReason) String() string {
	switch r {
	case EvictReasonCapacity:
		return "capacity"
	case EvictReasonRemoved:
		return "removed"
	case EvictReasonCleared:
		return "cleared"
	case EvictReasonExpired:
		return "expired"
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

// Stats holds cumulative counters describing cache activity. All counters
// only ever increase for the lifetime of a Cache. Evictions counts every
// entry removal that fires OnEvict, regardless of EvictReason.
type Stats struct {
	Hits      uint64
	Misses    uint64
	Evictions uint64
}

// Cache is a fixed-capacity LRU cache. The zero value is not usable;
// create one with New.
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List
	items    map[K]*list.Element
	onEvict  OnEvictFunc[K, V]

	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64

	// now is the time source used for TTL calculations. It defaults to
	// time.Now and is overridden in tests to make expiry deterministic.
	now func() time.Time
}

type entry[K comparable, V any] struct {
	key      K
	value    V
	expireAt time.Time // zero value means no expiry
}

func (e *entry[K, V]) expired(now time.Time) bool {
	return !e.expireAt.IsZero() && !now.Before(e.expireAt)
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
		now:      time.Now,
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
		c.misses.Add(1)
		var zero V
		return zero, false
	}
	c.ll.MoveToFront(el)
	c.hits.Add(1)
	return el.Value.(*entry[K, V]).value, true
}

// Peek returns the value stored for key without marking it as recently
// used, leaving the eviction order unchanged.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		c.misses.Add(1)
		var zero V
		return zero, false
	}
	c.hits.Add(1)
	return el.Value.(*entry[K, V]).value, true
}

// Put inserts or updates the value for key, marking it as recently used.
// If the cache is at capacity and key is new, the least-recently-used
// entry is evicted. The entry has no expiry.
func (c *Cache[K, V]) Put(key K, value V) {
	c.put(key, value, 0)
}

// PutWithTTL is like Put but the entry expires after ttl. A ttl of zero
// or less means the entry never expires. Once expired, the entry is
// treated as absent and is lazily evicted on the next Get, Peek, or
// PurgeExpired call.
func (c *Cache[K, V]) PutWithTTL(key K, value V, ttl time.Duration) {
	c.put(key, value, ttl)
}

func (c *Cache[K, V]) put(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expireAt time.Time
	if ttl > 0 {
		expireAt = c.now().Add(ttl)
	}

	if el, ok := c.items[key]; ok {
		en := el.Value.(*entry[K, V])
		en.value = value
		en.expireAt = expireAt
		c.ll.MoveToFront(el)
		return
	}

	el := c.ll.PushFront(&entry[K, V]{key: key, value: value, expireAt: expireAt})
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

// Keys returns the cache's keys ordered from most- to least-recently used.
func (c *Cache[K, V]) Keys() []K {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys := make([]K, 0, c.ll.Len())
	for el := c.ll.Front(); el != nil; el = el.Next() {
		keys = append(keys, el.Value.(*entry[K, V]).key)
	}
	return keys
}

// Values returns the cache's values ordered from most- to least-recently
// used.
func (c *Cache[K, V]) Values() []V {
	c.mu.Lock()
	defer c.mu.Unlock()

	values := make([]V, 0, c.ll.Len())
	for el := c.ll.Front(); el != nil; el = el.Next() {
		values = append(values, el.Value.(*entry[K, V]).value)
	}
	return values
}

// Resize changes the cache's capacity. If the new capacity is smaller than
// the current number of entries, the least-recently-used entries are
// evicted with EvictReasonCapacity until the cache fits.
func (c *Cache[K, V]) Resize(capacity int) {
	if capacity < 1 {
		capacity = 1
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.capacity = capacity
	for c.ll.Len() > c.capacity {
		c.evictOldest()
	}
}

// Stats returns a snapshot of the cache's cumulative hit/miss counters.
// It is safe to call concurrently with any other method.
func (c *Cache[K, V]) Stats() Stats {
	return Stats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
	}
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

// Clear removes all entries from the cache, invoking the OnEvict callback
// for each one with EvictReasonCleared.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for el := c.ll.Front(); el != nil; {
		next := el.Next()
		en := el.Value.(*entry[K, V])
		c.evictions.Add(1)
		if c.onEvict != nil {
			c.onEvict(en.key, en.value, EvictReasonCleared)
		}
		el = next
	}
	c.ll.Init()
	c.items = make(map[K]*list.Element, c.capacity)
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
	c.evictions.Add(1)
	if c.onEvict != nil {
		c.onEvict(en.key, en.value, reason)
	}
}
