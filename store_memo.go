package cache

import (
	"context"
	"sync"
	"time"

	"github.com/goforj/cache/cachecore"
)

const (
	memoMaxEntries = 256
	memoMaxAge     = time.Second
)

type memoEntry struct {
	body      []byte
	ok        bool
	expiresAt time.Time
}

// NewMemoStore decorates a backing store with a short-lived local read snapshot.
// Create one per request or unit of work. It holds up to 256 hits or misses, each
// for at most one second from the start of its backing read. Those reads can
// outlive the value's backing TTL or miss a write made through another handle.
// Use the backing store directly when you need backend TTL or immediate visibility.
// Writes through the decorator invalidate the memoized keys. The caller retains
// ownership of the backing store. NewMemoStore panics when the store is nil.
// @group Memoization
//
// Example: memoize a backing store
//
//	ctx := context.Background()
//	base := cache.NewMemoryStore(ctx)
//	memo := cache.NewMemoStore(base)
//	c := cache.NewCache(memo)
//	fmt.Println(c.Driver()) // memory
func NewMemoStore(store cachecore.Store) cachecore.Store {
	if store == nil {
		panic("cache: memo store requires a backing store")
	}
	return &memoStore{
		store: store,
		items: make(map[string]memoEntry),
	}
}

type memoStore struct {
	store cachecore.Store
	mu    sync.RWMutex
	items map[string]memoEntry

	generation uint64
}

// Driver identifies the backend for diagnostics and capability-specific behavior.
func (s *memoStore) Driver() cachecore.Driver {
	return s.store.Driver()
}

// Ready verifies that the backend can serve cache operations.
func (s *memoStore) Ready(ctx context.Context) error {
	return s.store.Ready(ctx)
}

// Get returns an owned copy of a stored value and distinguishes misses from failures.
func (s *memoStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	now := time.Now()
	s.mu.RLock()
	entry, ok := s.items[key]
	generation := s.generation
	s.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return cloneBytes(entry.body), entry.ok, nil
	}

	body, exists, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}

	s.mu.Lock()
	if expiresAt := now.Add(memoMaxAge); s.generation == generation && time.Now().Before(expiresAt) {
		if _, present := s.items[key]; !present && len(s.items) >= memoMaxEntries {
			// Drop the snapshot as a unit rather than keeping a second eviction index.
			clear(s.items)
		}
		s.items[key] = memoEntry{body: cloneBytes(body), ok: exists, expiresAt: expiresAt}
	}
	s.mu.Unlock()

	return cloneBytes(body), exists, nil
}

// Set stores an owned copy of a value using the requested or default TTL.
func (s *memoStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := s.store.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	s.forget(key)
	return nil
}

// Add stores a value only when the key is currently absent.
func (s *memoStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	created, err := s.store.Add(ctx, key, value, ttl)
	if err != nil {
		return false, err
	}
	if created {
		s.forget(key)
	}
	return created, nil
}

// LockAcquire delegates ownership-sensitive writes without memoizing lock state.
func (s *memoStore) LockAcquire(ctx context.Context, key string, owner []byte, ttl time.Duration) (bool, error) {
	store, ok := s.store.(lockStore)
	if !ok {
		return s.Add(ctx, key, owner, ttl)
	}
	created, err := store.LockAcquire(ctx, key, owner, ttl)
	if err == nil && created {
		s.forget(key)
	}
	return created, err
}

// LockRelease delegates atomic owner comparison and invalidates a released memo entry.
func (s *memoStore) LockRelease(ctx context.Context, key string, owner []byte) (bool, error) {
	store, ok := s.store.(lockStore)
	if !ok {
		if err := s.Delete(ctx, key); err != nil {
			return false, err
		}
		return true, nil
	}
	released, err := store.LockRelease(ctx, key, owner)
	if err == nil && released {
		s.forget(key)
	}
	return released, err
}

// Increment atomically adds delta while preserving the store's TTL contract.
func (s *memoStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	value, err := s.store.Increment(ctx, key, delta, ttl)
	if err != nil {
		return 0, err
	}
	s.forget(key)
	return value, nil
}

// Decrement atomically subtracts delta while preserving the store's TTL contract.
func (s *memoStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	value, err := s.store.Decrement(ctx, key, delta, ttl)
	if err != nil {
		return 0, err
	}
	s.forget(key)
	return value, nil
}

// Delete removes a key and treats an existing miss as success.
func (s *memoStore) Delete(ctx context.Context, key string) error {
	if err := s.store.Delete(ctx, key); err != nil {
		return err
	}
	s.forget(key)
	return nil
}

// DeleteMany removes every requested key under the store's namespace.
func (s *memoStore) DeleteMany(ctx context.Context, keys ...string) error {
	err := s.store.DeleteMany(ctx, keys...)
	s.mu.Lock()
	for _, key := range keys {
		delete(s.items, key)
	}
	s.generation++
	s.mu.Unlock()
	return err
}

// Flush removes entries within the store's configured scope.
func (s *memoStore) Flush(ctx context.Context) error {
	err := s.store.Flush(ctx)
	s.mu.Lock()
	clear(s.items)
	s.generation++
	s.mu.Unlock()
	return err
}

// forget advances the store generation so no in-flight read can republish stale data.
func (s *memoStore) forget(key string) {
	s.mu.Lock()
	delete(s.items, key)
	s.generation++
	s.mu.Unlock()
}

// Capabilities reports the optional inspection operations supported by the store.
func (s *memoStore) Capabilities() cachecore.InspectorCapabilities {
	inspector, ok := s.store.(cachecore.Inspector)
	if !ok {
		return cachecore.InspectorCapabilities{}
	}
	return inspector.Capabilities()
}

// ListPage returns a filtered, deterministic page of inspectable cache entries.
func (s *memoStore) ListPage(ctx context.Context, opts cachecore.ListPageOptions) (cachecore.ListPageResult, error) {
	inspector, ok := s.store.(cachecore.Inspector)
	if !ok {
		return cachecore.ListPageResult{}, ErrInspectorUnsupported
	}
	return inspector.ListPage(ctx, opts)
}

// cloneBytes prevents memoized values from sharing caller-owned backing arrays.
func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	clone := make([]byte, len(value))
	copy(clone, value)
	return clone
}
