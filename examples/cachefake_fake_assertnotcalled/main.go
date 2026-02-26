package main

import (
	"github.com/goforj/cache/cachefake"
	"testing"
)

func main() {
	// AssertNotCalled ensures key was never touched by op.

	f := cachefake.New()
	t := &testing.T{}
	f.AssertNotCalled(t, cachefake.OpDelete, "settings:mode")
}
