package rediscache

import (
	"context"
	"errors"
	"fmt"
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
	Client Client
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
// - Client: nil allowed (operations return errors until a client is provided)
//
// Example: explicit Redis driver config
//
//	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
//	store := rediscache.New(rediscache.Config{
//		BaseConfig: cachecore.BaseConfig{
//			DefaultTTL: 5 * time.Minute,
//			Prefix:     "app",
//		},
//		Client: rdb,
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
	return &store{
		client:     cfg.Client,
		defaultTTL: ttl,
		prefix:     prefix,
	}
}

func (s *store) Driver() cachecore.Driver {
	return cachecore.DriverRedis
}

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

func (s *store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s.client == nil {
		return errors.New("redis cache client unavailable")
	}
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	return s.client.Set(ctx, s.cacheKey(key), value, ttl).Err()
}

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

func (s *store) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

func (s *store) Delete(ctx context.Context, key string) error {
	if s.client == nil {
		return errors.New("redis cache client unavailable")
	}
	return s.client.Del(ctx, s.cacheKey(key)).Err()
}

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

func (s *store) cacheKey(key string) string {
	return s.prefix + ":" + key
}
