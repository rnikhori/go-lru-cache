package lru

import (
	"reflect"
	"testing"
)

func TestGetPutBasic(t *testing.T) {
	c := New[string, int](2)

	c.Put("a", 1)
	c.Put("b", 2)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %v, %v; want 1, true", v, ok)
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Fatalf("Get(b) = %v, %v; want 2, true", v, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatalf("Get(missing) = _, true; want false")
	}
	if got := c.Len(); got != 2 {
		t.Fatalf("Len() = %d; want 2", got)
	}
}

func TestEvictsLeastRecentlyUsed(t *testing.T) {
	c := New[int, string](3)

	c.Put(1, "one")
	c.Put(2, "two")
	c.Put(3, "three")

	// Touch 1 so that 2 becomes the least-recently-used entry.
	c.Get(1)

	c.Put(4, "four")

	if _, ok := c.Get(2); ok {
		t.Fatalf("expected key 2 to be evicted")
	}
	if v, ok := c.Get(1); !ok || v != "one" {
		t.Fatalf("expected key 1 to survive, got %v, %v", v, ok)
	}
	if v, ok := c.Get(3); !ok || v != "three" {
		t.Fatalf("expected key 3 to survive, got %v, %v", v, ok)
	}
	if v, ok := c.Get(4); !ok || v != "four" {
		t.Fatalf("expected key 4 to be present, got %v, %v", v, ok)
	}
	if got := c.Len(); got != 3 {
		t.Fatalf("Len() = %d; want 3", got)
	}
}

func TestPutUpdatesExistingKey(t *testing.T) {
	c := New[string, int](2)

	c.Put("a", 1)
	c.Put("a", 100)

	if v, ok := c.Get("a"); !ok || v != 100 {
		t.Fatalf("Get(a) = %v, %v; want 100, true", v, ok)
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1", got)
	}
}

func TestOnEvictFiresOnCapacityEviction(t *testing.T) {
	type evicted struct {
		key    int
		value  string
		reason EvictReason
	}
	var got []evicted

	c := New[int, string](2, WithOnEvict(func(key int, value string, reason EvictReason) {
		got = append(got, evicted{key, value, reason})
	}))

	c.Put(1, "one")
	c.Put(2, "two")
	c.Put(3, "three") // evicts 1

	if len(got) != 1 {
		t.Fatalf("len(got) = %d; want 1", len(got))
	}
	want := evicted{key: 1, value: "one", reason: EvictReasonCapacity}
	if got[0] != want {
		t.Fatalf("evicted = %+v; want %+v", got[0], want)
	}
}

func TestOnEvictNotCalledWithoutEviction(t *testing.T) {
	called := false
	c := New[int, string](2, WithOnEvict(func(int, string, EvictReason) {
		called = true
	}))

	c.Put(1, "one")
	c.Get(1)

	if called {
		t.Fatalf("OnEvict callback invoked without any eviction")
	}
}

func TestRemoveDeletesExistingKey(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	if ok := c.Remove("a"); !ok {
		t.Fatalf("Remove(a) = false; want true")
	}
	if _, ok := c.Get("a"); ok {
		t.Fatalf("expected key a to be gone after Remove")
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1", got)
	}
}

func TestRemoveMissingKeyReturnsFalse(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)

	if ok := c.Remove("missing"); ok {
		t.Fatalf("Remove(missing) = true; want false")
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1", got)
	}
}

func TestRemoveFiresOnEvictWithRemovedReason(t *testing.T) {
	var gotReason EvictReason
	calls := 0
	c := New[string, int](2, WithOnEvict(func(key string, value int, reason EvictReason) {
		calls++
		gotReason = reason
	}))
	c.Put("a", 1)

	c.Remove("a")

	if calls != 1 {
		t.Fatalf("OnEvict called %d times; want 1", calls)
	}
	if gotReason != EvictReasonRemoved {
		t.Fatalf("reason = %v; want %v", gotReason, EvictReasonRemoved)
	}
}

