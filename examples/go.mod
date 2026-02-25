module github.com/goforj/cache/examples

go 1.24.4

require (
	github.com/goforj/cache v0.0.0
	github.com/goforj/cache/cachecore v0.0.0
	github.com/goforj/cache/driver/dynamocache v0.0.0
	github.com/goforj/cache/driver/memcachedcache v0.0.0
	github.com/goforj/cache/driver/mysqlcache v0.0.0
	github.com/goforj/cache/driver/natscache v0.0.0
	github.com/goforj/cache/driver/postgrescache v0.0.0
	github.com/goforj/cache/driver/rediscache v0.0.0
	github.com/goforj/cache/driver/sqlcore v0.0.0
	github.com/goforj/cache/driver/sqlitecache v0.0.0
)

replace github.com/goforj/cache => ..
replace github.com/goforj/cache/cachecore => ../cachecore
replace github.com/goforj/cache/driver/dynamocache => ../driver/dynamocache
replace github.com/goforj/cache/driver/memcachedcache => ../driver/memcachedcache
replace github.com/goforj/cache/driver/mysqlcache => ../driver/mysqlcache
replace github.com/goforj/cache/driver/natscache => ../driver/natscache
replace github.com/goforj/cache/driver/postgrescache => ../driver/postgrescache
replace github.com/goforj/cache/driver/rediscache => ../driver/rediscache
replace github.com/goforj/cache/driver/sqlcore => ../driver/sqlcore
replace github.com/goforj/cache/driver/sqlitecache => ../driver/sqlitecache
