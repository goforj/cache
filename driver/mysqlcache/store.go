package mysqlcache

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/sqlcore"
)

// Config configures a mysql-backed cache store.
type Config struct {
	cachecore.BaseConfig
	DSN   string
	Table string
}

// New builds a mysql-backed cachecore.Store.
func New(cfg Config) (cachecore.Store, error) {
	return sqlcore.New(sqlcore.Config{
		BaseConfig: cachecore.BaseConfig{
			Prefix:     cfg.Prefix,
			DefaultTTL: cfg.DefaultTTL,
		},
		DriverName: "mysql",
		DSN:        cfg.DSN,
		Table:      cfg.Table,
	})
}
