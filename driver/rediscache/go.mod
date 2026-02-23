module github.com/goforj/cache/driver/rediscache

go 1.24.4

require (
	github.com/goforj/cache/cachecore v0.0.0
	github.com/goforj/cache/cachetest v0.0.0
	github.com/redis/go-redis/v9 v9.5.1
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)

replace github.com/goforj/cache/cachecore => ../../cachecore

replace github.com/goforj/cache/cachetest => ../../cachetest
