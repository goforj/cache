//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// NewMemoStore decorates store with per-process read memoization.
	//
	// Behavior:
	//   - First Get hits the backing store, clones the value, and memoizes it in-process.
	//   - Subsequent Get for the same key returns the memoized clone (no backend call).
	//   - Any write/delete/flush invalidates the memo entry so local reads stay in sync
	//     with changes made through this process.
	//   - Memo data is per-process only; other processes or external writers will not
	//     invalidate it. Use only when that staleness window is acceptable.

	// Example: memoize a backing store
	ctx := context.Background()
	base := cache.NewMemoryStore(ctx)
	memoStore := cache.NewMemoStore(base)
	c := cache.NewCache(memoStore)
	_ = c
}
