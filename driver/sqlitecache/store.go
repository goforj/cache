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
