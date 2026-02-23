package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/goforj/cache/cachecore"
)

// Cache provides an ergonomic cache API on top of Store.
type Cache struct {
	store      cachecore.Store
	defaultTTL time.Duration
	observer   Observer
}

// RateLimitStatus contains fixed-window rate limiting metadata.
// @group Rate Limiting
type RateLimitStatus struct {
	Allowed   bool
	Count     int64
	Remaining int64
	ResetAt   time.Time
}

// NewCache creates a cache facade bound to a concrete store.
// @group Core
//
// Example: cache from store
//
//	ctx := context.Background()
//	s := cache.NewMemoryStore(ctx)
//	c := cache.NewCache(s)
//	fmt.Println(c.Driver()) // memory
func NewCache(store cachecore.Store) *Cache {
	return NewCacheWithTTL(store, defaultCacheTTL)
}

// NewCacheWithTTL lets callers override the default TTL applied when ttl <= 0.
// @group Core
//
// Example: cache with custom default TTL
//
//	ctx := context.Background()
//	s := cache.NewMemoryStore(ctx)
//	c := cache.NewCacheWithTTL(s, 2*time.Minute)
//	fmt.Println(c.Driver(), c != nil) // memory true
func NewCacheWithTTL(store cachecore.Store, defaultTTL time.Duration) *Cache {
	if defaultTTL <= 0 {
		defaultTTL = defaultCacheTTL
	}
	return &Cache{
		store:      store,
		defaultTTL: defaultTTL,
	}
}

