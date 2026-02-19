//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// GetJSON decodes a JSON value into T when key exists.

	// Example: get JSON
	type Profile struct {
		Name string `json:"name"`
	}
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = cache.SetJSON(c, "profile:42", Profile{Name: "Ada"}, 0)
	profile, ok, _ := cache.GetJSON[Profile](c, "profile:42")
	fmt.Println(ok, profile.Name) // true Ada
}
