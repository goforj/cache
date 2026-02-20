package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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

// BatchGet returns all found values for the provided keys.
// Missing keys are omitted from the returned map.
// @group Cache
//
// Example: batch get keys
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = c.Set("a", []byte("1"), time.Minute)
//	_ = c.Set("b", []byte("2"), time.Minute)
//	values, err := c.BatchGet("a", "b", "missing")
//	fmt.Println(err == nil, string(values["a"]), string(values["b"])) // true 1 2
func (c *Cache) BatchGet(keys ...string) (map[string][]byte, error) {
	return c.BatchGetCtx(context.Background(), keys...)
}

// BatchGetCtx is the context-aware variant of BatchGet.
func (c *Cache) BatchGetCtx(ctx context.Context, keys ...string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(keys))
	for _, key := range keys {
		body, ok, err := c.GetCtx(ctx, key)
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

// BatchSet writes many key/value pairs using a shared ttl.
// @group Cache
//
// Example: batch set keys
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	err := c.BatchSet(map[string][]byte{
//		"a": []byte("1"),
//		"b": []byte("2"),
//	}, time.Minute)
//	fmt.Println(err == nil) // true
func (c *Cache) BatchSet(values map[string][]byte, ttl time.Duration) error {
	return c.BatchSetCtx(context.Background(), values, ttl)
}

// BatchSetCtx is the context-aware variant of BatchSet.
func (c *Cache) BatchSetCtx(ctx context.Context, values map[string][]byte, ttl time.Duration) error {
	for key, value := range values {
		if err := c.SetCtx(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// RefreshAhead returns cached value immediately and refreshes asynchronously when near expiry.
// On miss, it computes and stores synchronously.
// @group Cache
//
// Example: refresh ahead
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	body, err := c.RefreshAhead("dashboard:summary", time.Minute, 10*time.Second, func() ([]byte, error) {
//		return []byte("payload"), nil
//	})
//	fmt.Println(err == nil, len(body) > 0) // true true
func (c *Cache) RefreshAhead(key string, ttl, refreshAhead time.Duration, fn func() ([]byte, error)) ([]byte, error) {
	return c.RefreshAheadCtx(context.Background(), key, ttl, refreshAhead, func(ctx context.Context) ([]byte, error) {
		if fn == nil {
			return nil, errors.New("cache refresh ahead requires a callback")
		}
		return fn()
	})
}

// RefreshAheadCtx is the context-aware variant of RefreshAhead.
func (c *Cache) RefreshAheadCtx(ctx context.Context, key string, ttl, refreshAhead time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	if ttl <= 0 {
		return nil, errors.New("cache refresh ahead requires ttl > 0")
	}
	if refreshAhead <= 0 {
		return nil, errors.New("cache refresh ahead requires refreshAhead > 0")
	}
	start := time.Now()
	body, ok, err := c.GetCtx(ctx, key)
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
	meta, ok, err := c.GetCtx(context.Background(), metaKey)
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
	if err := c.SetCtx(ctx, key, value, ttl); err != nil {
		return err
	}
	expiresAt := time.Now().Add(c.resolveTTL(ttl)).UnixNano()
	return c.SetCtx(ctx, key+refreshMetaSuffix, []byte(strconv.FormatInt(expiresAt, 10)), ttl)
}

// RefreshAhead returns a typed value and refreshes asynchronously when near expiry.
// @group Cache
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
// @group Other
func RefreshAheadValueWithCodec[T any](ctx context.Context, cache *Cache, key string, ttl, refreshAhead time.Duration, fn func() (T, error), codec ValueCodec[T]) (T, error) {
	var zero T
	body, err := cache.RefreshAheadCtx(ctx, key, ttl, refreshAhead, func(ctx context.Context) ([]byte, error) {
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

// RateLimit increments key in a fixed window and reports whether requests are allowed.
// @group Cache
//
// Example: fixed-window rate limit
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	allowed, count, _ := c.RateLimit("rl:login:user:42", 5, time.Minute)
//	fmt.Println(allowed, count >= 1) // true true
func (c *Cache) RateLimit(key string, limit int64, window time.Duration) (bool, int64, error) {
	return c.RateLimitCtx(context.Background(), key, limit, window)
}

// RateLimitCtx is the context-aware variant of RateLimit.
func (c *Cache) RateLimitCtx(ctx context.Context, key string, limit int64, window time.Duration) (bool, int64, error) {
	allowed, count, _, _, err := c.RateLimitWithRemainingCtx(ctx, key, limit, window)
	return allowed, count, err
}

// RateLimitWithRemaining increments a fixed-window counter and returns allowance metadata.
// @group Cache
//
// Example: rate limit headers metadata
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	allowed, count, remaining, resetAt, err := c.RateLimitWithRemaining("rl:api:ip:1.2.3.4", 100, time.Minute)
//	fmt.Println(err == nil, allowed, count >= 1, remaining >= 0, resetAt.After(time.Now()))
func (c *Cache) RateLimitWithRemaining(key string, limit int64, window time.Duration) (bool, int64, int64, time.Time, error) {
	return c.RateLimitWithRemainingCtx(context.Background(), key, limit, window)
}

// RateLimitWithRemainingCtx is the context-aware variant of RateLimitWithRemaining.
func (c *Cache) RateLimitWithRemainingCtx(ctx context.Context, key string, limit int64, window time.Duration) (bool, int64, int64, time.Time, error) {
	if limit <= 0 {
		return false, 0, 0, time.Time{}, errors.New("cache rate limit requires limit > 0")
	}
	if window <= 0 {
		return false, 0, 0, time.Time{}, errors.New("cache rate limit requires window > 0")
	}

	now := time.Now()
	bucket := now.UnixNano() / window.Nanoseconds()
	bucketKey := fmt.Sprintf("%s:%d", key, bucket)
	count, err := c.IncrementCtx(ctx, bucketKey, 1, window)
	if err != nil {
		return false, 0, 0, time.Time{}, err
	}
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	resetAt := time.Unix(0, (bucket+1)*window.Nanoseconds())
	return count <= limit, count, remaining, resetAt, nil
}

// TryLock acquires a short-lived lock key when not already held.
// @group Cache
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
// @group Cache
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
// @group Cache
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
func (c *Cache) UnlockCtx(ctx context.Context, key string) error {
	start := time.Now()
	err := c.store.Delete(ctx, lockPrefix+key)
	c.observe(ctx, "unlock", key, err == nil, err, start)
	return err
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

const staleSuffix = ":__stale"
const lockPrefix = "__lock:"
const defaultLockRetryInterval = 25 * time.Millisecond
const refreshMetaSuffix = ":__refresh_exp"
const refreshLockPrefix = "__refresh_lock:"

// RememberStaleBytes returns a fresh value when available, otherwise computes and caches it.
// If computing fails and a stale value exists, it returns the stale value.
// The returned bool is true when a stale fallback was used.
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
func (c *Cache) RememberStaleBytesCtx(ctx context.Context, key string, ttl, staleTTL time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, bool, error) {
	start := time.Now()
	staleKey := key + staleSuffix

	body, ok, err := c.GetCtx(ctx, key)
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
		if err := c.SetCtx(ctx, key, value, ttl); err != nil {
			c.observe(ctx, "remember_stale", key, false, err, start)
			return nil, false, err
		}
		if staleTTL <= 0 {
			staleTTL = ttl
		}
		if staleTTL > 0 {
			_ = c.SetCtx(ctx, staleKey, value, staleTTL)
		}
		c.observe(ctx, "remember_stale", key, true, nil, start)
		return value, false, nil
	}

	staleBody, staleOK, staleErr := c.GetCtx(ctx, staleKey)
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

// RememberStale returns a typed value with stale fallback semantics using JSON encoding by default.
// @group Cache
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

// RememberStaleValueWithCodec allows custom encoding/decoding for typed stale remember operations.
// @group Other
//
// Example: remember stale with custom codec
//
//	type Profile struct { Name string }
//	codec := cache.ValueCodec[Profile]{
//		Encode: func(v Profile) ([]byte, error) { return []byte(v.Name), nil },
//		Decode: func(b []byte) (Profile, error) { return Profile{Name: string(b)}, nil },
//	}
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	profile, usedStale, err := cache.RememberStaleValueWithCodec(ctx, c, "profile:42", time.Minute, 10*time.Minute, func() (Profile, error) {
//		return Profile{Name: "Ada"}, nil
//	}, codec)
//	fmt.Println(err == nil, usedStale, profile.Name) // true false Ada
func RememberStaleValueWithCodec[T any](ctx context.Context, cache *Cache, key string, ttl, staleTTL time.Duration, fn func() (T, error), codec ValueCodec[T]) (T, bool, error) {
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

// RememberStaleCtx returns a typed value with stale fallback semantics using JSON encoding by default.
// @group Other
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
	return RememberStaleValueWithCodec(ctx, cache, key, ttl, staleTTL, func() (T, error) {
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
