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
	// Acquire attempts to acquire the lock once (non-blocking).

	// Example: single acquire attempt
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	lock := c.NewLockHandle("job:sync", 10*time.Second)
	locked, err := lock.Acquire()
	fmt.Println(err == nil, locked) // true true
}
