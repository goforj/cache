//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// TryLock acquires a short-lived lock key when not already held.

	// Example: try lock
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	locked, _ := c.TryLock("job:sync", 10*time.Second)
	fmt.Println(locked) // true
}
