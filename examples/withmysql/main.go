//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/mysqlcache"
)

func main() {
	// Example: mysql via explicit driver config.
	store, err := mysqlcache.New(mysqlcache.Config{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL: 5 * time.Minute,
			Prefix:     "app",
		},
		DSN:   "user:pass@tcp(127.0.0.1:3306)/app?parseTime=true",
		Table: "cache_entries",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // sql
}
