package main

import (
	"github.com/goforj/cache/cachefake"
	"testing"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// AssertTotal ensures the total call count for an op matches times.

	f := cachefake.New()
	c := f.Cache()
	_ = c.Delete("a")
	_ = c.Delete("b")
	t := &testing.T{}
	f.AssertTotal(t, cachefake.OpDelete, 2)
}
