package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// RememberStale returns a typed value with stale fallback semantics using JSON encoding by default.

	// Example: remember stale typed
	type Profile struct { Name string `json:"name"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	profile, usedStale, err := cache.RememberStale[Profile](c, "profile:42", time.Minute, 10*time.Minute, func() (Profile, error) {
		return Profile{Name: "Ada"}, nil
	})
	fmt.Println(err == nil, usedStale, profile.Name) // true false Ada
}
