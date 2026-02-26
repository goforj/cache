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

require (
	github.com/aws/aws-sdk-go-v2 v1.26.1 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.10 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.27.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.8.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.20.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.23.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.28.6 // indirect
	github.com/aws/smithy-go v1.20.2 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-sql-driver/mysql v1.7.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgx/v5 v5.5.4 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/nats-io/nats.go v1.48.0 // indirect
	github.com/nats-io/nkeys v0.4.11 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/redis/go-redis/v9 v9.5.1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	lukechampine.com/uint128 v1.2.0 // indirect
	modernc.org/cc/v3 v3.40.0 // indirect
	modernc.org/ccgo/v3 v3.16.13 // indirect
	modernc.org/libc v1.29.0 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.7.2 // indirect
	modernc.org/opt v0.1.3 // indirect
	modernc.org/sqlite v1.27.0 // indirect
	modernc.org/strutil v1.1.3 // indirect
	modernc.org/token v1.0.1 // indirect
)

replace github.com/goforj/cache => ..

replace github.com/goforj/cache/cachecore => ../cachecore

replace github.com/goforj/cache/cachetest => ../cachetest

replace github.com/goforj/cache/driver/dynamocache => ../driver/dynamocache

replace github.com/goforj/cache/driver/memcachedcache => ../driver/memcachedcache

replace github.com/goforj/cache/driver/mysqlcache => ../driver/mysqlcache

replace github.com/goforj/cache/driver/natscache => ../driver/natscache

replace github.com/goforj/cache/driver/postgrescache => ../driver/postgrescache

replace github.com/goforj/cache/driver/rediscache => ../driver/rediscache

replace github.com/goforj/cache/driver/sqlcore => ../driver/sqlcore

replace github.com/goforj/cache/driver/sqlitecache => ../driver/sqlitecache
