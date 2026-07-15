package rediscache

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/redis/go-redis/v9"
)

const (
	defaultTTL    = 5 * time.Minute
	defaultPrefix = "app"
)

// Client captures the subset of redis.Client used by the store.
type Client interface {
	Ping(ctx context.Context) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
}

// Config configures a Redis-backed cache store.
type Config struct {
	cachecore.BaseConfig
	Addr      string
	Username  string
	Password  string
	DB        int
	TLSConfig *tls.Config
	Client    Client
}

type store struct {
	client     Client
	defaultTTL time.Duration
	prefix     string
}

// New builds a Redis-backed cachecore.Store.
//
// Defaults:
// - DefaultTTL: 5*time.Minute when zero
// - Prefix: "app" when empty
// - Addr: empty by default (no client auto-created unless Addr is set)
// - Client: optional advanced override (takes precedence when set)
// - If neither Client nor Addr is set, operations return errors until a client is provided
//
// Example: explicit Redis driver config
//
//	store := rediscache.New(rediscache.Config{
//		BaseConfig: cachecore.BaseConfig{
//			DefaultTTL: 5 * time.Minute,
//			Prefix:     "app",
//		},
//		Addr: "127.0.0.1:6379",
//	})
//	fmt.Println(store.Driver()) // redis
func New(cfg Config) cachecore.Store {
	ttl := cfg.DefaultTTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = defaultPrefix
	}
	client := cfg.Client
	if client == nil && cfg.Addr != "" {
		client = redis.NewClient(&redis.Options{
			Addr:      cfg.Addr,
			Username:  cfg.Username,
			Password:  cfg.Password,
			DB:        cfg.DB,
			TLSConfig: cfg.TLSConfig,
		})
	}
	backend := &store{
		client:     client,
		defaultTTL: ttl,
		prefix:     prefix,
	}
	wrapped, _ := cachecore.WrapStore(backend, cfg.BaseConfig)
	return wrapped
}

// Driver identifies the backend for diagnostics and capability-specific behavior.
func (s *store) Driver() cachecore.Driver {
	return cachecore.DriverRedis
}

// Ready verifies that the backend can serve cache operations.
func (s *store) Ready(ctx context.Context) error {
	if s.client == nil {
		return errors.New("redis cache client unavailable")
	}
	return s.client.Ping(ctx).Err()
}

// Get returns an owned copy of a stored value and distinguishes misses from failures.
func (s *store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if s.client == nil {
		return nil, false, errors.New("redis cache client unavailable")
	}
	value, err := s.client.Get(ctx, s.cacheKey(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return value, true, nil
}

// Set stores an owned copy of a value using the requested or default TTL.
func (s *store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s.client == nil {
		return errors.New("redis cache client unavailable")
	}
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	return s.client.Set(ctx, s.cacheKey(key), value, ttl).Err()
}

// Add stores a value only when the key is currently absent.
func (s *store) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if s.client == nil {
		return false, errors.New("redis cache client unavailable")
	}
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	created, err := s.client.SetNX(ctx, s.cacheKey(key), value, ttl).Result()
	if err != nil {
		return false, err
	}
	return created, nil
}

// Increment atomically adds delta while preserving the store's TTL contract.
func (s *store) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if s.client == nil {
		return 0, errors.New("redis cache client unavailable")
	}
	cacheKey := s.cacheKey(key)
	value, err := s.client.IncrBy(ctx, cacheKey, delta).Result()
	if err != nil {
		return 0, err
	}
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	if ttl > 0 {
		if expireErr := s.client.Expire(ctx, cacheKey, ttl).Err(); expireErr != nil {
			return 0, fmt.Errorf("expire cache key: %w", expireErr)
		}
	}
	return value, nil
}

// Decrement atomically subtracts delta while preserving the store's TTL contract.
func (s *store) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

// Delete removes a key and treats an existing miss as success.
func (s *store) Delete(ctx context.Context, key string) error {
	if s.client == nil {
		return errors.New("redis cache client unavailable")
	}
	return s.client.Del(ctx, s.cacheKey(key)).Err()
}

// DeleteMany removes every requested key under the store's namespace.
func (s *store) DeleteMany(ctx context.Context, keys ...string) error {
	if s.client == nil {
		return errors.New("redis cache client unavailable")
	}
	if len(keys) == 0 {
		return nil
	}
	cacheKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		cacheKeys = append(cacheKeys, s.cacheKey(key))
	}
	return s.client.Del(ctx, cacheKeys...).Err()
}

// Flush removes entries within the store's configured scope.
func (s *store) Flush(ctx context.Context) error {
	if s.client == nil {
		return errors.New("redis cache client unavailable")
	}
	pattern := s.cacheKey("*")
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := s.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

// cacheKey applies the configured namespace before a key reaches the backend.
func (s *store) cacheKey(key string) string {
	return s.prefix + ":" + key
}

// Capabilities reports the optional inspection operations supported by the store.
func (s *store) Capabilities() cachecore.InspectorCapabilities {
	return cachecore.InspectorCapabilities{
		CanList:   true,
		CanRead:   true,
		CanDelete: true,
	}
}

// ListPage returns a filtered, deterministic page of inspectable cache entries.
func (s *store) ListPage(ctx context.Context, opts cachecore.ListPageOptions) (cachecore.ListPageResult, error) {
	if s.client == nil {
		return cachecore.ListPageResult{}, errors.New("redis cache client unavailable")
	}
	filter := cachecore.ListFilterTerm(opts)
	pattern := s.cacheKey("*")
	var (
		cursor  uint64
		keys    []string
		scanned []string
		err     error
	)
	for {
		scanned, cursor, err = s.client.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return cachecore.ListPageResult{}, err
		}
		keys = append(keys, scanned...)
		if cursor == 0 {
			break
		}
	}
	entries := make([]cachecore.CacheEntry, 0, len(keys))
	cachePrefix := s.prefix + ":"
	for _, fullKey := range keys {
		key := strings.TrimPrefix(fullKey, cachePrefix)
		body, ok, err := s.Get(ctx, key)
		if err != nil || !ok {
			continue
		}
		entries = append(entries, cachecore.CacheEntry{
			Key:       key,
			SizeBytes: len(body),
		})
	}
	entries = cachecore.FilterAndSortEntries(entries, filter)
	offset, err := cachecore.DecodeOffsetCursor(opts.Cursor)
	if err != nil {
		return cachecore.ListPageResult{}, err
	}
	return cachecore.SliceEntries(entries, offset, opts.Limit), nil
}
