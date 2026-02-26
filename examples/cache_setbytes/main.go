package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// SetBytes writes raw bytes to key.

	// Example: set bytes with ttl
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	fmt.Println(c.SetBytes("token", []byte("abc"), time.Minute) == nil) // true
}
