package cache

import (
	"context"
	"time"

	"github.com/goforj/cache/cachecore"
)

// errorStore is returned when a driver fails to initialize; it preserves the driver
// identity while surfacing the construction error on every call.
type errorStore struct {
	driver cachecore.Driver
	err    error
}

// Driver identifies the backend for diagnostics and capability-specific behavior.
func (e *errorStore) Driver() cachecore.Driver { return e.driver }

// Ready verifies that the backend can serve cache operations.
func (e *errorStore) Ready(context.Context) error { return e.err }

// Get returns an owned copy of a stored value and distinguishes misses from failures.
func (e *errorStore) Get(context.Context, string) ([]byte, bool, error) { return nil, false, e.err }

// Set stores an owned copy of a value using the requested or default TTL.
func (e *errorStore) Set(context.Context, string, []byte, time.Duration) error {
	return e.err
}

// Add stores a value only when the key is currently absent.
func (e *errorStore) Add(context.Context, string, []byte, time.Duration) (bool, error) {
	return false, e.err
}

// Increment atomically adds delta while preserving the store's TTL contract.
func (e *errorStore) Increment(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, e.err
}

// Decrement atomically subtracts delta while preserving the store's TTL contract.
func (e *errorStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return e.Increment(ctx, key, -delta, ttl)
}

// Delete removes a key and treats an existing miss as success.
func (e *errorStore) Delete(context.Context, string) error { return e.err }

// DeleteMany removes every requested key under the store's namespace.
func (e *errorStore) DeleteMany(context.Context, ...string) error { return e.err }

// Flush removes entries within the store's configured scope.
func (e *errorStore) Flush(context.Context) error { return e.err }
