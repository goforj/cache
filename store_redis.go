package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient captures the subset of redis.Client used by the store.
type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
}

type redisStore struct {
	client     RedisClient
	defaultTTL time.Duration
	prefix     string
}

func newRedisStore(client RedisClient, defaultTTL time.Duration, prefix string) Store {
	if defaultTTL <= 0 {
		defaultTTL = defaultCacheTTL
	}
	if prefix == "" {
		prefix = defaultCachePrefix
	}
	return &redisStore{
		client:     client,
		defaultTTL: defaultTTL,
		prefix:     prefix,
	}
}

func (s *redisStore) Driver() Driver {
	return DriverRedis
}

func (s *redisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
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

func (s *redisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s.client == nil {
		return errors.New("redis cache client unavailable")
	}
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	return s.client.Set(ctx, s.cacheKey(key), value, ttl).Err()
}

func (s *redisStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
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

func (s *redisStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
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

func (s *redisStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

func (s *redisStore) Delete(ctx context.Context, key string) error {
	if s.client == nil {
		return errors.New("redis cache client unavailable")
	}
	return s.client.Del(ctx, s.cacheKey(key)).Err()
}

func (s *redisStore) DeleteMany(ctx context.Context, keys ...string) error {
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

func (s *redisStore) Flush(ctx context.Context) error {
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

func (s *redisStore) cacheKey(key string) string {
	return s.prefix + ":" + key
}
