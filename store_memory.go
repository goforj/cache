package cache

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

type memoryStore struct {
	cache      *gocache.Cache
	defaultTTL time.Duration
	mu         sync.Mutex
}

func newMemoryStore(defaultTTL, cleanupInterval time.Duration) Store {
	if defaultTTL <= 0 {
		defaultTTL = defaultCacheTTL
	}
	if cleanupInterval <= 0 {
		cleanupInterval = defaultMemoryCleanupInterval
	}
	return &memoryStore{
		cache:      gocache.New(defaultTTL, cleanupInterval),
		defaultTTL: defaultTTL,
	}
}

func (s *memoryStore) Driver() Driver {
	return DriverMemory
}

func (s *memoryStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	item, ok := s.cache.Get(key)
	if !ok {
		return nil, false, nil
	}
	body, ok := item.([]byte)
	if !ok {
		return nil, false, nil
	}
	clone := make([]byte, len(body))
	copy(clone, body)
	return clone, true, nil
}

func (s *memoryStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	clone := make([]byte, len(value))
	copy(clone, value)
	s.cache.Set(key, clone, ttl)
	return nil
}

func (s *memoryStore) Add(_ context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	clone := make([]byte, len(value))
	copy(clone, value)
	if err := s.cache.Add(key, clone, ttl); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *memoryStore) Increment(_ context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, _, err := s.readInt64(key)
	if err != nil {
		return 0, err
	}
	next := current + delta
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	s.cache.Set(key, []byte(strconv.FormatInt(next, 10)), ttl)
	return next, nil
}

func (s *memoryStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	s.cache.Delete(key)
	return nil
}

func (s *memoryStore) DeleteMany(_ context.Context, keys ...string) error {
	for _, key := range keys {
		s.cache.Delete(key)
	}
	return nil
}

func (s *memoryStore) Flush(_ context.Context) error {
	s.cache.Flush()
	return nil
}

func (s *memoryStore) readInt64(key string) (int64, bool, error) {
	body, ok := s.cache.Get(key)
	if !ok {
		return 0, false, nil
	}
	switch value := body.(type) {
	case []byte:
		n, err := strconv.ParseInt(string(value), 10, 64)
		if err != nil {
			return 0, false, fmt.Errorf("cache key %q does not contain a numeric value", key)
		}
		return n, true, nil
	case string:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, false, fmt.Errorf("cache key %q does not contain a numeric value", key)
		}
		return n, true, nil
	case int:
		return int64(value), true, nil
	case int64:
		return value, true, nil
	case int32:
		return int64(value), true, nil
	case int16:
		return int64(value), true, nil
	case int8:
		return int64(value), true, nil
	case uint:
		return int64(value), true, nil
	case uint64:
		if value > uint64(^uint64(0)>>1) {
			return 0, false, fmt.Errorf("cache key %q overflows int64", key)
		}
		return int64(value), true, nil
	case uint32:
		return int64(value), true, nil
	case uint16:
		return int64(value), true, nil
	case uint8:
		return int64(value), true, nil
	default:
		return 0, false, fmt.Errorf("cache key %q does not contain a numeric value", key)
	}
}
