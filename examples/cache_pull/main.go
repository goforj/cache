//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Pull returns a typed value for key and removes it, using the default codec (JSON).

	// Example: pull typed value
	type Token struct {
		Value string `json:"value"`
	}
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.Set("reset:token:42", Token{Value: "abc"}, time.Minute)
	tok, ok, err := c.Pull[Token]("reset:token:42")
	fmt.Println(err == nil, ok, tok.Value) // true true abc
}
