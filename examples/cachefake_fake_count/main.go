//go:build ignore
// +build ignore

package main

import "github.com/goforj/cache/cachefake"

func main() {
	// Count returns calls for op+key.

	f := cachefake.New()
	c := f.Cache()
	_ = c.SetString("settings:mode", "dark", 0)
	n := f.Count(cachefake.OpSet, "settings:mode")
	_ = n
}
