module github.com/goforj/cache/driver/postgrescache

go 1.24.4

require (
	github.com/goforj/cache/cachecore v0.0.0
	github.com/goforj/cache/driver/sqlcore v0.0.0
	github.com/jackc/pgx/v5 v5.5.4
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

replace github.com/goforj/cache/cachecore => ../../cachecore

replace github.com/goforj/cache/driver/sqlcore => ../sqlcore
