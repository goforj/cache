package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
	"time"
)

func main() {
	// OnCacheOp implements Observer.

	// Example: observer func callback
	obs := cache.ObserverFunc(func(ctx context.Context, op, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver) {
		fmt.Println(op, key, hit, err == nil, driver)
		_ = ctx
		_ = dur
	})
	obs.OnCacheOp(context.Background(), "get", "user:42", true, nil, time.Millisecond, cachecore.DriverMemory)
}