func TestClearEmptiesCache(t *testing.T) {
	c := New[string, int](3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	c.Clear()

	if got := c.Len(); got != 0 {
		t.Fatalf("Len() after Clear = %d; want 0", got)
	}
	for _, k := range []string{"a", "b", "c"} {
		if _, ok := c.Get(k); ok {
			t.Fatalf("Get(%q) after Clear = true; want false", k)
		}
	}
}

func TestClearFiresOnEvictForEachEntry(t *testing.T) {
	var reasons []EvictReason
	c := New[string, int](3, WithOnEvict(func(key string, value int, reason EvictReason) {
		reasons = append(reasons, reason)
	}))
	c.Put("a", 1)
	c.Put("b", 2)

	c.Clear()

	if len(reasons) != 2 {
		t.Fatalf("len(reasons) = %d; want 2", len(reasons))
	}
	for _, r := range reasons {
		if r != EvictReasonCleared {
			t.Fatalf("reason = %v; want %v", r, EvictReasonCleared)
		}
	}
}

func TestClearThenPutWorksNormally(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Clear()

	c.Put("b", 2)
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Fatalf("Get(b) after Clear+Put = %v, %v; want 2, true", v, ok)
	}
}

func TestPeekReturnsValueWithoutPromoting(t *testing.T) {
	c := New[int, string](2)
	c.Put(1, "one")
	c.Put(2, "two")

	if v, ok := c.Peek(1); !ok || v != "one" {
		t.Fatalf("Peek(1) = %v, %v; want one, true", v, ok)
	}

	// 1 should still be least-recently-used since Peek must not promote it.
	c.Put(3, "three")

	if _, ok := c.Get(1); ok {
		t.Fatalf("expected key 1 to be evicted despite Peek")
	}
	if v, ok := c.Get(2); !ok || v != "two" {
		t.Fatalf("expected key 2 to survive, got %v, %v", v, ok)
	}
}

func TestPeekMissingKey(t *testing.T) {
	c := New[int, string](2)
	if _, ok := c.Peek(99); ok {
		t.Fatalf("Peek(99) = true; want false")
	}
}

func TestKeysOrderedMostToLeastRecentlyUsed(t *testing.T) {
	c := New[int, string](3)
	c.Put(1, "one")
	c.Put(2, "two")
	c.Put(3, "three")
	c.Get(1) // promote 1 to most-recently-used

	want := []int{1, 3, 2}
	if got := c.Keys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys() = %v; want %v", got, want)
	}
}

func TestValuesOrderedMostToLeastRecentlyUsed(t *testing.T) {
	c := New[int, string](3)
	c.Put(1, "one")
	c.Put(2, "two")
	c.Put(3, "three")
	c.Get(1)

	want := []string{"one", "three", "two"}
	if got := c.Values(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Values() = %v; want %v", got, want)
	}
}

func TestKeysAndValuesOnEmptyCache(t *testing.T) {
	c := New[int, string](2)

	if got := c.Keys(); len(got) != 0 {
		t.Fatalf("Keys() on empty cache = %v; want empty", got)
	}
	if got := c.Values(); len(got) != 0 {
		t.Fatalf("Values() on empty cache = %v; want empty", got)
	}
}

func TestStatsTracksHitsAndMisses(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)

	c.Get("a")        // hit
	c.Get("missing")  // miss
	c.Peek("a")       // hit
	c.Peek("missing") // miss
	c.Get("a")        // hit

	got := c.Stats()
	if got.Hits != 3 {
		t.Fatalf("Hits = %d; want 3", got.Hits)
	}
	if got.Misses != 2 {
		t.Fatalf("Misses = %d; want 2", got.Misses)
	}
}

func TestStatsStartsAtZero(t *testing.T) {
	c := New[string, int](2)
	got := c.Stats()
	if got.Hits != 0 || got.Misses != 0 {
		t.Fatalf("Stats() = %+v; want zero value", got)
	}
}

func TestStatsTracksEvictionsAcrossReasons(t *testing.T) {
	c := New[int, string](2)
	c.Put(1, "one")
	c.Put(2, "two")
	c.Put(3, "three") // evicts 1 by capacity

	if got := c.Stats().Evictions; got != 1 {
		t.Fatalf("Evictions after capacity eviction = %d; want 1", got)
	}

	c.Remove(2)
	if got := c.Stats().Evictions; got != 2 {
		t.Fatalf("Evictions after Remove = %d; want 2", got)
	}

	c.Clear()
	if got := c.Stats().Evictions; got != 3 {
		t.Fatalf("Evictions after Clear = %d; want 3", got)
	}
}

func TestCapacityOneEvictsImmediately(t *testing.T) {
	c := New[int, int](1)

	c.Put(1, 10)
	c.Put(2, 20)

	if _, ok := c.Get(1); ok {
		t.Fatalf("expected key 1 to be evicted")
	}
	if v, ok := c.Get(2); !ok || v != 20 {
		t.Fatalf("Get(2) = %v, %v; want 20, true", v, ok)
	}
}