// WithObserver attaches an observer to receive operation events.
// @group Observability
//
// Example: attach observer
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	c = c.WithObserver(cache.ObserverFunc(func(ctx context.Context, op, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver) {
//		// See docs/production-guide.md for a real metrics recipe.
//		fmt.Println(op, driver, hit, err == nil)
//		_ = ctx
//		_ = key
//		_ = dur
//	}))
//	_, _, _ = c.GetBytes("profile:42")
func (c *Cache) WithObserver(o Observer) *Cache {
	c.observer = o
	return c
}

// Store returns the underlying store implementation.
// @group Core
//
// Example: access store
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	fmt.Println(c.Store().Driver()) // memory
func (c *Cache) Store() cachecore.Store {
	return c.store
}

// Driver reports the underlying store driver.
// @group Core
func (c *Cache) Driver() cachecore.Driver {
	return c.store.Driver()
}

// GetBytes returns raw bytes for key when present.
// @group Reads
//
// Example: get bytes
//
//	ctx := context.Background()
//	s := cache.NewMemoryStore(ctx)
//	c := cache.NewCache(s)
//	_ = c.SetBytes("user:42", []byte("Ada"), 0)
//	value, ok, _ := c.GetBytes("user:42")
//	fmt.Println(ok, string(value)) // true Ada
func (c *Cache) GetBytes(key string) ([]byte, bool, error) {
	return c.GetBytesCtx(context.Background(), key)
}

// GetBytesCtx is the context-aware variant of GetBytes.
// @group Reads
func (c *Cache) GetBytesCtx(ctx context.Context, key string) ([]byte, bool, error) {
	start := time.Now()
	body, ok, err := c.store.Get(ctx, key)
	c.observe(ctx, "get", key, ok, err, start)
	return body, ok, err
}

// BatchGetBytes returns all found values for the provided keys.
// Missing keys are omitted from the returned map.
// @group Reads
//
// Example: batch get keys
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = c.SetBytes("a", []byte("1"), time.Minute)
//	_ = c.SetBytes("b", []byte("2"), time.Minute)
//	values, err := c.BatchGetBytes("a", "b", "missing")
//	fmt.Println(err == nil, string(values["a"]), string(values["b"])) // true 1 2
func (c *Cache) BatchGetBytes(keys ...string) (map[string][]byte, error) {
	return c.BatchGetBytesCtx(context.Background(), keys...)
}

// BatchGetBytesCtx is the context-aware variant of BatchGetBytes.
// @group Reads
func (c *Cache) BatchGetBytesCtx(ctx context.Context, keys ...string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(keys))
	for _, key := range keys {
		body, ok, err := c.GetBytesCtx(ctx, key)
		if err != nil {
			return nil, err
		}
		if ok {
			out[key] = body
		}
	}
	return out, nil
}

// GetString returns a UTF-8 string value for key when present.
// @group Reads
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

// GetStringCtx is the context-aware variant of GetString.
// @group Reads
func (c *Cache) GetStringCtx(ctx context.Context, key string) (string, bool, error) {
	start := time.Now()
	body, ok, err := c.GetBytesCtx(ctx, key)
	if err != nil || !ok {
		c.observe(ctx, "get_string", key, ok, err, start)
		return "", ok, err
	}
	val := string(body)
	c.observe(ctx, "get_string", key, true, nil, start)
	return val, true, nil
}

// GetJSON decodes a JSON value into T when key exists, using background context.
// @group Reads
//
// Example: get typed JSON
//
//	type Profile struct { Name string `json:"name"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = cache.SetJSON(c, "profile:42", Profile{Name: "Ada"}, time.Minute)
//	profile, ok, err := cache.GetJSON[Profile](c, "profile:42")
//	fmt.Println(err == nil, ok, profile.Name) // true true Ada
func GetJSON[T any](cache *Cache, key string) (T, bool, error) {
	return GetJSONCtx[T](context.Background(), cache, key)
}

// GetJSONCtx is the context-aware variant of GetJSON.
// @group Reads
func GetJSONCtx[T any](ctx context.Context, cache *Cache, key string) (T, bool, error) {
	var zero T
	start := time.Now()
	body, ok, err := cache.GetBytesCtx(ctx, key)
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

// Get returns a typed value for key using the default codec (JSON) when present.
// @group Reads
//
// Example: get typed values (struct + string)
//
//	type Profile struct { Name string `json:"name"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = cache.Set(c, "profile:42", Profile{Name: "Ada"}, time.Minute)
//	_ = cache.Set(c, "settings:mode", "dark", time.Minute)
//	profile, ok, err := cache.Get[Profile](c, "profile:42")
//	mode, ok2, err2 := cache.Get[string](c, "settings:mode")
//	fmt.Println(err == nil, ok, profile.Name, err2 == nil, ok2, mode) // true true Ada true true dark
func Get[T any](cache *Cache, key string) (T, bool, error) {
	return GetCtx[T](context.Background(), cache, key)
}

// GetCtx is the context-aware variant of Get.
// @group Reads
func GetCtx[T any](ctx context.Context, cache *Cache, key string) (T, bool, error) {
	var zero T
	body, ok, err := cache.GetBytesCtx(ctx, key)
	if err != nil || !ok {
		return zero, ok, err
	}
	out, err := defaultValueCodec[T]().Decode(body)
	if err != nil {
		return zero, false, err
	}
	return out, true, nil
}

// SetBytes writes raw bytes to key.
// @group Writes
//
// Example: set bytes with ttl
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	fmt.Println(c.SetBytes("token", []byte("abc"), time.Minute) == nil) // true
func (c *Cache) SetBytes(key string, value []byte, ttl time.Duration) error {
	return c.SetBytesCtx(context.Background(), key, value, ttl)
}

// SetBytesCtx is the context-aware variant of SetBytes.
// @group Writes
func (c *Cache) SetBytesCtx(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	start := time.Now()
	err := c.store.Set(ctx, key, value, c.resolveTTL(ttl))
	c.observe(ctx, "set", key, false, err, start)
	return err
}

// BatchSetBytes writes many key/value pairs using a shared ttl.
// @group Writes
//
// Example: batch set keys
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	err := c.BatchSetBytes(map[string][]byte{
//		"a": []byte("1"),
//		"b": []byte("2"),
//	}, time.Minute)
//	fmt.Println(err == nil) // true
func (c *Cache) BatchSetBytes(values map[string][]byte, ttl time.Duration) error {
	return c.BatchSetBytesCtx(context.Background(), values, ttl)
}

// BatchSetBytesCtx is the context-aware variant of BatchSetBytes.
// @group Writes
func (c *Cache) BatchSetBytesCtx(ctx context.Context, values map[string][]byte, ttl time.Duration) error {
	for key, value := range values {
		if err := c.SetBytesCtx(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// RefreshAheadBytes returns cached value immediately and refreshes asynchronously when near expiry.
// On miss, it computes and stores synchronously.
// @group Refresh Ahead
//
// Example: refresh ahead
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	body, err := c.RefreshAheadBytes("dashboard:summary", time.Minute, 10*time.Second, func() ([]byte, error) {
//		return []byte("payload"), nil
//	})
//	fmt.Println(err == nil, len(body) > 0) // true true
func (c *Cache) RefreshAheadBytes(key string, ttl, refreshAhead time.Duration, fn func() ([]byte, error)) ([]byte, error) {
	return c.RefreshAheadBytesCtx(context.Background(), key, ttl, refreshAhead, func(ctx context.Context) ([]byte, error) {
		if fn == nil {
			return nil, errors.New("cache refresh ahead requires a callback")
		}
		return fn()
	})
}

// RefreshAheadBytesCtx is the context-aware variant of RefreshAheadBytes.
// @group Refresh Ahead
func (c *Cache) RefreshAheadBytesCtx(ctx context.Context, key string, ttl, refreshAhead time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	if ttl <= 0 {
		return nil, errors.New("cache refresh ahead requires ttl > 0")
	}
	if refreshAhead <= 0 {
		return nil, errors.New("cache refresh ahead requires refreshAhead > 0")
	}
	start := time.Now()
	body, ok, err := c.GetBytesCtx(ctx, key)
	if err != nil {
		c.observe(ctx, "refresh_ahead", key, false, err, start)
		return nil, err
	}
	if ok {
		c.maybeTriggerRefreshAhead(key, ttl, refreshAhead, fn)
		c.observe(ctx, "refresh_ahead", key, true, nil, start)
		return body, nil
	}
	if fn == nil {
		err := errors.New("cache refresh ahead requires a callback")
		c.observe(ctx, "refresh_ahead", key, false, err, start)
		return nil, err
	}
	value, err := fn(ctx)
	if err != nil {
		c.observe(ctx, "refresh_ahead", key, false, err, start)
		return nil, err
	}
	if err := c.setRefreshAheadValue(ctx, key, value, ttl); err != nil {
		c.observe(ctx, "refresh_ahead", key, false, err, start)
		return nil, err
	}
	c.observe(ctx, "refresh_ahead", key, true, nil, start)
	return value, nil
}

func (c *Cache) maybeTriggerRefreshAhead(key string, ttl, refreshAhead time.Duration, fn func(context.Context) ([]byte, error)) {
	metaKey := key + refreshMetaSuffix
	meta, ok, err := c.GetBytesCtx(context.Background(), metaKey)
	if err != nil || !ok {
		return
	}
	expiresAt, err := strconv.ParseInt(string(meta), 10, 64)
	if err != nil {
		return
	}
	if time.Until(time.Unix(0, expiresAt)) > refreshAhead {
		return
	}
	go func() {
		lockKey := refreshLockPrefix + key
		locked, err := c.TryLock(lockKey, refreshAhead)
		if err != nil || !locked {
			return
		}
		defer func() { _ = c.Unlock(lockKey) }()
		if fn == nil {
			return
		}
		value, err := fn(context.Background())
		if err != nil {
			return
		}
		_ = c.setRefreshAheadValue(context.Background(), key, value, ttl)
	}()
}

func (c *Cache) setRefreshAheadValue(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.SetBytesCtx(ctx, key, value, ttl); err != nil {
		return err
	}
	expiresAt := time.Now().Add(c.resolveTTL(ttl)).UnixNano()
	return c.SetBytesCtx(ctx, key+refreshMetaSuffix, []byte(strconv.FormatInt(expiresAt, 10)), ttl)
}

// RefreshAhead returns a typed value and refreshes asynchronously when near expiry.
// @group Refresh Ahead
//
// Example: refresh ahead typed
//
//	type Summary struct { Text string `json:"text"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	s, err := cache.RefreshAhead[Summary](c, "dashboard:summary", time.Minute, 10*time.Second, func() (Summary, error) {
//		return Summary{Text: "ok"}, nil
//	})
//	fmt.Println(err == nil, s.Text) // true ok
func RefreshAhead[T any](cache *Cache, key string, ttl, refreshAhead time.Duration, fn func() (T, error)) (T, error) {
	return RefreshAheadCtx(context.Background(), cache, key, ttl, refreshAhead, func(ctx context.Context) (T, error) {
		if fn == nil {
			var zero T
			return zero, errors.New("cache refresh ahead requires a callback")
		}
		return fn()
	})
}

// RefreshAheadCtx is the context-aware variant of RefreshAhead.
// @group Refresh Ahead
func RefreshAheadCtx[T any](ctx context.Context, cache *Cache, key string, ttl, refreshAhead time.Duration, fn func(context.Context) (T, error)) (T, error) {
	return RefreshAheadValueWithCodec(ctx, cache, key, ttl, refreshAhead, func() (T, error) {
		if fn == nil {
			var zero T
			return zero, errors.New("cache refresh ahead requires a callback")
		}
		return fn(ctx)
	}, defaultValueCodec[T]())
}

// RefreshAheadValueWithCodec allows custom encoding/decoding for typed refresh-ahead operations.
// @group Refresh Ahead
func RefreshAheadValueWithCodec[T any](ctx context.Context, cache *Cache, key string, ttl, refreshAhead time.Duration, fn func() (T, error), codec ValueCodec[T]) (T, error) {
	var zero T
	body, err := cache.RefreshAheadBytesCtx(ctx, key, ttl, refreshAhead, func(ctx context.Context) ([]byte, error) {
		if fn == nil {
			return nil, errors.New("cache refresh ahead requires a callback")
		}
		val, err := fn()
		if err != nil {
			return nil, err
		}
		return codec.Encode(val)
	})
	if err != nil {
		return zero, err
	}
	out, err := codec.Decode(body)
	if err != nil {
		return zero, err
	}
	return out, nil
}

// SetString writes a string value to key.
// @group Writes
//
// Example: set string
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	fmt.Println(c.SetString("user:42:name", "Ada", time.Minute) == nil) // true
func (c *Cache) SetString(key string, value string, ttl time.Duration) error {
	return c.SetStringCtx(context.Background(), key, value, ttl)
}

// SetStringCtx is the context-aware variant of SetString.
// @group Writes
func (c *Cache) SetStringCtx(ctx context.Context, key string, value string, ttl time.Duration) error {
	start := time.Now()
	err := c.SetBytesCtx(ctx, key, []byte(value), ttl)
	c.observe(ctx, "set_string", key, false, err, start)
	return err
}

// SetJSON encodes value as JSON and writes it to key using background context.
// @group Writes
//
// Example: set typed JSON
//
//	type Settings struct { Enabled bool `json:"enabled"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	err := cache.SetJSON(c, "settings:alerts", Settings{Enabled: true}, time.Minute)
//	fmt.Println(err == nil) // true
func SetJSON[T any](cache *Cache, key string, value T, ttl time.Duration) error {
	return SetJSONCtx[T](context.Background(), cache, key, value, ttl)
}

// SetJSONCtx is the context-aware variant of SetJSON.
// @group Writes
func SetJSONCtx[T any](ctx context.Context, cache *Cache, key string, value T, ttl time.Duration) error {
	start := time.Now()
	body, err := json.Marshal(value)
	if err != nil {
		cache.observe(ctx, "set_json", key, false, err, start)
		return err
	}
	err = cache.SetBytesCtx(ctx, key, body, ttl)
	cache.observe(ctx, "set_json", key, false, err, start)
	return err
}

// Set encodes value with the default codec (JSON) and writes it to key.
// @group Writes
//
// Example: set typed values (struct + string)
//
//	type Settings struct { Enabled bool `json:"enabled"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	err := cache.Set(c, "settings:alerts", Settings{Enabled: true}, time.Minute)
//	err2 := cache.Set(c, "settings:mode", "dark", time.Minute)
//	fmt.Println(err == nil, err2 == nil) // true true
func Set[T any](cache *Cache, key string, value T, ttl time.Duration) error {
	return SetCtx[T](context.Background(), cache, key, value, ttl)
}

// SetCtx is the context-aware variant of Set.
// @group Writes
func SetCtx[T any](ctx context.Context, cache *Cache, key string, value T, ttl time.Duration) error {
	body, err := defaultValueCodec[T]().Encode(value)
	if err != nil {
		return err
	}
	return cache.SetBytesCtx(ctx, key, body, ttl)
}

// Add writes value only when key is not already present.
// @group Writes
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

// AddCtx is the context-aware variant of Add.
// @group Writes
func (c *Cache) AddCtx(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	start := time.Now()
	created, err := c.store.Add(ctx, key, value, c.resolveTTL(ttl))
	c.observe(ctx, "add", key, created, err, start)
	return created, err
}

// Increment increments a numeric value and returns the result.
// @group Writes
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

// IncrementCtx is the context-aware variant of Increment.
// @group Writes
func (c *Cache) IncrementCtx(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	start := time.Now()
	val, err := c.store.Increment(ctx, key, delta, c.resolveTTL(ttl))
	c.observe(ctx, "increment", key, err == nil, err, start)
	return val, err
}

// Decrement decrements a numeric value and returns the result.
// @group Writes
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

// DecrementCtx is the context-aware variant of Decrement.
// @group Writes
func (c *Cache) DecrementCtx(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	start := time.Now()
	val, err := c.store.Decrement(ctx, key, delta, c.resolveTTL(ttl))
	c.observe(ctx, "decrement", key, err == nil, err, start)
	return val, err
}

// RateLimit increments a fixed-window counter and returns allowance metadata.
// @group Rate Limiting
//
// Example: fixed-window rate limit metadata
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	res, err := c.RateLimit("rl:api:ip:1.2.3.4", 100, time.Minute)
//	fmt.Println(err == nil, res.Allowed, res.Count, res.Remaining, !res.ResetAt.IsZero())
//	// Output: true true 1 99 true
func (c *Cache) RateLimit(key string, limit int64, window time.Duration) (RateLimitStatus, error) {
	return c.RateLimitCtx(context.Background(), key, limit, window)
}

// RateLimitCtx is the context-aware variant of RateLimit.
// @group Rate Limiting
func (c *Cache) RateLimitCtx(ctx context.Context, key string, limit int64, window time.Duration) (RateLimitStatus, error) {
	if limit <= 0 {
		return RateLimitStatus{}, errors.New("cache rate limit requires limit > 0")
	}
	if window <= 0 {
		return RateLimitStatus{}, errors.New("cache rate limit requires window > 0")
	}

	now := time.Now()
	bucket := now.UnixNano() / window.Nanoseconds()
	bucketKey := fmt.Sprintf("%s:%d", key, bucket)
	count, err := c.IncrementCtx(ctx, bucketKey, 1, window)
	if err != nil {
		return RateLimitStatus{}, err
	}
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	resetAt := time.Unix(0, (bucket+1)*window.Nanoseconds())
	return RateLimitStatus{
		Allowed:   count <= limit,
		Count:     count,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}

// TryLock acquires a short-lived lock key when not already held.
// @group Locking
//
// Example: try lock
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	locked, _ := c.TryLock("job:sync", 10*time.Second)
//	fmt.Println(locked) // true
func (c *Cache) TryLock(key string, ttl time.Duration) (bool, error) {
	return c.TryLockCtx(context.Background(), key, ttl)
}

// TryLockCtx is the context-aware variant of TryLock.
// @group Locking
func (c *Cache) TryLockCtx(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		return false, errors.New("cache try lock requires ttl > 0")
	}
	start := time.Now()
	created, err := c.store.Add(ctx, lockPrefix+key, []byte("1"), ttl)
	c.observe(ctx, "try_lock", key, created, err, start)
	return created, err
}

// Lock waits until the lock is acquired or timeout elapses.
// @group Locking
//
// Example: lock with timeout
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	locked, err := c.Lock("job:sync", 10*time.Second, time.Second)
//	fmt.Println(err == nil, locked) // true true
func (c *Cache) Lock(key string, ttl, timeout time.Duration) (bool, error) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return c.LockCtx(ctx, key, ttl, defaultLockRetryInterval)
}

// LockCtx retries lock acquisition until success or context cancellation.
// @group Locking
func (c *Cache) LockCtx(ctx context.Context, key string, ttl, retryInterval time.Duration) (bool, error) {
	if retryInterval <= 0 {
		retryInterval = defaultLockRetryInterval
	}
	start := time.Now()
	for {
		locked, err := c.TryLockCtx(ctx, key, ttl)
		if err != nil {
			c.observe(ctx, "lock", key, false, err, start)
			return false, err
		}
		if locked {
			c.observe(ctx, "lock", key, true, nil, start)
			return true, nil
		}
		select {
		case <-ctx.Done():
			err := ctx.Err()
			c.observe(ctx, "lock", key, false, err, start)
			return false, err
		case <-time.After(retryInterval):
		}
	}
}

// Unlock releases a previously acquired lock key.
// @group Locking
//
// Example: unlock key
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	locked, _ := c.TryLock("job:sync", 10*time.Second)
//	if locked {
//		_ = c.Unlock("job:sync")
//	}
func (c *Cache) Unlock(key string) error {
	return c.UnlockCtx(context.Background(), key)
}

// UnlockCtx is the context-aware variant of Unlock.
// @group Locking
func (c *Cache) UnlockCtx(ctx context.Context, key string) error {
	start := time.Now()
	err := c.store.Delete(ctx, lockPrefix+key)
	c.observe(ctx, "unlock", key, err == nil, err, start)
	return err
}

// PullBytes returns value and removes it from cache.
// @group Invalidation
//
// Example: pull and delete
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = c.SetString("reset:token:42", "abc", time.Minute)
//	body, ok, _ := c.PullBytes("reset:token:42")
//	fmt.Println(ok, string(body)) // true abc
func (c *Cache) PullBytes(key string) ([]byte, bool, error) {
	return c.PullBytesCtx(context.Background(), key)
}

// PullBytesCtx is the context-aware variant of PullBytes.
// @group Invalidation
func (c *Cache) PullBytesCtx(ctx context.Context, key string) ([]byte, bool, error) {
	start := time.Now()
	body, ok, err := c.GetBytesCtx(ctx, key)
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

// Pull returns a typed value for key and removes it, using the default codec (JSON).
// @group Invalidation
//
// Example: pull typed value
//
//	type Token struct { Value string `json:"value"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = cache.Set(c, "reset:token:42", Token{Value: "abc"}, time.Minute)
//	tok, ok, err := cache.Pull[Token](c, "reset:token:42")
//	fmt.Println(err == nil, ok, tok.Value) // true true abc
func Pull[T any](cache *Cache, key string) (T, bool, error) {
	return PullCtx[T](context.Background(), cache, key)
}

// PullCtx is the context-aware variant of Pull.
// @group Invalidation
func PullCtx[T any](ctx context.Context, cache *Cache, key string) (T, bool, error) {
	var zero T
	body, ok, err := cache.PullBytesCtx(ctx, key)
	if err != nil || !ok {
		return zero, ok, err
	}
	out, err := defaultValueCodec[T]().Decode(body)
	if err != nil {
		return zero, false, err
	}
	return out, true, nil
}

// Delete removes a single key.
// @group Invalidation
//
// Example: delete key
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = c.SetBytes("a", []byte("1"), time.Minute)
//	fmt.Println(c.Delete("a") == nil) // true
func (c *Cache) Delete(key string) error {
	return c.DeleteCtx(context.Background(), key)
}

// DeleteCtx is the context-aware variant of Delete.
// @group Invalidation
func (c *Cache) DeleteCtx(ctx context.Context, key string) error {
	start := time.Now()
	err := c.store.Delete(ctx, key)
	c.observe(ctx, "delete", key, err == nil, err, start)
	return err
}

// DeleteMany removes multiple keys.
// @group Invalidation
//
// Example: delete many keys
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	fmt.Println(c.DeleteMany("a", "b") == nil) // true
func (c *Cache) DeleteMany(keys ...string) error {
	return c.DeleteManyCtx(context.Background(), keys...)
}

// DeleteManyCtx is the context-aware variant of DeleteMany.
// @group Invalidation
func (c *Cache) DeleteManyCtx(ctx context.Context, keys ...string) error {
	start := time.Now()
	err := c.store.DeleteMany(ctx, keys...)
	for _, key := range keys {
		c.observe(ctx, "delete_many", key, err == nil, err, start)
	}
	return err
}

// Flush clears all keys for this store scope.
// @group Invalidation
//
// Example: flush all keys
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = c.SetBytes("a", []byte("1"), time.Minute)
//	fmt.Println(c.Flush() == nil) // true
func (c *Cache) Flush() error {
	return c.FlushCtx(context.Background())
}

// FlushCtx is the context-aware variant of Flush.
// @group Invalidation
func (c *Cache) FlushCtx(ctx context.Context) error {
	start := time.Now()
	err := c.store.Flush(ctx)
	c.observe(ctx, "flush", "", err == nil, err, start)
	return err
}

// RememberBytes returns key value or computes/stores it when missing.
// @group Read Through
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
	return c.RememberBytesCtx(context.Background(), key, ttl, func(ctx context.Context) ([]byte, error) {
		if fn == nil {
			return nil, errors.New("cache remember requires a callback")
		}
		return fn()
	})
}

const staleSuffix = ":__stale"
const lockPrefix = "__lock:"
const defaultLockRetryInterval = 25 * time.Millisecond
const refreshMetaSuffix = ":__refresh_exp"
const refreshLockPrefix = "__refresh_lock:"

// RememberStaleBytes returns a fresh value when available, otherwise computes and caches it.
// If computing fails and a stale value exists, it returns the stale value.
// The returned bool is true when a stale fallback was used.
// @group Read Through
//
// Example: stale fallback on upstream failure
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	body, usedStale, err := c.RememberStaleBytes("profile:42", time.Minute, 10*time.Minute, func() ([]byte, error) {
//		return []byte(`{"name":"Ada"}`), nil
//	})
//	fmt.Println(err == nil, usedStale, len(body) > 0)
func (c *Cache) RememberStaleBytes(key string, ttl, staleTTL time.Duration, fn func() ([]byte, error)) ([]byte, bool, error) {
	return c.RememberStaleBytesCtx(context.Background(), key, ttl, staleTTL, func(ctx context.Context) ([]byte, error) {
		if fn == nil {
			return nil, errors.New("cache remember stale requires a callback")
		}
		return fn()
	})
}

// RememberStaleBytesCtx is the context-aware variant of RememberStaleBytes.
// @group Read Through
func (c *Cache) RememberStaleBytesCtx(ctx context.Context, key string, ttl, staleTTL time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, bool, error) {
	start := time.Now()
	staleKey := key + staleSuffix

	body, ok, err := c.GetBytesCtx(ctx, key)
	if err != nil {
		c.observe(ctx, "remember_stale", key, false, err, start)
		return nil, false, err
	}
	if ok {
		c.observe(ctx, "remember_stale", key, true, nil, start)
		return body, false, nil
	}
	if fn == nil {
		err := errors.New("cache remember stale requires a callback")
		c.observe(ctx, "remember_stale", key, false, err, start)
		return nil, false, err
	}

	value, err := fn(ctx)
	if err == nil {
		if err := c.SetBytesCtx(ctx, key, value, ttl); err != nil {
			c.observe(ctx, "remember_stale", key, false, err, start)
			return nil, false, err
		}
		if staleTTL <= 0 {
			staleTTL = ttl
		}
		if staleTTL > 0 {
			_ = c.SetBytesCtx(ctx, staleKey, value, staleTTL)
		}
		c.observe(ctx, "remember_stale", key, true, nil, start)
		return value, false, nil
	}

	staleBody, staleOK, staleErr := c.GetBytesCtx(ctx, staleKey)
	if staleErr == nil && staleOK {
		c.observe(ctx, "remember_stale", key, true, nil, start)
		return staleBody, true, nil
	}
	if staleErr != nil {
		err = errors.Join(err, staleErr)
	}
	c.observe(ctx, "remember_stale", key, false, err, start)
	return nil, false, err
}

// RememberBytesCtx is the context-aware variant of RememberBytes.
// @group Read Through
func (c *Cache) RememberBytesCtx(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	start := time.Now()
	body, ok, err := c.GetBytesCtx(ctx, key)
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
	if err := c.SetBytesCtx(ctx, key, body, ttl); err != nil {
		c.observe(ctx, "remember", key, false, err, start)
		return nil, err
	}
	c.observe(ctx, "remember", key, true, nil, start)
	return body, nil
}

// RememberStale returns a typed value with stale fallback semantics using JSON encoding by default.
// @group Read Through
//
// Example: remember stale typed
//
//	type Profile struct { Name string `json:"name"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	profile, usedStale, err := cache.RememberStale[Profile](c, "profile:42", time.Minute, 10*time.Minute, func() (Profile, error) {
//		return Profile{Name: "Ada"}, nil
//	})
//	fmt.Println(err == nil, usedStale, profile.Name) // true false Ada
func RememberStale[T any](cache *Cache, key string, ttl, staleTTL time.Duration, fn func() (T, error)) (T, bool, error) {
	return RememberStaleCtx(context.Background(), cache, key, ttl, staleTTL, func(ctx context.Context) (T, error) {
		if fn == nil {
			var zero T
			return zero, errors.New("cache remember stale requires a callback")
		}
		return fn()
	})
}

// Remember is the ergonomic, typed remember helper using JSON encoding by default.
// @group Read Through
//
// Example: remember typed value
//
//	type Profile struct { Name string `json:"name"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	profile, err := cache.Remember[Profile](c, "profile:42", time.Minute, func() (Profile, error) {
//		return Profile{Name: "Ada"}, nil
//	})
//	fmt.Println(err == nil, profile.Name) // true Ada
func Remember[T any](cache *Cache, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	return RememberCtx(context.Background(), cache, key, ttl, func(context.Context) (T, error) {
		if fn == nil {
			var zero T
			return zero, errors.New("cache remember requires a callback")
		}
		return fn()
	})
}

// RememberCtx is the context-aware variant of Remember.
// @group Read Through
func RememberCtx[T any](ctx context.Context, cache *Cache, key string, ttl time.Duration, fn func(context.Context) (T, error)) (T, error) {
	return rememberValueWithCodecCtx(ctx, cache, key, ttl, func() (T, error) {
		if fn == nil {
			var zero T
			return zero, errors.New("cache remember requires a callback")
		}
		return fn(ctx)
	}, defaultValueCodec[T]())
}

// ValueCodec defines how to encode/decode typed values for helper operations.
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

func rememberValueWithCodecCtx[T any](ctx context.Context, cache *Cache, key string, ttl time.Duration, fn func() (T, error), codec ValueCodec[T]) (T, error) {
	var zero T
	body, ok, err := cache.GetBytesCtx(ctx, key)
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
	if err := cache.SetBytesCtx(ctx, key, encoded, ttl); err != nil {
		return zero, err
	}
	return val, nil
}

func rememberStaleValueWithCodecCtx[T any](ctx context.Context, cache *Cache, key string, ttl, staleTTL time.Duration, fn func() (T, error), codec ValueCodec[T]) (T, bool, error) {
	var zero T
	body, stale, err := cache.RememberStaleBytesCtx(ctx, key, ttl, staleTTL, func(ctx context.Context) ([]byte, error) {
		if fn == nil {
			return nil, errors.New("cache remember stale requires a callback")
		}
		val, err := fn()
		if err != nil {
			return nil, err
		}
		return codec.Encode(val)
	})
	if err != nil {
		return zero, stale, err
	}
	out, err := codec.Decode(body)
	if err != nil {
		return zero, stale, err
	}
	return out, stale, nil
}

// RememberStaleCtx returns a typed value with stale fallback semantics using JSON encoding by default.
// @group Read Through
//
// Example: remember stale typed with context
//
//	type Profile struct { Name string `json:"name"` }
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	profile, usedStale, err := cache.RememberStaleCtx[Profile](ctx, c, "profile:42", time.Minute, 10*time.Minute, func(ctx context.Context) (Profile, error) {
//		return Profile{Name: "Ada"}, nil
//	})
//	fmt.Println(err == nil, usedStale, profile.Name) // true false Ada
func RememberStaleCtx[T any](ctx context.Context, cache *Cache, key string, ttl, staleTTL time.Duration, fn func(context.Context) (T, error)) (T, bool, error) {
	return rememberStaleValueWithCodecCtx(ctx, cache, key, ttl, staleTTL, func() (T, error) {
		if fn == nil {
			var zero T
			return zero, errors.New("cache remember stale requires a callback")
		}
		return fn(ctx)
	}, defaultValueCodec[T]())
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
