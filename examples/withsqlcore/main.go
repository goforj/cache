//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/sqlcore"
	_ "modernc.org/sqlite"
)

func main() {
	// Example: advanced shared SQL core config (sqlite shown).
	store, err := sqlcore.New(sqlcore.Config{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL: 5 * time.Minute,
			Prefix:     "app",
		},
		DriverName: "sqlite",
		DSN:        "file::memory:?cache=shared",
		Table:      "cache_entries",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // sql
}
