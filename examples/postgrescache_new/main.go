//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/postgrescache"
	"time"
)

func main() {
	// New builds a postgres-backed cachecore.Store using the pgx stdlib driver.
	// 
	// Defaults:
	// - DefaultTTL: 5*time.Minute when zero
	// - Prefix: "app" when empty
	// - Table: "cache_entries" when empty
	// - DSN: required

	// Example: postgres via explicit driver config
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
