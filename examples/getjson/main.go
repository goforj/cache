//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// GetJSON decodes a JSON value into T when key exists.

	// Example: get JSON
	type Profile struct { Name string `json:"name"` }
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	repo := cache.NewRepository(store)
	_ = cache.SetJSON(ctx, repo, "profile:42", Profile{Name: "Ada"}, 0)
	profile, ok, _ := cache.GetJSON[Profile](ctx, repo, "profile:42")
	_ = profile
	_ = ok
}
