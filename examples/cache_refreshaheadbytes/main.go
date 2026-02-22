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
	// RefreshAheadBytes returns cached value immediately and refreshes asynchronously when near expiry.
	// On miss, it computes and stores synchronously.

	// Example: refresh ahead
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	body, err := c.RefreshAheadBytes("dashboard:summary", time.Minute, 10*time.Second, func() ([]byte, error) {
		return []byte("payload"), nil
	})
	fmt.Println(err == nil, len(body) > 0) // true true
}
