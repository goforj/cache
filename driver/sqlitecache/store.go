package sqlitecache

import (
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/sqlcore"
	_ "modernc.org/sqlite"
)

// Config configures a sqlite-backed cache store.
type Config struct {
	cachecore.BaseConfig
	DSN   string
	Table string
}

// New builds a sqlite-backed cachecore.Store.
//
// Defaults:
// - DefaultTTL: 5*time.Minute when zero
// - Prefix: "app" when empty
// - Table: "cache_entries" when empty
// - DSN: required
//
// Example: sqlite via explicit driver config
//
//	store, err := sqlitecache.New(sqlitecache.Config{
//		BaseConfig: cachecore.BaseConfig{
//			DefaultTTL: 5 * time.Minute,
//			Prefix:     "app",
//		},
//		DSN:   "file::memory:?cache=shared",
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
		DriverName: "sqlite",
		DSN:        cfg.DSN,
		Table:      cfg.Table,
	})
}
