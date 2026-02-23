module github.com/goforj/cache

go 1.24.4

require (
	github.com/goforj/cache/cachecore v0.0.0
	github.com/goforj/cache/cachetest v0.0.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
)

replace github.com/goforj/cache/cachecore => ./cachecore

replace github.com/goforj/cache/cachetest => ./cachetest

replace github.com/goforj/cache/driver/rediscache => ./driver/rediscache

replace github.com/goforj/cache/driver/memcachedcache => ./driver/memcachedcache

replace github.com/goforj/cache/driver/natscache => ./driver/natscache

replace github.com/goforj/cache/driver/sqlcore => ./driver/sqlcore

replace github.com/goforj/cache/driver/dynamocache => ./driver/dynamocache
