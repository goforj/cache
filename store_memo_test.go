package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

// blockingMemoGetStore captures one read before allowing the caller to resume it.
type blockingMemoGetStore struct {
	inner   cachecore.Store
	started chan struct{}
	release chan struct{}
	block   atomic.Bool
}

// Driver delegates backend identification to the wrapped store.
func (s *blockingMemoGetStore) Driver() cachecore.Driver { return s.inner.Driver() }

// Ready delegates readiness to the wrapped store.
func (s *blockingMemoGetStore) Ready(ctx context.Context) error { return s.inner.Ready(ctx) }

// Get pauses the first completed backing read so a concurrent mutation can overtake it.
func (s *blockingMemoGetStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	body, ok, err := s.inner.Get(ctx, key)
	if s.block.CompareAndSwap(true, false) {
		close(s.started)
		<-s.release
	}
	return body, ok, err
}

// Set delegates writes to the wrapped store.
func (s *blockingMemoGetStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.inner.Set(ctx, key, value, ttl)
}

// Add delegates conditional writes to the wrapped store.
func (s *blockingMemoGetStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	return s.inner.Add(ctx, key, value, ttl)
}

// Increment delegates counter increments to the wrapped store.
func (s *blockingMemoGetStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Increment(ctx, key, delta, ttl)
}

// Decrement delegates counter decrements to the wrapped store.
func (s *blockingMemoGetStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Decrement(ctx, key, delta, ttl)
}

// Delete delegates deletion to the wrapped store.
func (s *blockingMemoGetStore) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}

// DeleteMany delegates batch deletion to the wrapped store.
func (s *blockingMemoGetStore) DeleteMany(ctx context.Context, keys ...string) error {
	return s.inner.DeleteMany(ctx, keys...)
}

// Flush delegates scoped deletion to the wrapped store.
func (s *blockingMemoGetStore) Flush(ctx context.Context) error { return s.inner.Flush(ctx) }

