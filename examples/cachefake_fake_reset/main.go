package main

import "github.com/goforj/cache/cachefake"

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Reset clears recorded counts.

	f := cachefake.New()
	_ = f.Cache().SetString("settings:mode", "dark", 0)
	f.Reset()
}
