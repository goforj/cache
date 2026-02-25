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
// @group Observability
//
// Example: observer func callback
//
//	obs := cache.ObserverFunc(func(ctx context.Context, op, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver) {
//		fmt.Println(op, key, hit, err == nil, driver)
//		_ = ctx
//		_ = dur
//	})
//	obs.OnCacheOp(context.Background(), "get", "user:42", true, nil, time.Millisecond, cachecore.DriverMemory)
func (f ObserverFunc) OnCacheOp(ctx context.Context, op string, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver) {
	if f == nil {
		return
	}
	f(ctx, op, key, hit, err, dur, driver)
}
