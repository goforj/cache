package main

import "github.com/goforj/cache/cachefake"

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Cache returns the cache facade to inject into code under test.

	f := cachefake.New()
	c := f.Cache()
	_, _, _ = c.GetBytes("settings:mode")
}
