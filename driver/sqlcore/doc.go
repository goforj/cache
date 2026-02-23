// Package sqlcore provides the shared SQL-backed cache store implementation.
// For dialect-specific registrations, prefer driver/sqlitecache, driver/postgrescache,
// or driver/mysqlcache.
//
// Example:
//
//	// Import a database/sql driver (or use a dialect wrapper package) before calling sqlcore.New.
//	store, err := sqlcore.New(sqlcore.Config{
//		DriverName: "pgx",
//		DSN:        "postgres://user:pass@localhost:5432/app?sslmode=disable",
//		Table:      "cache_entries",
//		Prefix:     "app",
//	})
//	if err != nil {
//		panic(err)
//	}
package sqlcore
