package main

import (
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/sqlcore"
	_ "modernc.org/sqlite"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// New opens a SQL-backed cache store and initializes its schema. The caller owns
	// the store and can release its database pool through io.Closer.
	//
	// Defaults:
	// - Table: "cache_entries" when empty
	// - DefaultTTL: 5*time.Minute when zero
	// - Prefix: "app" when empty
	// - DriverName: required
	// - DSN: required

	// Example: advanced shared SQL core config
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
