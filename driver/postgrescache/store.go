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
