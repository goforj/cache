package cachecore

import (
	"context"
	"time"
)

// Store is the shared app cache contract.
type Store interface {
	// Driver identifies the concrete backend.
	Driver() Driver
	// Ready verifies that the backend can serve operations.
	Ready(ctx context.Context) error
	// Get retrieves a value and distinguishes cache misses from backend errors.
	Get(ctx context.Context, key string) ([]byte, bool, error)
	// Set writes a value with the requested TTL.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Add writes a value only when the key does not already exist.
	Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
	// Increment atomically adds delta to a numeric value when the backend supports its contract.
	Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
	// Decrement atomically subtracts delta from a numeric value when the backend supports its contract.
	Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
	// Delete removes one key and succeeds when the key is absent.
	Delete(ctx context.Context, key string) error
	// DeleteMany removes the provided keys.
	DeleteMany(ctx context.Context, keys ...string) error
	// Flush removes all keys in the store's configured scope.
	Flush(ctx context.Context) error
}
