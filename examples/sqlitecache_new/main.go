package main

import (
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/sqlitecache"
	"time"
)

func main() {
	// New builds a sqlite-backed cachecore.Store.
	// 
	// Defaults:
	// - DefaultTTL: 5*time.Minute when zero
	// - Prefix: "app" when empty
	// - Table: "cache_entries" when empty
	// - DSN: required

	// Example: sqlite via explicit driver config
	store, err := sqlitecache.New(sqlitecache.Config{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL: 5 * time.Minute,
			Prefix:     "app",
		},
		DSN:   "file::memory:?cache=shared",
		Table: "cache_entries",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // sql
}
