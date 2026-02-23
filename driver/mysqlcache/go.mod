module github.com/goforj/cache/driver/mysqlcache

go 1.24.4

require (
	github.com/go-sql-driver/mysql v1.7.1
	github.com/goforj/cache/cachecore v0.0.0
	github.com/goforj/cache/driver/sqlcore v0.0.0
)

replace github.com/goforj/cache/cachecore => ../../cachecore

replace github.com/goforj/cache/driver/sqlcore => ../sqlcore
