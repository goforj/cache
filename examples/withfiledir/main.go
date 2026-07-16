//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Example: set file dir via explicit StoreConfig.
	ctx := context.Background()
	store := cache.NewFileStoreWithConfig(ctx, cache.StoreConfig{FileDir: "/tmp/cache"})
	fmt.Println(store.Driver()) // file
}
