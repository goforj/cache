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
	// Remember returns key value or computes/stores it when missing.

	// Example: remember bytes
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	data, err := c.Remember(ctx, "dashboard:summary", time.Minute, func(context.Context) ([]byte, error) {
		return []byte("payload"), nil
	})
	fmt.Println(err == nil, string(data)) // true payload
}
