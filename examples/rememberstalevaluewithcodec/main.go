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
	// RememberStaleValueWithCodec allows custom encoding/decoding for typed stale remember operations.

	// Example: remember stale with custom codec
	type Profile struct { Name string }
	codec := cache.ValueCodec[Profile]{
		Encode: func(v Profile) ([]byte, error) { return []byte(v.Name), nil },
		Decode: func(b []byte) (Profile, error) { return Profile{Name: string(b)}, nil },
	}
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	profile, usedStale, err := cache.RememberStaleValueWithCodec(ctx, c, "profile:42", time.Minute, 10*time.Minute, func() (Profile, error) {
		return Profile{Name: "Ada"}, nil
	}, codec)
	fmt.Println(err == nil, usedStale, profile.Name) // true false Ada
}
