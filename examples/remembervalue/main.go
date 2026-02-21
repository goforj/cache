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
	// RememberValue returns a typed value or computes/stores it when missing using JSON encoding by default.

	// Example: remember typed value (codec default)
	type Summary struct { Text string `json:"text"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	s, err := cache.RememberValue[Summary](c, "dashboard:summary", time.Minute, func() (Summary, error) {
		return Summary{Text: "ok"}, nil
	})
	fmt.Println(err == nil, s.Text) // true ok
}
