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

func (e *errorStore) Driver() cachecore.Driver                          { return e.driver }
func (e *errorStore) Ready(context.Context) error                        { return e.err }
func (e *errorStore) Get(context.Context, string) ([]byte, bool, error) { return nil, false, e.err }
func (e *errorStore) Set(context.Context, string, []byte, time.Duration) error {
	return e.err
}
func (e *errorStore) Add(context.Context, string, []byte, time.Duration) (bool, error) {
	return false, e.err
}
func (e *errorStore) Increment(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, e.err
}
func (e *errorStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return e.Increment(ctx, key, -delta, ttl)
}
func (e *errorStore) Delete(context.Context, string) error        { return e.err }
func (e *errorStore) DeleteMany(context.Context, ...string) error { return e.err }
func (e *errorStore) Flush(context.Context) error                 { return e.err }
