package cache

import (
	"context"
	"time"

	"github.com/goforj/cache/cachecore"
)

// CoreAPI exposes basic cache metadata.
type CoreAPI interface {
	Driver() cachecore.Driver
	Ready() error
	ReadyContext(ctx context.Context) error
}

// ReadAPI exposes read-oriented cache operations.
type ReadAPI interface {
	GetBytes(key string) ([]byte, bool, error)
	GetBytesContext(ctx context.Context, key string) ([]byte, bool, error)
	BatchGetBytes(keys ...string) (map[string][]byte, error)
	BatchGetBytesContext(ctx context.Context, keys ...string) (map[string][]byte, error)
	GetString(key string) (string, bool, error)
	GetStringContext(ctx context.Context, key string) (string, bool, error)
	PullBytes(key string) ([]byte, bool, error)
	PullBytesContext(ctx context.Context, key string) ([]byte, bool, error)
}

// WriteAPI exposes write and invalidation operations.
type WriteAPI interface {
	SetBytes(key string, value []byte, ttl time.Duration) error
	SetBytesContext(ctx context.Context, key string, value []byte, ttl time.Duration) error
	SetString(key string, value string, ttl time.Duration) error
	SetStringContext(ctx context.Context, key string, value string, ttl time.Duration) error
	BatchSetBytes(values map[string][]byte, ttl time.Duration) error
	BatchSetBytesContext(ctx context.Context, values map[string][]byte, ttl time.Duration) error
	Add(key string, value []byte, ttl time.Duration) (bool, error)
	AddContext(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
	Delete(key string) error
	DeleteContext(ctx context.Context, key string) error
	DeleteMany(keys ...string) error
	DeleteManyContext(ctx context.Context, keys ...string) error
	Flush() error
	FlushContext(ctx context.Context) error
}

// CounterAPI exposes increment/decrement operations.
type CounterAPI interface {
	Increment(key string, delta int64, ttl time.Duration) (int64, error)
	IncrementContext(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
	Decrement(key string, delta int64, ttl time.Duration) (int64, error)
	DecrementContext(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
}

// RateLimitAPI exposes rate limiting helpers.
type RateLimitAPI interface {
	RateLimit(key string, limit int64, window time.Duration) (RateLimitStatus, error)
	RateLimitContext(ctx context.Context, key string, limit int64, window time.Duration) (RateLimitStatus, error)
}

// LockAPI exposes lock helpers based on cache keys.
type LockAPI interface {
	TryLock(key string, ttl time.Duration) (bool, error)
	TryLockContext(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Lock(key string, ttl, timeout time.Duration) (bool, error)
	LockContext(ctx context.Context, key string, ttl, retryInterval time.Duration) (bool, error)
	Unlock(key string) error
	UnlockContext(ctx context.Context, key string) error
}

// RememberAPI exposes remember and stale-remember helpers.
type RememberAPI interface {
	RememberBytes(key string, ttl time.Duration, fn func() ([]byte, error)) ([]byte, error)
	RememberBytesContext(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error)
	RememberStaleBytes(key string, ttl, staleTTL time.Duration, fn func() ([]byte, error)) ([]byte, bool, error)
	RememberStaleBytesContext(ctx context.Context, key string, ttl, staleTTL time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, bool, error)
}

// RefreshAheadAPI exposes refresh-ahead helpers.
type RefreshAheadAPI interface {
	RefreshAheadBytes(key string, ttl, refreshAhead time.Duration, fn func() ([]byte, error)) ([]byte, error)
	RefreshAheadBytesContext(ctx context.Context, key string, ttl, refreshAhead time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error)
}

// CacheAPI is the composed application-facing interface for Cache.
type CacheAPI interface {
	CoreAPI
	ReadAPI
	WriteAPI
	CounterAPI
	RateLimitAPI
	LockAPI
	RememberAPI
	RefreshAheadAPI
}

var _ CacheAPI = (*Cache)(nil)
