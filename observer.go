package cache

import (
	"context"
	"time"

	"github.com/goforj/cache/cachecore"
)

type CacheOpEvent struct {
	Operation string
	Key       string
	Hit       bool
	Err       error
	Duration  time.Duration
	Driver    cachecore.Driver
}

// Observer receives events for cache operations.
// It is called from Cache helpers after each operation completes.
type Observer interface {
	OnCacheOp(ctx context.Context, event CacheOpEvent)
}

// ObserverFunc adapts a function to the Observer interface.
type ObserverFunc func(ctx context.Context, event CacheOpEvent)

// OnCacheOp implements Observer.
// @group Observability
//
// Example: observer func callback
//
//	obs := cache.ObserverFunc(func(ctx context.Context, event cache.CacheOpEvent) {
//		fmt.Println(event.Operation, event.Key, event.Hit, event.Err == nil, event.Driver)
//		_ = ctx
//		_ = event.Duration
//	})
//	obs.OnCacheOp(context.Background(), cache.CacheOpEvent{Operation: "get", Key: "user:42", Hit: true, Duration: time.Millisecond, Driver: cachecore.DriverMemory})
func (f ObserverFunc) OnCacheOp(ctx context.Context, event CacheOpEvent) {
	if f == nil {
		return
	}
	f(ctx, event)
}
