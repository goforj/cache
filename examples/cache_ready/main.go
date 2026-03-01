package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// Ready checks whether the underlying store is ready to serve requests.

	// Example: readiness probe
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	fmt.Println(c.Ready() == nil) // true
}
