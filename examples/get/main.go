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

	// Example: get typed value
	type Profile struct { Name string `json:"name"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = cache.Set(c, "profile:42", Profile{Name: "Ada"}, time.Minute)
	profile, ok, err := cache.Get[Profile](c, "profile:42")
	fmt.Println(err == nil, ok, profile.Name) // true true Ada
}
