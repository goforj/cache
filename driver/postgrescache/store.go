package postgrescache

import (
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/sqlcore"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Config configures a postgres-backed cache store.
type Config struct {
	cachecore.BaseConfig
	DSN   string
	Table string
}

// New builds a postgres-backed cachecore.Store using the pgx stdlib driver.
//
// Defaults:
// - DefaultTTL: 5*time.Minute when zero
// - Prefix: "app" when empty
// - Table: "cache_entries" when empty
// - DSN: required
//
// Example: postgres via explicit driver config
//
//	store, err := postgrescache.New(postgrescache.Config{
//		BaseConfig: cachecore.BaseConfig{
//			DefaultTTL: 5 * time.Minute,
//			Prefix:     "app",
//		},
//		DSN:   "postgres://user:pass@localhost:5432/app?sslmode=disable",
//		Table: "cache_entries",
//	})
//	if err != nil {
//		panic(err)
//	}
//	fmt.Println(store.Driver()) // sql
func New(cfg Config) (cachecore.Store, error) {
	return sqlcore.New(sqlcore.Config{
		BaseConfig: cachecore.BaseConfig{
			Prefix:     cfg.Prefix,
			DefaultTTL: cfg.DefaultTTL,
		},
		DriverName: "pgx",
		DSN:        cfg.DSN,
		Table:      cfg.Table,
	})
}
