package cache

import (
	"context"
	"sync"
	"time"

	"github.com/goforj/cache/cachecore"
)

type memoEntry struct {
	body []byte
	ok   bool
}

// NewMemoStore decorates store with per-process read memoization.
//
// Behavior:
//   - First Get hits the backing store, clones the value, and memoizes it in-process.
//   - Subsequent Get for the same key returns the memoized clone (no backend call).
//   - Any write/delete/flush invalidates the memo entry so local reads stay in sync
//     with changes made through this process.
//   - Memo data is per-process only; other processes or external writers will not
//     invalidate it. Use only when that staleness window is acceptable.
//
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
	return &memoStore{
		store: store,
		items: make(map[string]memoEntry),
	}
}

type memoStore struct {
	store cachecore.Store
	mu    sync.RWMutex
	items map[string]memoEntry
}

func (s *memoStore) Driver() cachecore.Driver {
	return s.store.Driver()
}

func (s *memoStore) Ready(ctx context.Context) error {
	return s.store.Ready(ctx)
}

func (s *memoStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	s.mu.RLock()
	entry, ok := s.items[key]
	s.mu.RUnlock()
	if ok {
		return cloneBytes(entry.body), entry.ok, nil
	}

	body, exists, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}

	s.mu.Lock()
	s.items[key] = memoEntry{body: cloneBytes(body), ok: exists}
	s.mu.Unlock()

	return cloneBytes(body), exists, nil
}

func (s *memoStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := s.store.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	s.forget(key)
	return nil
}

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

func (s *memoStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	value, err := s.store.Increment(ctx, key, delta, ttl)
	if err != nil {
		return 0, err
	}
	s.forget(key)
	return value, nil
}

func (s *memoStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	value, err := s.store.Decrement(ctx, key, delta, ttl)
	if err != nil {
		return 0, err
	}
	s.forget(key)
	return value, nil
}

func (s *memoStore) Delete(ctx context.Context, key string) error {
	if err := s.store.Delete(ctx, key); err != nil {
		return err
	}
	s.forget(key)
	return nil
}

func (s *memoStore) DeleteMany(ctx context.Context, keys ...string) error {
	if err := s.store.DeleteMany(ctx, keys...); err != nil {
		return err
	}
	s.mu.Lock()
	for _, key := range keys {
		delete(s.items, key)
	}
	s.mu.Unlock()
	return nil
}

func (s *memoStore) Flush(ctx context.Context) error {
	if err := s.store.Flush(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	s.items = make(map[string]memoEntry)
	s.mu.Unlock()
	return nil
}

func (s *memoStore) forget(key string) {
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	clone := make([]byte, len(value))
	copy(clone, value)
	return clone
}
