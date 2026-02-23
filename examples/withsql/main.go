//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/sqlitecache"
)

func main() {
	// Example: sqlite via explicit driver config.
	store, err := sqlitecache.New(sqlitecache.Config{
		BaseConfig: cachecore.BaseConfig{Prefix: "svc"},
		DSN:        "file::memory:?cache=shared",
		Table:      "cache_entries",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // sql
}
