package cache

import (
	"context"
	"time"

	"github.com/goforj/cache/cachecore"
)

// Observer receives events for cache operations.
// It is called from Cache helpers after each operation completes.
type Observer interface {
	OnCacheOp(ctx context.Context, op string, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver)
}

// ObserverFunc adapts a function to the Observer interface.
type ObserverFunc func(ctx context.Context, op string, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver)

// OnCacheOp implements Observer.
func (f ObserverFunc) OnCacheOp(ctx context.Context, op string, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver) {
	if f == nil {
		return
	}
	f(ctx, op, key, hit, err, dur, driver)
}
