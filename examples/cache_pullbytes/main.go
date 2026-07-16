package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// PullBytes returns value and removes it from cache.

	// Example: pull and delete
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.SetString("reset:token:42", "abc", time.Minute)
	body, ok, _ := c.PullBytes("reset:token:42")
	fmt.Println(ok, string(body)) // true abc
}
