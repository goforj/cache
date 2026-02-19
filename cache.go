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
	observer   Observer
}

// NewCache creates a cache facade bound to a concrete store.
// @group Cache
//
// Example: cache from store
//
//	ctx := context.Background()
//	s := cache.NewMemoryStore(ctx)
//	c := cache.NewCache(s)
//	fmt.Println(c.Driver()) // DriverMemory
func NewCache(store Store) *Cache {
	return NewCacheWithTTL(store, defaultCacheTTL)
}

// NewCacheWithTTL lets callers override the default TTL applied when ttl <= 0.
// @group Cache
//
// Example: cache with custom default TTL
//
//	ctx := context.Background()
//	s := cache.NewMemoryStore(ctx)
//	c := cache.NewCacheWithTTL(s, 2*time.Minute)
//	fmt.Println(c.Driver(), c != nil) // DriverMemory true
func NewCacheWithTTL(store Store, defaultTTL time.Duration) *Cache {
	if defaultTTL <= 0 {
		defaultTTL = defaultCacheTTL
	}
	return &Cache{
		store:      store,
		defaultTTL: defaultTTL,
	}
}

// WithObserver attaches an observer to receive operation events.
func (c *Cache) WithObserver(o Observer) *Cache {
	c.observer = o
	return c
}

// Store returns the underlying store implementation.
// @group Cache
//
// Example: access store
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	fmt.Println(c.Store().Driver()) // DriverMemory
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
//	s := cache.NewMemoryStore(ctx)
//	c := cache.NewCache(s)
//	_ = c.Set(ctx, "user:42", []byte("Ada"), 0)
//	value, ok, _ := c.Get(ctx, "user:42")
//	fmt.Println(ok, string(value)) // true Ada
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	start := time.Now()
	body, ok, err := c.store.Get(ctx, key)
	c.observe(ctx, "get", key, ok, err, start)
	return body, ok, err
}

// GetString returns a UTF-8 string value for key when present.
// @group Cache
//
// Example: get string
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = c.SetString(ctx, "user:42:name", "Ada", 0)
//	name, ok, _ := c.GetString(ctx, "user:42:name")
//	fmt.Println(ok, name) // true Ada
func (c *Cache) GetString(ctx context.Context, key string) (string, bool, error) {
	start := time.Now()
	body, ok, err := c.Get(ctx, key)
	if err != nil || !ok {
		c.observe(ctx, "get_string", key, ok, err, start)
		return "", ok, err
	}
	val := string(body)
	c.observe(ctx, "get_string", key, true, nil, start)
	return val, true, nil
}

// GetJSON decodes a JSON value into T when key exists.
// @group Cache JSON
//
// Example: get JSON
//
//	type Profile struct { Name string `json:"name"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = cache.SetJSON(ctx, c, "profile:42", Profile{Name: "Ada"}, 0)
//	profile, ok, _ := cache.GetJSON[Profile](ctx, c, "profile:42")
//	fmt.Println(ok, profile.Name) // true Ada
func GetJSON[T any](ctx context.Context, cache *Cache, key string) (T, bool, error) {
	var zero T
	start := time.Now()
	body, ok, err := cache.Get(ctx, key)
	if err != nil || !ok {
		cache.observe(ctx, "get_json", key, ok, err, start)
		return zero, ok, err
	}
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		cache.observe(ctx, "get_json", key, false, err, start)
		return zero, false, err
	}
	cache.observe(ctx, "get_json", key, true, nil, start)
	return out, true, nil
}

// Set writes raw bytes to key.
// @group Cache
//
// Example: set bytes with ttl
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	fmt.Println(c.Set(ctx, "token", []byte("abc"), time.Minute) == nil) // true
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	start := time.Now()
	err := c.store.Set(ctx, key, value, c.resolveTTL(ttl))
	c.observe(ctx, "set", key, false, err, start)
	return err
}

// SetString writes a string value to key.
// @group Cache
//
// Example: set string
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	fmt.Println(c.SetString(ctx, "user:42:name", "Ada", time.Minute) == nil) // true
func (c *Cache) SetString(ctx context.Context, key string, value string, ttl time.Duration) error {
	start := time.Now()
	err := c.Set(ctx, key, []byte(value), ttl)
	c.observe(ctx, "set_string", key, false, err, start)
	return err
}

// SetJSON encodes value as JSON and writes it to key.
// @group Cache JSON
//
// Example: set JSON
//
//	type Profile struct { Name string `json:"name"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	fmt.Println(cache.SetJSON(ctx, c, "profile:42", Profile{Name: "Ada"}, time.Minute) == nil) // true
func SetJSON[T any](ctx context.Context, cache *Cache, key string, value T, ttl time.Duration) error {
	start := time.Now()
	body, err := json.Marshal(value)
	if err != nil {
		cache.observe(ctx, "set_json", key, false, err, start)
		return err
	}
	err = cache.Set(ctx, key, body, ttl)
	cache.observe(ctx, "set_json", key, false, err, start)
	return err
}

// Add writes value only when key is not already present.
// @group Cache
//
// Example: add once
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	created, _ := c.Add(ctx, "boot:seeded", []byte("1"), time.Hour)
//	fmt.Println(created) // true
func (c *Cache) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	start := time.Now()
	created, err := c.store.Add(ctx, key, value, c.resolveTTL(ttl))
	c.observe(ctx, "add", key, created, err, start)
	return created, err
}

// Increment increments a numeric value and returns the result.
// @group Cache
//
// Example: increment counter
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	val, _ := c.Increment(ctx, "rate:login:42", 1, time.Minute)
//	fmt.Println(val) // 1
func (c *Cache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	start := time.Now()
	val, err := c.store.Increment(ctx, key, delta, c.resolveTTL(ttl))
	c.observe(ctx, "increment", key, err == nil, err, start)
	return val, err
}

