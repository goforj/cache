//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/driver/mysqlcache"
	"github.com/goforj/cache/driver/postgrescache"
	"github.com/goforj/cache/driver/sqlitecache"
)

func main() {
	// SQL dialect driver constructors.

	// Example: sqlite helper
	store, err := sqlitecache.New(sqlitecache.Config{
		DSN:   "file:cache.db?cache=shared&mode=rwc",
		Table: "cache_entries",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // sql

	// Example: postgres helper
	dsnPg := "postgres://user:pass@localhost:5432/app?sslmode=disable"
	storePg, err := postgrescache.New(postgrescache.Config{DSN: dsnPg, Table: "cache_entries"})
	if err != nil {
		panic(err)
	}
	fmt.Println(storePg.Driver()) // sql

	// Example: mysql helper
	dsnMy := "user:pass@tcp(localhost:3306)/app?parseTime=true"
	storeMy, err := mysqlcache.New(mysqlcache.Config{DSN: dsnMy, Table: "cache_entries"})
	if err != nil {
		panic(err)
	}
	fmt.Println(storeMy.Driver()) // sql
}
