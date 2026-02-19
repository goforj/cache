//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// SetJSON encodes value as JSON and writes it to key.

	// Example: set JSON
	type Profile struct { Name string `json:"name"` }
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	repo := cache.NewRepository(store)
	_ = cache.SetJSON(ctx, repo, "profile:42", Profile{Name: "Ada"}, time.Minute)
}
