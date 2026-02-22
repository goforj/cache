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
	// BatchGet returns all found values for the provided keys.
	// Missing keys are omitted from the returned map.

	// Example: batch get keys
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.SetBytes("a", []byte("1"), time.Minute)
	_ = c.SetBytes("b", []byte("2"), time.Minute)
	values, err := c.BatchGetBytes("a", "b", "missing")
	fmt.Println(err == nil, string(values["a"]), string(values["b"])) // true 1 2
}
