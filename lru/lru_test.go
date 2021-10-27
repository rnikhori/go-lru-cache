package lru

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"
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

func TestResizeGrowAllowsMoreEntries(t *testing.T) {
	c := New[int, string](2)
	c.Put(1, "one")
	c.Put(2, "two")

	c.Resize(3)
	c.Put(3, "three")

	if got := c.Len(); got != 3 {
		t.Fatalf("Len() = %d; want 3", got)
	}
	for _, k := range []int{1, 2, 3} {
		if _, ok := c.Get(k); !ok {
			t.Fatalf("expected key %d to survive growth", k)
		}
	}
}

func TestResizeShrinkEvictsLeastRecentlyUsed(t *testing.T) {
	var evicted []int
	c := New[int, string](3, WithOnEvict(func(key int, value string, reason EvictReason) {
		evicted = append(evicted, key)
	}))
	c.Put(1, "one")
	c.Put(2, "two")
	c.Put(3, "three")
	c.Get(1) // 1 most recent, order: 1, 3, 2

	c.Resize(1)

	if got := c.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1", got)
	}
	if v, ok := c.Get(1); !ok || v != "one" {
		t.Fatalf("expected key 1 to survive, got %v, %v", v, ok)
	}
	want := []int{2, 3}
	if len(evicted) != 2 || evicted[0] != want[0] || evicted[1] != want[1] {
		t.Fatalf("evicted = %v; want %v", evicted, want)
	}
}

func TestResizeNonPositiveCapacityTreatedAsOne(t *testing.T) {
	c := New[int, string](3)
	c.Put(1, "one")
	c.Put(2, "two")

	c.Resize(0)

	if got := c.Len(); got != 1 {
		t.Fatalf("Len() after Resize(0) = %d; want 1", got)
	}
	c.Put(3, "three")
	if got := c.Len(); got != 1 {
		t.Fatalf("Len() after Put following Resize(0) = %d; want 1", got)
	}
}

// fakeClock returns a now func() time.Time whose value can be advanced
// manually, for deterministic TTL tests.
func fakeClock(start time.Time) (now func() time.Time, advance func(time.Duration)) {
	t := start
	return func() time.Time { return t }, func(d time.Duration) { t = t.Add(d) }
}

func TestPutWithTTLExpiresOnGet(t *testing.T) {
	c := New[string, int](2)
	now, advance := fakeClock(time.Now())
	c.now = now

	c.PutWithTTL("a", 1, 10*time.Millisecond)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) before expiry = %v, %v; want 1, true", v, ok)
	}

	advance(11 * time.Millisecond)

	if _, ok := c.Get("a"); ok {
		t.Fatalf("Get(a) after expiry = true; want false")
	}
	if got := c.Len(); got != 0 {
		t.Fatalf("Len() after expiry eviction = %d; want 0", got)
	}
}

func TestPutWithTTLFiresOnEvictWithExpiredReason(t *testing.T) {
	var gotReason EvictReason
	calls := 0
	c := New[string, int](2, WithOnEvict(func(key string, value int, reason EvictReason) {
		calls++
		gotReason = reason
	}))
	now, advance := fakeClock(time.Now())
	c.now = now

	c.PutWithTTL("a", 1, 5*time.Millisecond)
	advance(6 * time.Millisecond)
	c.Get("a")

	if calls != 1 {
		t.Fatalf("OnEvict called %d times; want 1", calls)
	}
	if gotReason != EvictReasonExpired {
		t.Fatalf("reason = %v; want %v", gotReason, EvictReasonExpired)
	}
}

func TestPutWithTTLNonPositiveMeansNoExpiry(t *testing.T) {
	c := New[string, int](2)
	now, advance := fakeClock(time.Now())
	c.now = now

	c.PutWithTTL("a", 1, 0)
	advance(24 * time.Hour)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %v, %v; want 1, true", v, ok)
	}
}

func TestPutHasNoExpiry(t *testing.T) {
	c := New[string, int](2)
	now, advance := fakeClock(time.Now())
	c.now = now

	c.Put("a", 1)
	advance(365 * 24 * time.Hour)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %v, %v; want 1, true", v, ok)
	}
}

func TestPeekExpiresEntry(t *testing.T) {
	c := New[string, int](2)
	now, advance := fakeClock(time.Now())
	c.now = now

	c.PutWithTTL("a", 1, 10*time.Millisecond)
	advance(11 * time.Millisecond)

	if _, ok := c.Peek("a"); ok {
		t.Fatalf("Peek(a) after expiry = true; want false")
	}
	if got := c.Len(); got != 0 {
		t.Fatalf("Len() after Peek-triggered expiry = %d; want 0", got)
	}
}

func TestPeekDoesNotExpireBeforeTTL(t *testing.T) {
	c := New[string, int](2)
	now, advance := fakeClock(time.Now())
	c.now = now

	c.PutWithTTL("a", 1, 10*time.Millisecond)
	advance(5 * time.Millisecond)

	if v, ok := c.Peek("a"); !ok || v != 1 {
		t.Fatalf("Peek(a) = %v, %v; want 1, true", v, ok)
	}
}

func TestWithDefaultTTLAppliesToPut(t *testing.T) {
	c := New[string, int](2, WithDefaultTTL[string, int](10*time.Millisecond))
	now, advance := fakeClock(time.Now())
	c.now = now

	c.Put("a", 1)
	advance(11 * time.Millisecond)

	if _, ok := c.Get("a"); ok {
		t.Fatalf("Get(a) after default TTL expiry = true; want false")
	}
}

