package cache

import "github.com/goforj/cache/cachecore"

import (
	"context"
	"time"
)

type nullStore struct{}

// newNullStore creates the intentional no-op backend used when caching is disabled.
func newNullStore() cachecore.Store { return &nullStore{} }

// Driver identifies the backend for diagnostics and capability-specific behavior.
func (s *nullStore) Driver() cachecore.Driver { return cachecore.DriverNull }

// Ready verifies that the backend can serve cache operations.
func (s *nullStore) Ready(context.Context) error { return nil }

// Get returns an owned copy of a stored value and distinguishes misses from failures.
func (s *nullStore) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, nil
}

// Set stores an owned copy of a value using the requested or default TTL.
func (s *nullStore) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

// Add stores a value only when the key is currently absent.
func (s *nullStore) Add(context.Context, string, []byte, time.Duration) (bool, error) {
	return true, nil
}

// Increment atomically adds delta while preserving the store's TTL contract.
func (s *nullStore) Increment(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, nil
}

// Decrement atomically subtracts delta while preserving the store's TTL contract.
func (s *nullStore) Decrement(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, nil
}

// Delete removes a key and treats an existing miss as success.
func (s *nullStore) Delete(context.Context, string) error { return nil }

// DeleteMany removes every requested key under the store's namespace.
func (s *nullStore) DeleteMany(context.Context, ...string) error { return nil }

// Flush removes entries within the store's configured scope.
func (s *nullStore) Flush(context.Context) error { return nil }
