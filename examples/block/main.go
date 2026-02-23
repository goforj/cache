//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Block waits up to timeout to acquire the lock, runs fn if acquired, then releases.
	//
	// retryInterval <= 0 falls back to the cache default lock retry interval.

	// Example: wait for lock, then auto-release
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	lock := c.NewLockHandle("job:sync", 10*time.Second)
	locked, err := lock.Block(500*time.Millisecond, 25*time.Millisecond, func() error {
		// do protected work
		return nil
	})
	fmt.Println(err == nil, locked) // true true
}
