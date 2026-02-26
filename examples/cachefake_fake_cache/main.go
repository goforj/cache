package main

import "github.com/goforj/cache/cachefake"

func main() {
	// Cache returns the cache facade to inject into code under test.

	f := cachefake.New()
	c := f.Cache()
	_, _, _ = c.GetBytes("settings:mode")
}
