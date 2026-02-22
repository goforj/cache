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
	// Get acquires the lock once, runs fn if acquired, then releases automatically.

	// Example: get bytes
	ctx := context.Background()
	s := cache.NewMemoryStore(ctx)
	c := cache.NewCache(s)
	_ = c.Set("user:42", []byte("Ada"), 0)
	value, ok, _ := c.Get("user:42")
	fmt.Println(ok, string(value)) // true Ada
	// Example: acquire once and auto-release
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	lock := c.NewLockHandle("job:sync", 10*time.Second)
	locked, err := lock.Get(func() error {
		// do protected work
		return nil
	})
	fmt.Println(err == nil, locked) // true true
}
