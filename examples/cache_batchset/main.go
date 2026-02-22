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
	// BatchSet writes many key/value pairs using a shared ttl.

	// Example: batch set keys
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	err := c.BatchSetBytes(map[string][]byte{
		"a": []byte("1"),
		"b": []byte("2"),
	}, time.Minute)
	fmt.Println(err == nil) // true
}
