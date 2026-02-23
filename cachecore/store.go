package cachecore

import (
	"context"
	"time"
)

// Store is the shared app cache contract.
type Store interface {
	Driver() Driver
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
	Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
	Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
	Delete(ctx context.Context, key string) error
	DeleteMany(ctx context.Context, keys ...string) error
	Flush(ctx context.Context) error
}
