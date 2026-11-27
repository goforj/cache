package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// RememberBytes returns a cached value or computes and stores it on a miss.
	// Concurrent misses can each run the callback.

	// Example: remember bytes
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	data, err := c.RememberBytes("dashboard:summary", time.Minute, func() ([]byte, error) {
		return []byte("payload"), nil
	})
	fmt.Println(err == nil, string(data)) // true payload
}
