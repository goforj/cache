module github.com/goforj/cache/examples

go 1.27.0

require (
	github.com/goforj/cache v0.0.0
	github.com/goforj/cache/cachecore v0.4.1
	github.com/goforj/cache/driver/dynamocache v0.0.0
	github.com/goforj/cache/driver/memcachedcache v0.0.0
	github.com/goforj/cache/driver/mysqlcache v0.0.0
	github.com/goforj/cache/driver/natscache v0.0.0
	github.com/goforj/cache/driver/postgrescache v0.0.0
	github.com/goforj/cache/driver/rediscache v0.0.0
	github.com/goforj/cache/driver/sqlcore v0.4.1
	github.com/goforj/cache/driver/sqlitecache v0.0.0
	modernc.org/sqlite v1.58.0
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.46.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.33.3 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.20.3 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.19.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.67.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.9.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.37.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.42.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.49.0 // indirect
	github.com/aws/smithy-go v1.28.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-sql-driver/mysql v1.10.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.18.7 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/nats-io/nats.go v1.53.1 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/redis/go-redis/v9 v9.22.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/crypto v0.56.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	modernc.org/libc v1.75.6 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

replace github.com/goforj/cache => ./..

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
