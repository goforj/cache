package main

import (
	"github.com/goforj/cache/cachefake"
	"testing"
)

func main() {
	// AssertTotal ensures the total call count for an op matches times.

	f := cachefake.New()
	c := f.Cache()
	_ = c.Delete("a")
	_ = c.Delete("b")
	t := &testing.T{}
	f.AssertTotal(t, cachefake.OpDelete, 2)
}
