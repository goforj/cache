//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithFileDir sets the directory used by the file driver.

	// Example: set file dir
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverFile, cache.WithFileDir("/tmp/cache"))
	fmt.Println(store.Driver()) // file
}
