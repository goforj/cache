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
	// Remember returns key value or computes/stores it when missing.

	// Example: remember typed struct
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))

	type Profile struct {
		Name string `json:"name"`
	}

	profile, err := cache.Remember[Profile](c, "user:42:profile", time.Minute, func() (Profile, error) {
		return Profile{Name: "Ada"}, nil
	})
	fmt.Println(err == nil, profile.Name) // true Ada
}
