package cache

import (
	"context"
	"time"
)

// CoreAPI exposes basic cache metadata.
type CoreAPI interface {
	Driver() Driver
}

// ReadAPI exposes read-oriented cache operations.
type ReadAPI interface {
	Get(key string) ([]byte, bool, error)
	GetCtx(ctx context.Context, key string) ([]byte, bool, error)
	BatchGet(keys ...string) (map[string][]byte, error)
	BatchGetCtx(ctx context.Context, keys ...string) (map[string][]byte, error)
	GetString(key string) (string, bool, error)
	GetStringCtx(ctx context.Context, key string) (string, bool, error)
	Pull(key string) ([]byte, bool, error)
	PullCtx(ctx context.Context, key string) ([]byte, bool, error)
}

// WriteAPI exposes write and invalidation operations.
type WriteAPI interface {
	Set(key string, value []byte, ttl time.Duration) error
	SetCtx(ctx context.Context, key string, value []byte, ttl time.Duration) error
	SetString(key string, value string, ttl time.Duration) error
	SetStringCtx(ctx context.Context, key string, value string, ttl time.Duration) error
	BatchSet(values map[string][]byte, ttl time.Duration) error
	BatchSetCtx(ctx context.Context, values map[string][]byte, ttl time.Duration) error
	Add(key string, value []byte, ttl time.Duration) (bool, error)
	AddCtx(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
	Delete(key string) error
	DeleteCtx(ctx context.Context, key string) error
	DeleteMany(keys ...string) error
	DeleteManyCtx(ctx context.Context, keys ...string) error
	Flush() error
	FlushCtx(ctx context.Context) error
}

// CounterAPI exposes increment/decrement operations.
type CounterAPI interface {
	Increment(key string, delta int64, ttl time.Duration) (int64, error)
	IncrementCtx(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
	Decrement(key string, delta int64, ttl time.Duration) (int64, error)
	DecrementCtx(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
}

// RateLimitAPI exposes rate limiting helpers.
type RateLimitAPI interface {
	RateLimit(key string, limit int64, window time.Duration) (bool, int64, error)
	RateLimitCtx(ctx context.Context, key string, limit int64, window time.Duration) (bool, int64, error)
	RateLimitWithRemaining(key string, limit int64, window time.Duration) (bool, int64, int64, time.Time, error)
	RateLimitWithRemainingCtx(ctx context.Context, key string, limit int64, window time.Duration) (bool, int64, int64, time.Time, error)
}

// LockAPI exposes lock helpers based on cache keys.
type LockAPI interface {
	TryLock(key string, ttl time.Duration) (bool, error)
	TryLockCtx(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Lock(key string, ttl, timeout time.Duration) (bool, error)
	LockCtx(ctx context.Context, key string, ttl, retryInterval time.Duration) (bool, error)
	Unlock(key string) error
	UnlockCtx(ctx context.Context, key string) error
}

// RememberAPI exposes remember and stale-remember helpers.
type RememberAPI interface {
	RememberBytes(key string, ttl time.Duration, fn func() ([]byte, error)) ([]byte, error)
	RememberCtx(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error)
	RememberString(key string, ttl time.Duration, fn func() (string, error)) (string, error)
	RememberStringCtx(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) (string, error)) (string, error)
	RememberStaleBytes(key string, ttl, staleTTL time.Duration, fn func() ([]byte, error)) ([]byte, bool, error)
	RememberStaleBytesCtx(ctx context.Context, key string, ttl, staleTTL time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, bool, error)
}

// RefreshAheadAPI exposes refresh-ahead helpers.
type RefreshAheadAPI interface {
	RefreshAhead(key string, ttl, refreshAhead time.Duration, fn func() ([]byte, error)) ([]byte, error)
	RefreshAheadCtx(ctx context.Context, key string, ttl, refreshAhead time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error)
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

