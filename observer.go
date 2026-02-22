package cache

import (
	"context"
	"time"
)

// Observer receives events for cache operations.
// It is called from Cache helpers after each operation completes.
type Observer interface {
	OnCacheOp(ctx context.Context, op string, key string, hit bool, err error, dur time.Duration, driver Driver)
}

// ObserverFunc adapts a function to the Observer interface.
type ObserverFunc func(ctx context.Context, op string, key string, hit bool, err error, dur time.Duration, driver Driver)

// OnCacheOp implements Observer.
func (f ObserverFunc) OnCacheOp(ctx context.Context, op string, key string, hit bool, err error, dur time.Duration, driver Driver) {
	if f == nil {
		return
	}
	f(ctx, op, key, hit, err, dur, driver)
}
