package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// NewMemoStore decorates a backing store with a short-lived local read snapshot.
	// Create one per request or unit of work. It holds up to 256 hits or misses, each
	// for at most one second from the start of its backing read. Those reads can
	// outlive the value's backing TTL or miss a write made through another handle.
	// Use the backing store directly when you need backend TTL or immediate visibility.
	// Writes through the decorator invalidate the memoized keys. The caller retains
	// ownership of the backing store. NewMemoStore panics when the store is nil.

	// Example: memoize a backing store
	ctx := context.Background()
	base := cache.NewMemoryStore(ctx)
	memo := cache.NewMemoStore(base)
	c := cache.NewCache(memo)
	fmt.Println(c.Driver()) // memory
}
