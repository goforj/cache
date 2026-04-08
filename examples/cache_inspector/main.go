package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// Inspector returns the optional browsing interface for the underlying store.

	// Example: detect inspector support
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	inspector, ok := c.Inspector()
	fmt.Println(ok, inspector.Capabilities().CanList) // true true
}
