package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// GetJSON decodes a JSON value into T when key exists, using background context.

	// Example: get typed JSON
	type Profile struct { Name string `json:"name"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = cache.SetJSON(c, "profile:42", Profile{Name: "Ada"}, time.Minute)
	profile, ok, err := cache.GetJSON[Profile](c, "profile:42")
	fmt.Println(err == nil, ok, profile.Name) // true true Ada
}
