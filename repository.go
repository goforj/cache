package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Cache provides an ergonomic cache API on top of Store.
type Cache struct {
	store      Store
	defaultTTL time.Duration
}

// NewCache creates a cache facade bound to a concrete store.
// @group Cache
//
// Example: cache from store
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	c := cache.NewCache(store)
//	_ = c
func NewCache(store Store) *Cache {
	return NewCacheWithTTL(store, defaultCacheTTL)
}

// NewCacheWithTTL lets callers override the default TTL applied when ttl <= 0.
// @group Cache
//
// Example: cache with custom default TTL
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	c := cache.NewCacheWithTTL(store, 2*time.Minute)
//	_ = ctx
//	_ = c
func NewCacheWithTTL(store Store, defaultTTL time.Duration) *Cache {
	if defaultTTL <= 0 {
		defaultTTL = defaultCacheTTL
	}
	return &Cache{
		store:      store,
		defaultTTL: defaultTTL,
	}
}

// Store returns the underlying store implementation.
// @group Cache
//
// Example: access store
//
//	store := cache.NewCache(cache.NewMemoryStore(ctx)).Store()
func (c *Cache) Store() Store {
	return c.store
}

// Driver reports the underlying store driver.
// @group Cache
func (c *Cache) Driver() Driver {
	return c.store.Driver()
}

// Get returns raw bytes for key when present.
// @group Cache
//
// Example: get bytes
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	cache := cache.NewCache(store)
//	_ = cache.Set(ctx, "user:42", []byte("Ada"), 0)
//	value, ok, _ := cache.Get(ctx, "user:42")
//	_ = value
//	_ = ok
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return c.store.Get(ctx, key)
}

// GetString returns a UTF-8 string value for key when present.
// @group Cache
func (c *Cache) GetString(ctx context.Context, key string) (string, bool, error) {
	body, ok, err := c.Get(ctx, key)
	if err != nil || !ok {
		return "", ok, err
	}
	return string(body), true, nil
}

// GetJSON decodes a JSON value into T when key exists.
// @group Cache JSON
func GetJSON[T any](ctx context.Context, cache *Cache, key string) (T, bool, error) {
	var zero T
	body, ok, err := cache.Get(ctx, key)
	if err != nil || !ok {
		return zero, ok, err
	}
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return zero, false, err
	}
	return out, true, nil
}

// Set writes raw bytes to key.
// @group Cache
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.store.Set(ctx, key, value, c.resolveTTL(ttl))
}

// SetString writes a string value to key.
// @group Cache
func (c *Cache) SetString(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.Set(ctx, key, []byte(value), ttl)
}

// SetJSON encodes value as JSON and writes it to key.
// @group Cache JSON
func SetJSON[T any](ctx context.Context, cache *Cache, key string, value T, ttl time.Duration) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return cache.Set(ctx, key, body, ttl)
}

// Add writes value only when key is not already present.
// @group Cache
func (c *Cache) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	return c.store.Add(ctx, key, value, c.resolveTTL(ttl))
}

// Increment increments a numeric value and returns the result.
// @group Cache
func (c *Cache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return c.store.Increment(ctx, key, delta, c.resolveTTL(ttl))
}

// Decrement decrements a numeric value and returns the result.
// @group Cache
func (c *Cache) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return c.store.Decrement(ctx, key, delta, c.resolveTTL(ttl))
}

// Pull returns value and removes it from cache.
// @group Cache
func (c *Cache) Pull(ctx context.Context, key string) ([]byte, bool, error) {
	body, ok, err := c.Get(ctx, key)
	if err != nil || !ok {
		return nil, ok, err
	}
	if err := c.Delete(ctx, key); err != nil {
		return nil, false, err
	}
	return body, true, nil
}

// Delete removes a single key.
// @group Cache
func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.store.Delete(ctx, key)
}

// DeleteMany removes multiple keys.
// @group Cache
func (c *Cache) DeleteMany(ctx context.Context, keys ...string) error {
	return c.store.DeleteMany(ctx, keys...)
}

// Flush clears all keys for this store scope.
// @group Cache
func (c *Cache) Flush(ctx context.Context) error {
	return c.store.Flush(ctx)
}

// Remember returns key value or computes/stores it when missing.
// @group Cache
func (c *Cache) Remember(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	body, ok, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if ok {
		return body, nil
	}
	if fn == nil {
		return nil, errors.New("cache remember requires a callback")
	}
	body, err = fn(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.Set(ctx, key, body, ttl); err != nil {
		return nil, err
	}
	return body, nil
}

// RememberString returns key value or computes/stores it when missing.
// @group Cache
func (c *Cache) RememberString(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) (string, error)) (string, error) {
	value, err := c.Remember(ctx, key, ttl, func(ctx context.Context) ([]byte, error) {
		if fn == nil {
			return nil, errors.New("cache remember string requires a callback")
		}
		body, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return []byte(body), nil
	})
	if err != nil {
		return "", err
	}
	return string(value), nil
}

// RememberJSON returns key value or computes/stores JSON when missing.
// @group Cache JSON
func RememberJSON[T any](ctx context.Context, cache *Cache, key string, ttl time.Duration, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	out, ok, err := GetJSON[T](ctx, cache, key)
	if err != nil {
		return zero, err
	}
	if ok {
		return out, nil
	}
	if fn == nil {
		return zero, errors.New("cache remember json requires a callback")
	}
	value, err := fn(ctx)
	if err != nil {
		return zero, err
	}
	if err := SetJSON(ctx, cache, key, value, ttl); err != nil {
		return zero, err
	}
	return value, nil
}

func (c *Cache) resolveTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return c.defaultTTL
}
