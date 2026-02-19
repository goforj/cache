package cachefake

import (
    "context"
    "sync"
    "testing"
    "time"

	"github.com/goforj/cache"
)

// Op identifies a cache operation for assertions.
type Op string

const (
	OpGet        Op = "get"
	OpSet        Op = "set"
	OpAdd        Op = "add"
	OpInc        Op = "inc"
	OpDec        Op = "dec"
	OpDelete     Op = "delete"
	OpDeleteMany Op = "delete_many"
	OpFlush      Op = "flush"
)

// Fake exposes a deterministic in-memory store plus assertion helpers for tests.
// It wraps the memory store so no external services are needed.
type Fake struct {
	cache  *cache.Cache
	counts map[Op]map[string]int
	mu     sync.Mutex
}

// New creates a Fake using an in-memory store.
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
func (f *Fake) Cache() *cache.Cache { return f.cache }

// Reset clears recorded counts.
func (f *Fake) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts = make(map[Op]map[string]int)
}

// AssertCalled verifies key was touched by op the expected number of times.
func (f *Fake) AssertCalled(t *testing.T, op Op, key string, times int) {
	t.Helper()
	if got := f.Count(op, key); got != times {
		t.Fatalf("expected %s %q called %d times, got %d", op, key, times, got)
	}
}

// AssertNotCalled ensures key was never touched by op.
func (f *Fake) AssertNotCalled(t *testing.T, op Op, key string) {
	t.Helper()
	if got := f.Count(op, key); got != 0 {
		t.Fatalf("expected %s %q not called, got %d", op, key, got)
	}
}

// AssertTotal ensures the total call count for an op matches times.
func (f *Fake) AssertTotal(t *testing.T, op Op, times int) {
	t.Helper()
	if got := f.Total(op); got != times {
		t.Fatalf("expected %s total=%d, got %d", op, times, got)
	}
}

// Count returns calls for op+key.
func (f *Fake) Count(op Op, key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts[op] == nil {
		return 0
	}
	return f.counts[op][key]
}

// Total returns total calls for an op across keys.
func (f *Fake) Total(op Op) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	var sum int
	for _, v := range f.counts[op] {
		sum += v
	}
	return sum
}

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
	inner   cache.Store
	onCount func(Op, string)
}

func (s *countingStore) Driver() cache.Driver { return s.inner.Driver() }

func (s *countingStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	s.bump(OpGet, key)
	return s.inner.Get(ctx, key)
}

func (s *countingStore) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	s.bump(OpSet, key)
	return s.inner.Set(ctx, key, val, ttl)
}

func (s *countingStore) Add(ctx context.Context, key string, val []byte, ttl time.Duration) (bool, error) {
	s.bump(OpAdd, key)
	return s.inner.Add(ctx, key, val, ttl)
}

func (s *countingStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	s.bump(OpInc, key)
	return s.inner.Increment(ctx, key, delta, ttl)
}

func (s *countingStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	s.bump(OpDec, key)
	return s.inner.Decrement(ctx, key, delta, ttl)
}

func (s *countingStore) Delete(ctx context.Context, key string) error {
	s.bump(OpDelete, key)
	return s.inner.Delete(ctx, key)
}

func (s *countingStore) DeleteMany(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		s.bump(OpDeleteMany, k)
	}
	return s.inner.DeleteMany(ctx, keys...)
}

func (s *countingStore) Flush(ctx context.Context) error {
	s.bump(OpFlush, "")
	return s.inner.Flush(ctx)
}

func (s *countingStore) bump(op Op, key string) {
	if s.onCount != nil {
		s.onCount(op, key)
	}
}
