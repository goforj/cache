package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// GetString returns a UTF-8 string value for key when present.

	// Example: get string
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.SetString("user:42:name", "Ada", 0)
	name, ok, _ := c.GetString("user:42:name")
	fmt.Println(ok, name) // true Ada
}
