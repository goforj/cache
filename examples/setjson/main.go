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
	// SetJSON encodes value as JSON and writes it to key.

	// Example: set JSON
	type Profile struct { Name string `json:"name"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	fmt.Println(cache.SetJSON(ctx, c, "profile:42", Profile{Name: "Ada"}, time.Minute) == nil) // true
}
