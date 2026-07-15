package main

import "github.com/goforj/cache/cachefake"

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Count returns calls for op+key.

	f := cachefake.New()
	c := f.Cache()
	_ = c.SetString("settings:mode", "dark", 0)
	n := f.Count(cachefake.OpSet, "settings:mode")
	_ = n
}
