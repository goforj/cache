package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// OnCacheOp implements Observer.

	// Example: observer func callback
	obs := cache.ObserverFunc(func(ctx context.Context, event cache.CacheOpEvent) {
		fmt.Println(event.Operation, event.Key, event.Hit, event.Err == nil, event.Driver)
		_ = ctx
		_ = event.Duration
	})
	obs.OnCacheOp(context.Background(), cache.CacheOpEvent{Operation: "get", Key: "user:42", Hit: true, Duration: time.Millisecond, Driver: cachecore.DriverMemory})
}
