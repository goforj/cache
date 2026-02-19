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
	// Pull returns value and removes it from cache.

	// Example: pull and delete
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.SetString(ctx, "reset:token:42", "abc", time.Minute)
	body, ok, _ := c.Pull(ctx, "reset:token:42")
	fmt.Println(ok, string(body)) // true abc
}
