//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithSQL configures the SQL driver (driver name + DSN + optional table).

	// Example: sqlite via options
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverSQL,
		cache.WithSQL("sqlite", "file::memory:?cache=shared", "cache_entries"),
		cache.WithPrefix("svc"),
	)
	fmt.Println(store.Driver()) // sql
}
