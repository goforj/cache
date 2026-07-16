package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// NewMemoStore decorates store with per-process read memoization.
	//
	// Behavior:
	//   - First Get hits the backing store, clones the value, and memoizes it in-process.
	//   - Subsequent Get for the same key returns the memoized clone (no backend call).
	//   - Any write/delete/flush invalidates the memo entry so local reads stay in sync
	//     with changes made through this process.
	//   - Successful mutations advance one bounded process generation. This can prevent an
	//     unrelated in-flight read from being memoized, but it does not evict unrelated hits.
	//   - Memo data is per-process only; other processes or external writers will not
	//     invalidate it. Use only when that staleness window is acceptable.
	//   - NewMemoStore panics when store is nil because the backing store is required.

	// Example: memoize a backing store
	ctx := context.Background()
	base := cache.NewMemoryStore(ctx)
	memo := cache.NewMemoStore(base)
	c := cache.NewCache(memo)
	fmt.Println(c.Driver()) // memory
}
