package main

import "github.com/goforj/cache/cachefake"

func main() {
	// New creates a Fake using an in-memory store.

	f := cachefake.New()
	c := f.Cache()
	_ = c.SetString("settings:mode", "dark", 0)
}
