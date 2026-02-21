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
	// Remember is the ergonomic, typed remember helper using JSON encoding by default.

	// Example: remember typed value
	type Profile struct { Name string `json:"name"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	profile, err := cache.Remember[Profile](c, "profile:42", time.Minute, func() (Profile, error) {
		return Profile{Name: "Ada"}, nil
	})
	fmt.Println(err == nil, profile.Name) // true Ada
}
