//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Release unlocks the key if this handle previously acquired it.
	//
	// It is safe to call multiple times; repeated calls become no-ops after the first
	// successful release.

	// Example: release a held lock
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	lock := c.NewLockHandle("job:sync", 10*time.Second)
	locked, _ := lock.Acquire()
	if locked {
		_ = lock.Release()
	}
}
