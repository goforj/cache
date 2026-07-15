package cachefake

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
)

// Op identifies a cache operation for assertions.
type Op string

const (
	// OpGet identifies raw cache reads in fake assertions.
	OpGet Op = "get"
	// OpSet identifies unconditional cache writes in fake assertions.
	OpSet Op = "set"
	// OpAdd identifies conditional cache writes in fake assertions.
	OpAdd Op = "add"
	// OpInc identifies counter increments in fake assertions.
	OpInc Op = "inc"
	// OpDec identifies counter decrements in fake assertions.
	OpDec Op = "dec"
	// OpDelete identifies single-key removals in fake assertions.
	OpDelete Op = "delete"
	// OpDeleteMany identifies batch removals in fake assertions.
	OpDeleteMany Op = "delete_many"
	// OpFlush identifies namespace flushes in fake assertions.
	OpFlush Op = "flush"
)

// Fake exposes a deterministic in-memory store plus assertion helpers for tests.
// It wraps the memory store so no external services are needed.
type Fake struct {
	cache  *cache.Cache
	counts map[Op]map[string]int
	mu     sync.Mutex
}

// New creates a Fake using an in-memory store.
// @group Testing Helpers
// Example:
//
//	f := cachefake.New()
//	c := f.Cache()
//	_ = c.SetString("settings:mode", "dark", 0)
func New() *Fake {
	store := &countingStore{inner: cache.NewMemoryStore(context.Background())}
	f := &Fake{
		cache:  cache.NewCache(store),
		counts: make(map[Op]map[string]int),
	}
	store.onCount = f.record
	return f
}

// Cache returns the cache facade to inject into code under test.
// @group Testing Helpers
// Example:
//
//	f := cachefake.New()
//	c := f.Cache()
//	_, _, _ = c.GetBytes("settings:mode")
func (f *Fake) Cache() *cache.Cache { return f.cache }

// Reset clears recorded counts.
// @group Testing Helpers
// Example:
//
//	f := cachefake.New()
//	_ = f.Cache().SetString("settings:mode", "dark", 0)
//	f.Reset()
func (f *Fake) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts = make(map[Op]map[string]int)
}

// AssertCalled verifies key was touched by op the expected number of times.
// @group Testing Helpers
// Example:
//
//	f := cachefake.New()
//	c := f.Cache()
//	_ = c.SetString("settings:mode", "dark", 0)
//	t := &testing.T{}
//	f.AssertCalled(t, cachefake.OpSet, "settings:mode", 1)
func (f *Fake) AssertCalled(t *testing.T, op Op, key string, times int) {
	t.Helper()
	if got := f.Count(op, key); got != times {
		t.Fatalf("expected %s %q called %d times, got %d", op, key, times, got)
	}
}

// AssertNotCalled ensures key was never touched by op.
// @group Testing Helpers
// Example:
//
//	f := cachefake.New()
//	t := &testing.T{}
//	f.AssertNotCalled(t, cachefake.OpDelete, "settings:mode")
func (f *Fake) AssertNotCalled(t *testing.T, op Op, key string) {
	t.Helper()
	if got := f.Count(op, key); got != 0 {
		t.Fatalf("expected %s %q not called, got %d", op, key, got)
	}
}

// AssertTotal ensures the total call count for an op matches times.
// @group Testing Helpers
// Example:
//
//	f := cachefake.New()
//	c := f.Cache()
//	_ = c.Delete("a")
//	_ = c.Delete("b")
//	t := &testing.T{}
//	f.AssertTotal(t, cachefake.OpDelete, 2)
func (f *Fake) AssertTotal(t *testing.T, op Op, times int) {
	t.Helper()
	if got := f.Total(op); got != times {
		t.Fatalf("expected %s total=%d, got %d", op, times, got)
	}
}

// Count returns calls for op+key.
// @group Testing Helpers
// Example:
//
//	f := cachefake.New()
//	c := f.Cache()
//	_ = c.SetString("settings:mode", "dark", 0)
//	n := f.Count(cachefake.OpSet, "settings:mode")
//	_ = n
func (f *Fake) Count(op Op, key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts[op] == nil {
		return 0
	}
	return f.counts[op][key]
}

// Total returns total calls for an op across keys.
// @group Testing Helpers
// Example:
//
//	f := cachefake.New()
//	c := f.Cache()
//	_ = c.Delete("a")
//	_ = c.Delete("b")
//	n := f.Total(cachefake.OpDelete)
//	_ = n
func (f *Fake) Total(op Op) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	var sum int
	for _, v := range f.counts[op] {
		sum += v
	}
	return sum
}

// record serializes assertion state because fakes are commonly shared by concurrent code under test.
func (f *Fake) record(op Op, key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts[op] == nil {
		f.counts[op] = make(map[string]int)
	}
	f.counts[op][key]++
}

// countingStore wraps a Store to record calls.
type countingStore struct {
	inner   cachecore.Store
	onCount func(Op, string)
}

// Driver identifies the backend for diagnostics and capability-specific behavior.
func (s *countingStore) Driver() cachecore.Driver { return s.inner.Driver() }

// Ready verifies that the backend can serve cache operations.
func (s *countingStore) Ready(ctx context.Context) error {
	return s.inner.Ready(ctx)
}

// Get returns an owned copy of a stored value and distinguishes misses from failures.
func (s *countingStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	s.bump(OpGet, key)
	return s.inner.Get(ctx, key)
}

// Set stores an owned copy of a value using the requested or default TTL.
func (s *countingStore) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	s.bump(OpSet, key)
	return s.inner.Set(ctx, key, val, ttl)
}

// Add stores a value only when the key is currently absent.
func (s *countingStore) Add(ctx context.Context, key string, val []byte, ttl time.Duration) (bool, error) {
	s.bump(OpAdd, key)
	return s.inner.Add(ctx, key, val, ttl)
}

// Increment atomically adds delta while preserving the store's TTL contract.
func (s *countingStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	s.bump(OpInc, key)
	return s.inner.Increment(ctx, key, delta, ttl)
}

// Decrement atomically subtracts delta while preserving the store's TTL contract.
func (s *countingStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	s.bump(OpDec, key)
	return s.inner.Decrement(ctx, key, delta, ttl)
}

// Delete removes a key and treats an existing miss as success.
func (s *countingStore) Delete(ctx context.Context, key string) error {
	s.bump(OpDelete, key)
	return s.inner.Delete(ctx, key)
}

// DeleteMany removes every requested key under the store's namespace.
func (s *countingStore) DeleteMany(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		s.bump(OpDeleteMany, k)
	}
	return s.inner.DeleteMany(ctx, keys...)
}

// Flush removes entries within the store's configured scope.
func (s *countingStore) Flush(ctx context.Context) error {
	s.bump(OpFlush, "")
	return s.inner.Flush(ctx)
}

// bump keeps instrumentation optional for independently constructed counting stores.
func (s *countingStore) bump(op Op, key string) {
	if s.onCount != nil {
		s.onCount(op, key)
	}
}
