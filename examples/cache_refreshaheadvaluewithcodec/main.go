package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// RefreshAheadValueWithCodec allows custom encoding/decoding for typed refresh-ahead operations.

	// Example: refresh ahead with a custom codec
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	codec := cache.ValueCodec[string]{
		Encode: func(value string) ([]byte, error) { return []byte(value), nil },
		Decode: func(body []byte) (string, error) { return string(body), nil },
	}
	value, err := c.RefreshAheadValueWithCodec("status", time.Minute, 10*time.Second, func() (string, error) {
		return "ready", nil
	}, codec)
	fmt.Println(err == nil, value)
	// true ready
}
