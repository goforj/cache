//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Delete removes a single key.

	// Example: delete key
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.SetBytes("a", []byte("1"), time.Minute)
	fmt.Println(c.Delete("a") == nil) // true
}
