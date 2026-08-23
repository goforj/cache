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
	// Get returns a typed value for key using the default codec (JSON) when present.

	// Example: get typed values (struct + string)
	type Profile struct {
		Name string `json:"name"`
	}
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.Set("profile:42", Profile{Name: "Ada"}, time.Minute)
	_ = c.Set("settings:mode", "dark", time.Minute)
	profile, ok, err := c.Get[Profile]("profile:42")
	mode, ok2, err2 := c.Get[string]("settings:mode")
	fmt.Println(err == nil, ok, profile.Name, err2 == nil, ok2, mode) // true true Ada true true dark
}