// TestMemoStoreCachesReadsAndInvalidatesOnMutation verifies memoized hits are reused until a local write changes the key.
func TestMemoStoreCachesReadsAndInvalidatesOnMutation(t *testing.T) {
	ctx := context.Background()
	base := newMemoryStore(0, 0)

	if err := base.Set(ctx, "k", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	store := NewMemoStore(base)

	body, ok, err := store.Get(ctx, "k")
	if err != nil || !ok || string(body) != "v1" {
		t.Fatalf("unexpected first get: ok=%v err=%v value=%q", ok, err, string(body))
	}

	if err := base.Set(ctx, "k", []byte("v2"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err = store.Get(ctx, "k")
	if err != nil || !ok || string(body) != "v1" {
		t.Fatalf("expected memoized value before invalidation")
	}

	if err := store.Set(ctx, "k", []byte("v3"), time.Minute); err != nil {
		t.Fatalf("memo set failed: %v", err)
	}
	body, ok, err = store.Get(ctx, "k")
	if err != nil || !ok || string(body) != "v3" {
		t.Fatalf("expected refreshed value after set")
	}
}

// TestMemoStoreConcurrentReadDoesNotRepublishStaleValue verifies local invalidation wins over an in-flight read.
func TestMemoStoreConcurrentReadDoesNotRepublishStaleValue(t *testing.T) {
	ctx := context.Background()
	base := newMemoryStore(0, 0)
	if err := base.Set(ctx, "k", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("seed base: %v", err)
	}
	blocking := &blockingMemoGetStore{
		inner:   base,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	blocking.block.Store(true)
	store := NewMemoStore(blocking)

	readDone := make(chan error, 1)
	go func() {
		body, ok, err := store.Get(ctx, "k")
		if err == nil && (!ok || string(body) != "v1") {
			err = errors.New("blocked read returned an unexpected value")
		}
		readDone <- err
	}()
	<-blocking.started
	if err := store.Set(ctx, "k", []byte("v2"), time.Minute); err != nil {
		t.Fatalf("concurrent set: %v", err)
	}
	close(blocking.release)
	if err := <-readDone; err != nil {
		t.Fatalf("blocked read: %v", err)
	}

	body, ok, err := store.Get(ctx, "k")
	if err != nil || !ok || string(body) != "v2" {
		t.Fatalf("stale read was republished: body=%q ok=%v err=%v", body, ok, err)
	}
}

// TestMemoStoreMutationInvalidatesUnrelatedInflightPublish documents the bounded global generation tradeoff.
func TestMemoStoreMutationInvalidatesUnrelatedInflightPublish(t *testing.T) {
	ctx := context.Background()
	base := newMemoryStore(0, 0)
	if err := base.Set(ctx, "read-key", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("seed read key: %v", err)
	}
	blocking := &blockingMemoGetStore{
		inner:   base,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	blocking.block.Store(true)
	store := NewMemoStore(blocking)

	readDone := make(chan error, 1)
	go func() {
		body, ok, err := store.Get(ctx, "read-key")
		if err == nil && (!ok || string(body) != "v1") {
			err = errors.New("blocked read returned an unexpected value")
		}
		readDone <- err
	}()
	<-blocking.started
	if err := store.Set(ctx, "write-key", []byte("mutation"), time.Minute); err != nil {
		t.Fatalf("unrelated mutation: %v", err)
	}
	if err := base.Set(ctx, "read-key", []byte("v2"), time.Minute); err != nil {
		t.Fatalf("external read-key update: %v", err)
	}
	close(blocking.release)
	if err := <-readDone; err != nil {
		t.Fatalf("blocked read: %v", err)
	}

	body, ok, err := store.Get(ctx, "read-key")
	if err != nil || !ok || string(body) != "v2" {
		t.Fatalf("unrelated mutation did not stop stale publish: body=%q ok=%v err=%v", body, ok, err)
	}
}

// TestMemoStoreMutationPathsInvalidateCache verifies every mutating operation clears affected memo entries.
func TestMemoStoreMutationPathsInvalidateCache(t *testing.T) {
	ctx := context.Background()
	store := NewMemoStore(newMemoryStore(0, 0))

	if _, err := store.Increment(ctx, "n", 1, time.Minute); err != nil {
		t.Fatalf("increment failed: %v", err)
	}
	if value, _, err := store.Get(ctx, "n"); err != nil || string(value) != "1" {
		t.Fatalf("unexpected counter value")
	}
	if _, err := store.Decrement(ctx, "n", 1, time.Minute); err != nil {
		t.Fatalf("decrement failed: %v", err)
	}
	if value, _, err := store.Get(ctx, "n"); err != nil || string(value) != "0" {
		t.Fatalf("unexpected counter value after decrement")
	}

	if ok, err := store.Add(ctx, "a", []byte("1"), time.Minute); err != nil || !ok {
		t.Fatalf("add failed: ok=%v err=%v", ok, err)
	}
	if ok, err := store.Add(ctx, "a", []byte("2"), time.Minute); err != nil || ok {
		t.Fatalf("unexpected add result: ok=%v err=%v", ok, err)
	}
	if err := store.DeleteMany(ctx, "a", "n"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}

	if err := store.Set(ctx, "f", []byte("x"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if _, _, err := store.Get(ctx, "f"); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	_, ok, err := store.Get(ctx, "f")
	if err != nil {
		t.Fatalf("get after flush failed: %v", err)
	}
	if ok {
		t.Fatalf("expected flush to clear memo + backing store")
	}
}

// TestMemoStoreDeleteInvalidates verifies deletion prevents a previously memoized hit from resurfacing.
func TestMemoStoreDeleteInvalidates(t *testing.T) {
	ctx := context.Background()
	base := newMemoryStore(0, 0)
	if err := base.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	store := NewMemoStore(base)

	if _, ok, err := store.Get(ctx, "k"); err != nil || !ok {
		t.Fatalf("get failed: %v ok=%v", err, ok)
	}
	if err := store.Delete(ctx, "k"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	_, ok, err := store.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}
	if ok {
		t.Fatalf("expected memo + backing deletion")
	}
	if store.Driver() != cachecore.DriverMemory {
		t.Fatalf("expected driver passthrough")
	}
}

// TestMemoStoreExternalWritesDoNotInvalidateMemoizedHitsOrMisses documents that out-of-band mutations remain invisible until memo expiry.
func TestMemoStoreExternalWritesDoNotInvalidateMemoizedHitsOrMisses(t *testing.T) {
	ctx := context.Background()
	base := newMemoryStore(0, 0)
	store := NewMemoStore(base)

	// Memoize a miss.
	if _, ok, err := store.Get(ctx, "miss"); err != nil || ok {
		t.Fatalf("expected initial miss: ok=%v err=%v", ok, err)
	}
	if err := base.Set(ctx, "miss", []byte("now-present"), time.Minute); err != nil {
		t.Fatalf("base set failed: %v", err)
	}
	if body, ok, err := store.Get(ctx, "miss"); err != nil || ok || body != nil {
		t.Fatalf("expected memoized miss after external write: ok=%v err=%v body=%q", ok, err, string(body))
	}

	// Local write invalidates local memo entry and refreshes future reads.
	if err := store.Set(ctx, "miss", []byte("local-write"), time.Minute); err != nil {
		t.Fatalf("memo set failed: %v", err)
	}
	if body, ok, err := store.Get(ctx, "miss"); err != nil || !ok || string(body) != "local-write" {
		t.Fatalf("expected local write to invalidate memoized miss: ok=%v err=%v body=%q", ok, err, string(body))
	}

	// Memoize a hit and prove external writes do not invalidate it.
	if err := base.Set(ctx, "hit", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("base set hit failed: %v", err)
	}
	if body, ok, err := store.Get(ctx, "hit"); err != nil || !ok || string(body) != "v1" {
		t.Fatalf("expected initial hit: ok=%v err=%v body=%q", ok, err, string(body))
	}
	if err := base.Set(ctx, "hit", []byte("v2"), time.Minute); err != nil {
		t.Fatalf("base set hit v2 failed: %v", err)
	}
	if body, ok, err := store.Get(ctx, "hit"); err != nil || !ok || string(body) != "v1" {
		t.Fatalf("expected memoized stale hit after external write: ok=%v err=%v body=%q", ok, err, string(body))
	}
}

// TestMemoStoreInvalidationIsLocalAcrossMemoStoresSharingBase verifies invalidation does not coordinate independent memo layers.
func TestMemoStoreInvalidationIsLocalAcrossMemoStoresSharingBase(t *testing.T) {
	ctx := context.Background()
	base := newMemoryStore(0, 0)
	if err := base.Set(ctx, "k", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("base set failed: %v", err)
	}

	storeA := NewMemoStore(base)
	storeB := NewMemoStore(base)

	if body, ok, err := storeA.Get(ctx, "k"); err != nil || !ok || string(body) != "v1" {
		t.Fatalf("storeA initial get failed: ok=%v err=%v body=%q", ok, err, string(body))
	}
	if body, ok, err := storeB.Get(ctx, "k"); err != nil || !ok || string(body) != "v1" {
		t.Fatalf("storeB initial get failed: ok=%v err=%v body=%q", ok, err, string(body))
	}

	// Mutating through storeB invalidates only storeB's local memo cache.
	if err := storeB.Set(ctx, "k", []byte("v2"), time.Minute); err != nil {
		t.Fatalf("storeB set failed: %v", err)
	}
	if body, ok, err := storeB.Get(ctx, "k"); err != nil || !ok || string(body) != "v2" {
		t.Fatalf("storeB should observe its local write: ok=%v err=%v body=%q", ok, err, string(body))
	}
	if body, ok, err := storeA.Get(ctx, "k"); err != nil || !ok || string(body) != "v1" {
		t.Fatalf("storeA should remain stale until its own invalidation path: ok=%v err=%v body=%q", ok, err, string(body))
	}

	// Mutating through storeA refreshes storeA but does not invalidate storeB.
	if err := storeA.Set(ctx, "k", []byte("v3"), time.Minute); err != nil {
		t.Fatalf("storeA set failed: %v", err)
	}
	if body, ok, err := storeA.Get(ctx, "k"); err != nil || !ok || string(body) != "v3" {
		t.Fatalf("storeA should observe refreshed value after local write: ok=%v err=%v body=%q", ok, err, string(body))
	}
	if body, ok, err := storeB.Get(ctx, "k"); err != nil || !ok || string(body) != "v2" {
		t.Fatalf("storeB should remain stale after storeA write: ok=%v err=%v body=%q", ok, err, string(body))
	}
}
