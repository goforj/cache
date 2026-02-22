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
	// Get returns a typed value for key using the default codec (JSON) when present.

	// Example: get typed values (struct + string)
	type Profile struct { Name string `json:"name"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = cache.Set(c, "profile:42", Profile{Name: "Ada"}, time.Minute)
	_ = cache.Set(c, "settings:mode", "dark", time.Minute)
	profile, ok, err := cache.Get[Profile](c, "profile:42")
	mode, ok2, err2 := cache.Get[string](c, "settings:mode")
	fmt.Println(err == nil, ok, profile.Name, err2 == nil, ok2, mode) // true true Ada true true dark
}
