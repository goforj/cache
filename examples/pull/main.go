package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Pull returns a typed value for key and removes it, using the default codec (JSON).

	// Example: pull typed value
	type Token struct { Value string `json:"value"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = cache.Set(c, "reset:token:42", Token{Value: "abc"}, time.Minute)
	tok, ok, err := cache.Pull[Token](c, "reset:token:42")
	fmt.Println(err == nil, ok, tok.Value) // true true abc
}
