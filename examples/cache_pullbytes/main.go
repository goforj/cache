package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// PullBytes reads a value and then deletes its key. The operations are separate.
	// Concurrent readers can receive the same value.

	// Example: pull and delete
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.SetString("preview:banner:42", "abc", time.Minute)
	body, ok, _ := c.PullBytes("preview:banner:42")
	fmt.Println(ok, string(body)) // true abc
}
