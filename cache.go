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
//	fmt.Println(c.Driver()) // memory
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
//	fmt.Println(c.Driver(), c != nil) // memory true
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
//	fmt.Println(c.Store().Driver()) // memory
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
//	_ = c.Set("user:42", []byte("Ada"), 0)
//	value, ok, _ := c.Get("user:42")
//	fmt.Println(ok, string(value)) // true Ada
func (c *Cache) Get(key string) ([]byte, bool, error) {
	return c.GetCtx(context.Background(), key)
}

func (c *Cache) GetCtx(ctx context.Context, key string) ([]byte, bool, error) {
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
//	_ = c.SetString("user:42:name", "Ada", 0)
//	name, ok, _ := c.GetString("user:42:name")
//	fmt.Println(ok, name) // true Ada
func (c *Cache) GetString(key string) (string, bool, error) {
	return c.GetStringCtx(context.Background(), key)
}

func (c *Cache) GetStringCtx(ctx context.Context, key string) (string, bool, error) {
	start := time.Now()
	body, ok, err := c.GetCtx(ctx, key)
	if err != nil || !ok {
		c.observe(ctx, "get_string", key, ok, err, start)
		return "", ok, err
	}
	val := string(body)
	c.observe(ctx, "get_string", key, true, nil, start)
	return val, true, nil
}

// GetJSON decodes a JSON value into T when key exists, using background context.
// @group Cache JSON
func GetJSON[T any](cache *Cache, key string) (T, bool, error) {
	return GetJSONCtx[T](context.Background(), cache, key)
}

// GetJSONCtx is the context-aware variant of GetJSON.
func GetJSONCtx[T any](ctx context.Context, cache *Cache, key string) (T, bool, error) {
	var zero T
	start := time.Now()
	body, ok, err := cache.GetCtx(ctx, key)
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
//	fmt.Println(c.Set("token", []byte("abc"), time.Minute) == nil) // true
func (c *Cache) Set(key string, value []byte, ttl time.Duration) error {
	return c.SetCtx(context.Background(), key, value, ttl)
}

func (c *Cache) SetCtx(ctx context.Context, key string, value []byte, ttl time.Duration) error {
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
//	fmt.Println(c.SetString("user:42:name", "Ada", time.Minute) == nil) // true
func (c *Cache) SetString(key string, value string, ttl time.Duration) error {
	return c.SetStringCtx(context.Background(), key, value, ttl)
}

func (c *Cache) SetStringCtx(ctx context.Context, key string, value string, ttl time.Duration) error {
	start := time.Now()
	err := c.SetCtx(ctx, key, []byte(value), ttl)
	c.observe(ctx, "set_string", key, false, err, start)
	return err
}

// SetJSON encodes value as JSON and writes it to key using background context.
// @group Cache JSON
func SetJSON[T any](cache *Cache, key string, value T, ttl time.Duration) error {
	return SetJSONCtx[T](context.Background(), cache, key, value, ttl)
}

// SetJSONCtx is the context-aware variant of SetJSON.
func SetJSONCtx[T any](ctx context.Context, cache *Cache, key string, value T, ttl time.Duration) error {
	start := time.Now()
	body, err := json.Marshal(value)
	if err != nil {
		cache.observe(ctx, "set_json", key, false, err, start)
		return err
	}
	err = cache.SetCtx(ctx, key, body, ttl)
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
//	created, _ := c.Add("boot:seeded", []byte("1"), time.Hour)
//	fmt.Println(created) // true
func (c *Cache) Add(key string, value []byte, ttl time.Duration) (bool, error) {
	return c.AddCtx(context.Background(), key, value, ttl)
}

func (c *Cache) AddCtx(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
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
//	val, _ := c.Increment("rate:login:42", 1, time.Minute)
//	fmt.Println(val) // 1
func (c *Cache) Increment(key string, delta int64, ttl time.Duration) (int64, error) {
	return c.IncrementCtx(context.Background(), key, delta, ttl)
}

func (c *Cache) IncrementCtx(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
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
//	val, _ := c.Decrement("rate:login:42", 1, time.Minute)
//	fmt.Println(val) // -1
func (c *Cache) Decrement(key string, delta int64, ttl time.Duration) (int64, error) {
	return c.DecrementCtx(context.Background(), key, delta, ttl)
}

func (c *Cache) DecrementCtx(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
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
//	_ = c.SetString("reset:token:42", "abc", time.Minute)
//	body, ok, _ := c.Pull("reset:token:42")
//	fmt.Println(ok, string(body)) // true abc
func (c *Cache) Pull(key string) ([]byte, bool, error) {
	return c.PullCtx(context.Background(), key)
}

func (c *Cache) PullCtx(ctx context.Context, key string) ([]byte, bool, error) {
	start := time.Now()
	body, ok, err := c.GetCtx(ctx, key)
	if err != nil || !ok {
		c.observe(ctx, "pull", key, ok, err, start)
		return nil, ok, err
	}
	if err := c.DeleteCtx(ctx, key); err != nil {
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
//	_ = c.Set("a", []byte("1"), time.Minute)
//	fmt.Println(c.Delete("a") == nil) // true
func (c *Cache) Delete(key string) error {
	return c.DeleteCtx(context.Background(), key)
}

func (c *Cache) DeleteCtx(ctx context.Context, key string) error {
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
//	fmt.Println(c.DeleteMany("a", "b") == nil) // true
func (c *Cache) DeleteMany(keys ...string) error {
	return c.DeleteManyCtx(context.Background(), keys...)
}

func (c *Cache) DeleteManyCtx(ctx context.Context, keys ...string) error {
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
//	_ = c.Set("a", []byte("1"), time.Minute)
//	fmt.Println(c.Flush() == nil) // true
func (c *Cache) Flush() error {
	return c.FlushCtx(context.Background())
}

func (c *Cache) FlushCtx(ctx context.Context) error {
	start := time.Now()
	err := c.store.Flush(ctx)
	c.observe(ctx, "flush", "", err == nil, err, start)
	return err
}

// RememberBytes returns key value or computes/stores it when missing.
// @group Cache
//
// Example: remember bytes
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	data, err := c.RememberBytes("dashboard:summary", time.Minute, func() ([]byte, error) {
//		return []byte("payload"), nil
//	})
//	fmt.Println(err == nil, string(data)) // true payload
func (c *Cache) RememberBytes(key string, ttl time.Duration, fn func() ([]byte, error)) ([]byte, error) {
    return c.RememberCtx(context.Background(), key, ttl, func(ctx context.Context) ([]byte, error) {
        if fn == nil {
            return nil, errors.New("cache remember requires a callback")
        }
        return fn()
    })
}

func (c *Cache) RememberCtx(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	start := time.Now()
	body, ok, err := c.GetCtx(ctx, key)
	if err != nil {
		c.observe(ctx, "remember", key, ok, err, start)
		return nil, err
	}
	if ok {
		c.observe(ctx, "remember", key, true, nil, start)
		return body, nil
	}
	if fn == nil {
		err := errors.New("cache remember requires a callback")
		c.observe(ctx, "remember", key, false, err, start)
		return nil, err
	}
	body, err = fn(ctx)
	if err != nil {
		c.observe(ctx, "remember", key, false, err, start)
		return nil, err
	}
	if err := c.SetCtx(ctx, key, body, ttl); err != nil {
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
//	val, err := c.RememberString("settings:mode", time.Minute, func() (string, error) {
//		return "on", nil
//	})
//	fmt.Println(err == nil, val) // true on
func (c *Cache) RememberString(key string, ttl time.Duration, fn func() (string, error)) (string, error) {
	return c.RememberStringCtx(context.Background(), key, ttl, func(context.Context) (string, error) {
		if fn == nil {
			return "", errors.New("cache remember string requires a callback")
		}
		return fn()
	})
}

func (c *Cache) RememberStringCtx(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) (string, error)) (string, error) {
	start := time.Now()
	value, err := c.RememberCtx(ctx, key, ttl, func(ctx context.Context) ([]byte, error) {
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
//	settings, err := cache.RememberJSON[Settings](c, "settings:alerts", time.Minute, func() (Settings, error) {
//		return Settings{Enabled: true}, nil
//	})
//	fmt.Println(err == nil, settings.Enabled) // true true
func RememberJSON[T any](cache *Cache, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	return RememberJSONCtx(context.Background(), cache, key, ttl, func(ctx context.Context) (T, error) {
		if fn == nil {
			var zero T
			return zero, errors.New("cache remember json requires a callback")
		}
		return fn()
	})
}

// Remember is the ergonomic, typed remember helper using JSON encoding by default.
// @group Cache
func Remember[T any](cache *Cache, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
    return RememberValue(cache, key, ttl, fn)
}

// ValueCodec defines how to encode/decode values for RememberValue.
type ValueCodec[T any] struct {
	Encode func(T) ([]byte, error)
	Decode func([]byte) (T, error)
}

// defaultValueCodec uses JSON to encode/decode values.
func defaultValueCodec[T any]() ValueCodec[T] {
	return ValueCodec[T]{
		Encode: func(v T) ([]byte, error) { return json.Marshal(v) },
		Decode: func(b []byte) (T, error) {
			var out T
			err := json.Unmarshal(b, &out)
			return out, err
		},
	}
}

// RememberValue returns a typed value or computes/stores it when missing using JSON encoding by default.
// @group Cache
func RememberValue[T any](cache *Cache, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
    return RememberValueWithCodec(context.Background(), cache, key, ttl, fn, defaultValueCodec[T]())
}

// RememberValueWithCodec allows custom encoding/decoding for typed remember operations.
func RememberValueWithCodec[T any](ctx context.Context, cache *Cache, key string, ttl time.Duration, fn func() (T, error), codec ValueCodec[T]) (T, error) {
	var zero T
	body, ok, err := cache.GetCtx(ctx, key)
	if err != nil {
		return zero, err
	}
	if ok {
		return codec.Decode(body)
	}
	if fn == nil {
		return zero, errors.New("cache remember value requires a callback")
	}
	val, err := fn()
	if err != nil {
		return zero, err
	}
	encoded, err := codec.Encode(val)
	if err != nil {
		return zero, err
	}
	if err := cache.SetCtx(ctx, key, encoded, ttl); err != nil {
		return zero, err
	}
	return val, nil
}

func RememberJSONCtx[T any](ctx context.Context, cache *Cache, key string, ttl time.Duration, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	out, ok, err := GetJSONCtx[T](ctx, cache, key)
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
	if err := SetJSONCtx(ctx, cache, key, value, ttl); err != nil {
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
