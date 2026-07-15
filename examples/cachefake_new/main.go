package main

import "github.com/goforj/cache/cachefake"

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// New creates a Fake using an in-memory store.

	f := cachefake.New()
	c := f.Cache()
	_ = c.SetString("settings:mode", "dark", 0)
}
