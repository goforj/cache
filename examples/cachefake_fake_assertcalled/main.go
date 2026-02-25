//go:build ignore
// +build ignore

package main

import (
	"github.com/goforj/cache/cachefake"
	"testing"
)

func main() {
	// AssertCalled verifies key was touched by op the expected number of times.

	f := cachefake.New()
	c := f.Cache()
	_ = c.SetString("settings:mode", "dark", 0)
	t := &testing.T{}
	f.AssertCalled(t, cachefake.OpSet, "settings:mode", 1)
}
