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
	// RememberString returns key value or computes/stores it when missing.

	// Example: remember string
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	val, err := c.RememberString("settings:mode", time.Minute, func() (string, error) {
		return "on", nil
	})
	fmt.Println(err == nil, val) // true on
}
