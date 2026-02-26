package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// RefreshAhead returns a typed value and refreshes asynchronously when near expiry.

	// Example: refresh ahead typed
	type Summary struct { Text string `json:"text"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	s, err := cache.RefreshAhead[Summary](c, "dashboard:summary", time.Minute, 10*time.Second, func() (Summary, error) {
		return Summary{Text: "ok"}, nil
	})
	fmt.Println(err == nil, s.Text) // true ok
}
