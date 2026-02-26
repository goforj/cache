package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// RememberBytes returns key value or computes/stores it when missing.

	// Example: remember bytes
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	data, err := c.RememberBytes("dashboard:summary", time.Minute, func() ([]byte, error) {
		return []byte("payload"), nil
	})
	fmt.Println(err == nil, string(data)) // true payload
}
