package main

import (
	"github.com/goforj/cache/cachefake"
	"testing"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// AssertNotCalled ensures key was never touched by op.

	f := cachefake.New()
	t := &testing.T{}
	f.AssertNotCalled(t, cachefake.OpDelete, "settings:mode")
}