// Decrement decrements a numeric value and returns the result.
// @group Cache
//
// Example: decrement counter
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	val, _ := c.Decrement(ctx, "rate:login:42", 1, time.Minute)
//	fmt.Println(val) // -1
func (c *Cache) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	start := time.Now()
	val, err := c.store.Decrement(ctx, key, delta, c.resolveTTL(ttl))
	c.observe(ctx, "decrement", key, err == nil, err, start)
	return val, err
}

// Pull returns value and removes it from cache.
// @group Cache
//
// Example: pull and delete
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = c.SetString(ctx, "reset:token:42", "abc", time.Minute)
//	body, ok, _ := c.Pull(ctx, "reset:token:42")
//	fmt.Println(ok, string(body)) // true abc
func (c *Cache) Pull(ctx context.Context, key string) ([]byte, bool, error) {
	start := time.Now()
	body, ok, err := c.Get(ctx, key)
	if err != nil || !ok {
		c.observe(ctx, "pull", key, ok, err, start)
		return nil, ok, err
	}
	if err := c.Delete(ctx, key); err != nil {
		c.observe(ctx, "pull", key, false, err, start)
		return nil, false, err
	}
	c.observe(ctx, "pull", key, true, nil, start)
	return body, true, nil
}

// Delete removes a single key.
// @group Cache
//
// Example: delete key
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = c.Set(ctx, "a", []byte("1"), time.Minute)
//	fmt.Println(c.Delete(ctx, "a") == nil) // true
func (c *Cache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := c.store.Delete(ctx, key)
	c.observe(ctx, "delete", key, err == nil, err, start)
	return err
}

// DeleteMany removes multiple keys.
// @group Cache
//
// Example: delete many keys
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	fmt.Println(c.DeleteMany(ctx, "a", "b") == nil) // true
func (c *Cache) DeleteMany(ctx context.Context, keys ...string) error {
	start := time.Now()
	err := c.store.DeleteMany(ctx, keys...)
	for _, key := range keys {
		c.observe(ctx, "delete_many", key, err == nil, err, start)
	}
	return err
}

// Flush clears all keys for this store scope.
// @group Cache
//
// Example: flush all keys
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = c.Set(ctx, "a", []byte("1"), time.Minute)
//	fmt.Println(c.Flush(ctx) == nil) // true
func (c *Cache) Flush(ctx context.Context) error {
	start := time.Now()
	err := c.store.Flush(ctx)
	c.observe(ctx, "flush", "", err == nil, err, start)
	return err
}

// Remember returns key value or computes/stores it when missing.
// @group Cache
//
// Example: remember bytes
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	data, err := c.Remember(ctx, "dashboard:summary", time.Minute, func(context.Context) ([]byte, error) {
//		return []byte("payload"), nil
//	})
//	fmt.Println(err == nil, string(data)) // true payload
func (c *Cache) Remember(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	start := time.Now()
	body, ok, err := c.Get(ctx, key)
	if err != nil {
		c.observe(ctx, "remember", key, ok, err, start)
		return nil, err
	}
	if ok {
		c.observe(ctx, "remember", key, true, nil, start)
		return body, nil
	}
	if fn == nil {
		c.observe(ctx, "remember", key, false, errors.New("cache remember requires a callback"), start)
		return nil, errors.New("cache remember requires a callback")
	}
	body, err = fn(ctx)
	if err != nil {
		c.observe(ctx, "remember", key, false, err, start)
		return nil, err
	}
	if err := c.Set(ctx, key, body, ttl); err != nil {
		c.observe(ctx, "remember", key, false, err, start)
		return nil, err
	}
	c.observe(ctx, "remember", key, true, nil, start)
	return body, nil
}

// RememberString returns key value or computes/stores it when missing.
// @group Cache
//
// Example: remember string
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	val, err := c.RememberString(ctx, "settings:mode", time.Minute, func(context.Context) (string, error) {
//		return "on", nil
//	})
//	fmt.Println(err == nil, val) // true on
func (c *Cache) RememberString(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) (string, error)) (string, error) {
	start := time.Now()
	value, err := c.Remember(ctx, key, ttl, func(ctx context.Context) ([]byte, error) {
		if fn == nil {
			c.observe(ctx, "remember_string", key, false, errors.New("cache remember string requires a callback"), start)
			return nil, errors.New("cache remember string requires a callback")
		}
		body, err := fn(ctx)
		if err != nil {
			c.observe(ctx, "remember_string", key, false, err, start)
			return nil, err
		}
		return []byte(body), nil
	})
	if err != nil {
		c.observe(ctx, "remember_string", key, false, err, start)
		return "", err
	}
	out := string(value)
	c.observe(ctx, "remember_string", key, true, nil, start)
	return out, nil
}

// RememberJSON returns key value or computes/stores JSON when missing.
// @group Cache JSON
//
// Example: remember JSON
//
//	type Settings struct { Enabled bool `json:"enabled"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	settings, err := cache.RememberJSON[Settings](ctx, c, "settings:alerts", time.Minute, func(context.Context) (Settings, error) {
//		return Settings{Enabled: true}, nil
//	})
//	fmt.Println(err == nil, settings.Enabled) // true true
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

func (c *Cache) observe(ctx context.Context, op, key string, hit bool, err error, start time.Time) {
	if c.observer == nil {
		return
	}
	c.observer.OnCacheOp(ctx, op, key, hit, err, time.Since(start), c.Driver())
}
