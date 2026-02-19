//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Pull returns value and removes it from cache.

	// Example: pull and delete
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	repo := cache.NewRepository(store)
	_ = repo.SetString(ctx, "reset:token:42", "abc", time.Minute)
	body, ok, _ := repo.Pull(ctx, "reset:token:42")
	_ = body
	_ = ok
}
