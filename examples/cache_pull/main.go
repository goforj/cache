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
	// Pull reads a typed JSON value and then deletes its key. The operations are separate.

	// Example: pull typed value
	type Token struct {
		Value string `json:"value"`
	}
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.Set("preview:banner:42", Token{Value: "abc"}, time.Minute)
	tok, ok, err := c.Pull[Token]("preview:banner:42")
	fmt.Println(err == nil, ok, tok.Value) // true true abc
}
