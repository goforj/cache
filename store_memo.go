package cache

import (
	"context"
	"sync"
	"time"
)

type memoEntry struct {
	body []byte
	ok   bool
}

// NewMemoStore decorates store with per-process read memoization.
// @group Memoization
//
// Example: memoize a backing store
//
//	ctx := context.Background()
//	base := cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory})
//	memoStore := cache.NewMemoStore(base)
//	repo := cache.NewRepository(memoStore)
//	_ = repo
func NewMemoStore(store Store) Store {
	return &memoStore{
		store: store,
		items: make(map[string]memoEntry),
	}
}

type memoStore struct {
	store Store
	mu    sync.RWMutex
	items map[string]memoEntry
}

func (s *memoStore) Driver() Driver {
	return s.store.Driver()
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