func TestPutWithTTLOverridesDefaultTTL(t *testing.T) {
	c := New[string, int](2, WithDefaultTTL[string, int](10*time.Millisecond))
	now, advance := fakeClock(time.Now())
	c.now = now

	c.PutWithTTL("a", 1, 0) // no expiry, overriding the default TTL
	advance(1 * time.Hour)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %v, %v; want 1, true", v, ok)
	}
}

func TestLenExcludesExpiredEntries(t *testing.T) {
	c := New[string, int](3)
	now, advance := fakeClock(time.Now())
	c.now = now

	c.PutWithTTL("a", 1, 10*time.Millisecond)
	c.Put("b", 2)
	advance(11 * time.Millisecond)

	if got := c.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1", got)
	}
}

func TestKeysAndValuesExcludeExpiredEntries(t *testing.T) {
	c := New[string, int](3)
	now, advance := fakeClock(time.Now())
	c.now = now

	c.PutWithTTL("a", 1, 10*time.Millisecond)
	c.Put("b", 2)
	advance(11 * time.Millisecond)

	if got := c.Keys(); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("Keys() = %v; want [b]", got)
	}
	if got := c.Values(); !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("Values() = %v; want [2]", got)
	}
}

func TestPurgeExpiredRemovesOnlyExpiredEntries(t *testing.T) {
	var evicted []string
	c := New[string, int](5, WithOnEvict(func(key string, value int, reason EvictReason) {
		evicted = append(evicted, key)
	}))
	now, advance := fakeClock(time.Now())
	c.now = now

	c.PutWithTTL("a", 1, 10*time.Millisecond)
	c.PutWithTTL("b", 2, 10*time.Millisecond)
	c.Put("c", 3)
	advance(11 * time.Millisecond)

	n := c.PurgeExpired()

	if n != 2 {
		t.Fatalf("PurgeExpired() = %d; want 2", n)
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("Len() after purge = %d; want 1", got)
	}
	if got := c.Stats().Evictions; got != 2 {
		t.Fatalf("Evictions = %d; want 2", got)
	}
	want := map[string]bool{"a": true, "b": true}
	if len(evicted) != 2 || !want[evicted[0]] || !want[evicted[1]] {
		t.Fatalf("evicted = %v; want a and b in some order", evicted)
	}
}

func TestPurgeExpiredNoOpWhenNothingExpired(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)

	if n := c.PurgeExpired(); n != 0 {
		t.Fatalf("PurgeExpired() = %d; want 0", n)
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1", got)
	}
}

func TestJanitorPurgesExpiredEntriesOverTime(t *testing.T) {
	purged := make(chan string, 1)
	c := New[string, int](2, WithOnEvict(func(key string, value int, reason EvictReason) {
		if reason == EvictReasonExpired {
			purged <- key
		}
	}))

	c.PutWithTTL("a", 1, 5*time.Millisecond)
	c.StartJanitor(5 * time.Millisecond)
	defer c.StopJanitor()

	select {
	case key := <-purged:
		if key != "a" {
			t.Fatalf("purged key = %q; want a", key)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for janitor to purge expired entry")
	}
}

func TestStopJanitorStopsFurtherPurging(t *testing.T) {
	var count int32
	c := New[string, int](2, WithOnEvict(func(key string, value int, reason EvictReason) {
		if reason == EvictReasonExpired {
			atomic.AddInt32(&count, 1)
		}
	}))

	c.StartJanitor(2 * time.Millisecond)
	c.StopJanitor()

	c.PutWithTTL("a", 1, 1*time.Millisecond)
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&count) != 0 {
		t.Fatalf("purge count after StopJanitor = %d; want 0", count)
	}
}

func TestStartJanitorTwiceIsNoOp(t *testing.T) {
	c := New[string, int](2)

	c.StartJanitor(time.Hour)
	firstStop := c.janitorStop
	c.StartJanitor(time.Hour) // should be a no-op, not replace the running janitor

	if c.janitorStop != firstStop {
		t.Fatalf("second StartJanitor replaced the running janitor's stop channel")
	}
	c.StopJanitor()
}

func TestStopJanitorTwiceDoesNotPanic(t *testing.T) {
	c := New[string, int](2)
	c.StartJanitor(time.Hour)
	c.StopJanitor()
	c.StopJanitor() // must not panic or block
}

func TestStopJanitorWithoutStartIsNoOp(t *testing.T) {
	c := New[string, int](2)
	c.StopJanitor() // must not panic or block
}

func TestJanitorCanBeRestartedAfterStop(t *testing.T) {
	c := New[string, int](2)
	c.StartJanitor(time.Hour)
	c.StopJanitor()
	c.StartJanitor(5 * time.Millisecond)
	defer c.StopJanitor()

	time.Sleep(20 * time.Millisecond) // give the new janitor a chance to tick
}

func TestCapReflectsConstructedCapacity(t *testing.T) {
	c := New[string, int](7)
	if got := c.Cap(); got != 7 {
		t.Fatalf("Cap() = %d; want 7", got)
	}
}

func TestCapReflectsResize(t *testing.T) {
	c := New[string, int](2)
	c.Resize(9)
	if got := c.Cap(); got != 9 {
		t.Fatalf("Cap() after Resize(9) = %d; want 9", got)
	}
	c.Resize(0)
	if got := c.Cap(); got != 1 {
		t.Fatalf("Cap() after Resize(0) = %d; want 1", got)
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
