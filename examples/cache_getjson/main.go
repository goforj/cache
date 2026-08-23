package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// GetJSON decodes a JSON value into T when key exists.

	// Example: get typed JSON
	type Profile struct {
		Name string `json:"name"`
	}
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.SetJSON("profile:42", Profile{Name: "Ada"}, time.Minute)
	profile, ok, err := c.GetJSON[Profile]("profile:42")
	fmt.Println(err == nil, ok, profile.Name) // true true Ada
}
