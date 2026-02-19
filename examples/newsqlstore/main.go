//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// NewSQLStore builds a SQL-backed store (postgres, mysql, sqlite).

	// Example: sqlite helper
	ctx := context.Background()
	store := cache.NewSQLStore(ctx, "sqlite", "file:cache.db?cache=shared&mode=rwc", "cache_entries")
	fmt.Println(store.Driver()) // sql

	// Example: postgres helper
	dsnPg := "postgres://user:pass@localhost:5432/app?sslmode=disable"
	storePg := cache.NewSQLStore(ctx, "pgx", dsnPg, "cache_entries")
	fmt.Println(storePg.Driver()) // sql

	// Example: mysql helper
	dsnMy := "user:pass@tcp(localhost:3306)/app?parseTime=true"
	storeMy := cache.NewSQLStore(ctx, "mysql", dsnMy, "cache_entries")
	fmt.Println(storeMy.Driver()) // sql
}
