//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/postgrescache"
)

func main() {
	// Example: postgres via explicit driver config.
	store, err := postgrescache.New(postgrescache.Config{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL: 5 * time.Minute,
			Prefix:     "app",
		},
		DSN:   "postgres://user:pass@localhost:5432/app?sslmode=disable",
		Table: "cache_entries",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // sql
}
