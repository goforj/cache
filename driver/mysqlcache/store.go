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
//
// Defaults:
// - DefaultTTL: 5*time.Minute when zero
// - Prefix: "app" when empty
// - Table: "cache_entries" when empty
// - DSN: required
//
// Example: mysql via explicit driver config
//
//	store, err := mysqlcache.New(mysqlcache.Config{
//		BaseConfig: cachecore.BaseConfig{
//			DefaultTTL: 5 * time.Minute,
//			Prefix:     "app",
//		},
//		DSN:   "user:pass@tcp(127.0.0.1:3306)/app?parseTime=true",
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
		DriverName: "mysql",
		DSN:        cfg.DSN,
		Table:      cfg.Table,
	})
}
