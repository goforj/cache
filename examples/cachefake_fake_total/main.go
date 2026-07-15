package main

import "github.com/goforj/cache/cachefake"

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Total returns total calls for an op across keys.

	f := cachefake.New()
	c := f.Cache()
	_ = c.Delete("a")
	_ = c.Delete("b")
	n := f.Total(cachefake.OpDelete)
	_ = n
}
