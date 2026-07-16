package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// ListPage lists cache entries from an inspector-capable store.

	// Example: browse cache keys
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.SetString("profile:1", "Ada", time.Minute)
	_ = c.SetString("profile:2", "Grace", time.Minute)
	page, _ := cache.ListPage(ctx, c, cachecore.ListPageOptions{
		Query: "profile:",
		Limit: 10,
	})
	fmt.Println(len(page.Entries), page.Entries[0].Key) // 2 profile:1
}
