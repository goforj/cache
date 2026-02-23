//go:build bench
// +build bench

package bench

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"io"
	"log"

	"github.com/docker/go-connections/nat"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/dynamocache"
	"github.com/goforj/cache/driver/memcachedcache"
	"github.com/goforj/cache/driver/natscache"
	"github.com/goforj/cache/driver/rediscache"
	"github.com/goforj/cache/driver/sqlcore"
	"github.com/goforj/cache/driver/sqlitecache"
	"github.com/redis/go-redis/v9"
)

type benchCase struct {
	name string
	new  func(testing.TB) (*cache.Cache, func())
}

func init() {
	// Silence testcontainers logs during benchmarks.
	testcontainers.Logger = log.New(io.Discard, "", 0)
	// Silence MySQL driver debug logs during benchmarks.
	mysql.SetLogger(log.New(io.Discard, "", 0))
}

func BenchmarkCacheSetGet(b *testing.B) {
	ctx := context.Background()
	wantedDriver := os.Getenv("BENCH_DRIVER")
	include := func(name string) bool {
		return wantedDriver == "" || wantedDriver == name
	}

	var cases []benchCase

	if include("memory") {
		cases = append(cases, benchCase{
			name: "memory",
			new: func(testing.TB) (*cache.Cache, func()) {
				return cache.NewCache(cache.NewMemoryStore(ctx)), func() {}
			},
		})
	}

	if include("file") {
		cases = append(cases, benchCase{
			name: "file",
			new: func(tb testing.TB) (*cache.Cache, func()) {
				dir := tb.TempDir()
				return cache.NewCache(cache.NewFileStore(ctx, dir)), func() {}
			},
		})
	}

	// Redis
	if include("redis") {
		if addr := os.Getenv("REDIS_ADDR"); addr != "" {
			cases = append(cases, redisCase(ctx, addr))
		} else if c, cleanup, err := startRedis(ctx); err == nil {
			cases = append(cases, benchCase{name: "redis", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
		} else if wantedDriver == "redis" {
			b.Fatalf("redis benchmark setup failed: %v", err)
		}
	}

	// Memcached
	if include("memcached") {
		if addr := os.Getenv("MEMCACHED_ADDR"); addr != "" {
			cases = append(cases, memcachedCase(ctx, addr))
		} else if c, cleanup, err := startMemcached(ctx); err == nil {
			cases = append(cases, benchCase{name: "memcached", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
		} else if wantedDriver == "memcached" {
			b.Fatalf("memcached benchmark setup failed: %v", err)
		}
	}

	// NATS (JetStream KV)
	if include("nats") || include("nats_bucket_ttl") {
		if url := os.Getenv("NATS_URL"); url != "" {
			if include("nats") {
				cases = append(cases, natsCase(ctx, url))
			}
			if include("nats_bucket_ttl") {
				cases = append(cases, natsBucketTTLCase(ctx, url))
			}
		} else {
			if include("nats") {
				if c, cleanup, err := startNATS(ctx); err == nil {
					cases = append(cases, benchCase{name: "nats", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
				} else if wantedDriver == "nats" {
					b.Fatalf("nats benchmark setup failed: %v", err)
				}
			}
			if include("nats_bucket_ttl") {
				if c2, cleanup2, err2 := startNATS(ctx, benchWithNATSBucketTTL(true)); err2 == nil {
					cases = append(cases, benchCase{name: "nats_bucket_ttl", new: func(testing.TB) (*cache.Cache, func()) { return c2, cleanup2 }})
				} else if wantedDriver == "nats_bucket_ttl" {
					b.Fatalf("nats_bucket_ttl benchmark setup failed: %v", err2)
				}
			}
		}
	}

	// DynamoDB
	if include("dynamodb") {
		if endpoint := os.Getenv("DYNAMO_ENDPOINT"); endpoint != "" {
			cases = append(cases, dynamoCase(ctx, endpoint))
		} else if c, cleanup, err := startDynamo(ctx); err == nil {
			cases = append(cases, benchCase{name: "dynamodb", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
		} else if wantedDriver == "dynamodb" {
			b.Fatalf("dynamodb benchmark setup failed: %v", err)
		}
	}

	// SQL: Postgres and MySQL
	if include("sql_postgres") {
		if dsn := os.Getenv("BENCH_PG_DSN"); dsn != "" {
			cases = append(cases, postgresCase(ctx, dsn))
		} else if c, cleanup, err := startPostgres(ctx); err == nil {
			cases = append(cases, benchCase{name: "sql_postgres", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
		} else if wantedDriver == "sql_postgres" {
			b.Fatalf("sql_postgres benchmark setup failed: %v", err)
		}
	}

	if include("sql_mysql") {
		if dsn := os.Getenv("BENCH_MYSQL_DSN"); dsn != "" {
			cases = append(cases, mysqlCase(ctx, dsn))
		} else if c, cleanup, err := startMySQL(ctx); err == nil {
			cases = append(cases, benchCase{name: "sql_mysql", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
		} else if wantedDriver == "sql_mysql" {
			b.Fatalf("sql_mysql benchmark setup failed: %v", err)
		}
	}

	// SQLite in-memory is always available.
	if include("sql_sqlite") {
		cases = append(cases, benchCase{
			name: "sql_sqlite",
			new: func(tb testing.TB) (*cache.Cache, func()) {
				dsn := "file:" + filepath.Join(tb.TempDir(), "bench.sqlite") + "?cache=shared&mode=rwc"
				store, err := newSQLBenchStore("sqlite", dsn, benchWithPrefix("bench"))
				if err != nil {
					tb.Fatalf("sqlite benchmark setup failed: %v", err)
				}
				return cache.NewCache(store), func() {}
			},
		})
	}

	if len(cases) == 0 {
		b.Fatalf("no benchmark cases selected; BENCH_DRIVER=%q", wantedDriver)
	}

	for _, bc := range cases {
		bc := bc
		b.Run(bc.name, func(b *testing.B) {
			c, cleanup := bc.new(b)
			if cleanup != nil {
				defer cleanup()
			}
			benchmarkSetGet(b, c)
		})
	}
}

func benchmarkSetGet(b *testing.B, c *cache.Cache) {
	b.Helper()
	type profile struct {
		Name  string `json:"name"`
		Level int    `json:"level"`
	}

	cases := []struct {
		name  string
		setup func()
		run   func()
	}{
		{
			name: "set_get_bytes",
			setup: func() {
				_ = c.SetBytes("bench:key", []byte("value"), time.Minute)
			},
			run: func() {
				_ = c.SetBytes("bench:key", []byte("value"), time.Minute)
				_, _, _ = c.GetBytes("bench:key")
			},
		},
		{
			name: "set_get_typed_string",
			setup: func() {
				_ = cache.Set(c, "bench:key", "value", time.Minute)
			},
			run: func() {
				_ = cache.Set(c, "bench:key", "value", time.Minute)
				_, _, _ = cache.Get[string](c, "bench:key")
			},
		},
		{
			name: "set_get_typed_struct",
			setup: func() {
				_ = cache.Set(c, "bench:key", profile{Name: "Ada", Level: 7}, time.Minute)
			},
			run: func() {
				_ = cache.Set(c, "bench:key", profile{Name: "Ada", Level: 7}, time.Minute)
				_, _, _ = cache.Get[profile](c, "bench:key")
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			tc.setup()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tc.run()
			}
		})
	}
}

// --- case helpers ----

func redisCase(ctx context.Context, addr string) benchCase {
	return benchCase{
		name: "redis",
		new: func(testing.TB) (*cache.Cache, func()) {
			client := redis.NewClient(&redis.Options{Addr: addr})
			store := rediscache.New(rediscache.Config{Client: client})
			return cache.NewCache(store), func() { _ = client.Close() }
		},
	}
}

func memcachedCase(ctx context.Context, addr string) benchCase {
	return benchCase{
		name: "memcached",
		new: func(testing.TB) (*cache.Cache, func()) {
			store := memcachedcache.New(memcachedcache.Config{Addresses: []string{addr}})
			return cache.NewCache(store), func() {}
		},
	}
}

func dynamoCase(ctx context.Context, endpoint string) benchCase {
	return benchCase{
		name: "dynamodb",
		new: func(tb testing.TB) (*cache.Cache, func()) {
			store, err := newDynamoBenchStore(ctx, endpoint)
			if err != nil {
				tb.Fatalf("dynamo benchmark setup failed: %v", err)
			}
			return cache.NewCache(store), func() {}
		},
	}
}

func natsCase(ctx context.Context, natsURL string) benchCase {
	return benchCase{
		name: "nats",
		new: func(tb testing.TB) (*cache.Cache, func()) {
			c, cleanup, err := natsCacheForURL(ctx, natsURL)
			if err != nil {
				tb.Fatalf("nats benchmark setup failed: %v", err)
			}
			return c, cleanup
		},
	}
}

func natsBucketTTLCase(ctx context.Context, natsURL string) benchCase {
	return benchCase{
		name: "nats_bucket_ttl",
		new: func(tb testing.TB) (*cache.Cache, func()) {
			c, cleanup, err := natsCacheForURL(ctx, natsURL, benchWithNATSBucketTTL(true))
			if err != nil {
				tb.Fatalf("nats bucket ttl benchmark setup failed: %v", err)
			}
			return c, cleanup
		},
	}
}

func postgresCase(ctx context.Context, dsn string) benchCase {
	return benchCase{
		name: "sql_postgres",
		new: func(tb testing.TB) (*cache.Cache, func()) {
			store, err := newSQLBenchStore("pgx", dsn, benchWithPrefix("bench"))
			if err != nil {
				tb.Fatalf("postgres benchmark setup failed: %v", err)
			}
			return cache.NewCache(store), func() {}
		},
	}
}

func mysqlCase(ctx context.Context, dsn string) benchCase {
	return benchCase{
		name: "sql_mysql",
		new: func(tb testing.TB) (*cache.Cache, func()) {
			store, err := newSQLBenchStore("mysql", dsn, benchWithPrefix("bench"))
			if err != nil {
				tb.Fatalf("mysql benchmark setup failed: %v", err)
			}
			return cache.NewCache(store), func() {}
		},
	}
}

// --- testcontainers fallbacks (best effort) ---

func startRedis(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-bookworm",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}
	c, addr, err := startContainer(ctx, req, "6379/tcp")
	if err != nil {
		return nil, nil, err
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	store := rediscache.New(rediscache.Config{Client: client})
	cleanup := func() {
		_ = client.Close()
		_ = c.Terminate(context.Background())
	}
	return cache.NewCache(store), cleanup, nil
}

func startMemcached(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "memcached:1.6-bookworm",
		ExposedPorts: []string{"11211/tcp"},
		WaitingFor:   wait.ForListeningPort("11211/tcp").WithStartupTimeout(30 * time.Second),
	}
	c, addr, err := startContainer(ctx, req, "11211/tcp")
	if err != nil {
		return nil, nil, err
	}
	store := memcachedcache.New(memcachedcache.Config{Addresses: []string{addr}})
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startDynamo(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "amazon/dynamodb-local:latest",
		ExposedPorts: []string{"8000/tcp"},
		WaitingFor:   wait.ForListeningPort("8000/tcp").WithStartupTimeout(45 * time.Second),
	}
	c, addr, err := startContainer(ctx, req, "8000/tcp")
	if err != nil {
		return nil, nil, err
	}
	endpoint := "http://" + addr
	store, err := newDynamoBenchStore(ctx, endpoint)
	if err != nil {
		_ = c.Terminate(context.Background())
		return nil, nil, err
	}
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func newDynamoBenchStore(ctx context.Context, endpoint string, opts ...benchStoreOption) (cachecore.Store, error) {
	cfg := dynamocache.Config{
		BaseConfig: cachecore.BaseConfig{DefaultTTL: time.Minute},
		Endpoint:   endpoint,
		Region:     "us-east-1",
		Table:      "cache_entries",
	}
	for _, opt := range opts {
		var benchCfg benchConfig
		benchCfg = opt(benchCfg)
		if benchCfg.DefaultTTL > 0 {
			cfg.DefaultTTL = benchCfg.DefaultTTL
		}
		if benchCfg.Prefix != "" {
			cfg.Prefix = benchCfg.Prefix
		}
		if benchCfg.DynamoClient != nil {
			if client, ok := benchCfg.DynamoClient.(dynamocache.DynamoAPI); ok {
				cfg.Client = client
			}
		}
		if benchCfg.DynamoEndpoint != "" {
			cfg.Endpoint = benchCfg.DynamoEndpoint
		}
		if benchCfg.DynamoRegion != "" {
			cfg.Region = benchCfg.DynamoRegion
		}
		if benchCfg.DynamoTable != "" {
			cfg.Table = benchCfg.DynamoTable
		}
	}
	return dynamocache.New(ctx, cfg)
}

func startNATS(ctx context.Context, opts ...benchStoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "nats:2",
		Cmd:          []string{"-js"},
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor:   wait.ForLog("Server is ready").WithStartupTimeout(30 * time.Second),
	}
	c, addr, err := startContainer(ctx, req, "4222/tcp")
	if err != nil {
		return nil, nil, err
	}
	cacheValue, cleanup, err := natsCacheForURL(ctx, "nats://"+addr, opts...)
	if err != nil {
		_ = c.Terminate(context.Background())
		return nil, nil, err
	}
	return cacheValue, func() {
		cleanup()
		_ = c.Terminate(context.Background())
	}, nil
}

func startPostgres(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-bookworm",
		Env:          map[string]string{"POSTGRES_PASSWORD": "pass", "POSTGRES_USER": "user", "POSTGRES_DB": "app"},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}
	c, addr, err := startContainer(ctx, req, "5432/tcp")
	if err != nil {
		return nil, nil, err
	}
	dsn := "postgres://user:pass@" + addr + "/app?sslmode=disable"
	store, err := newSQLBenchStore("pgx", dsn, benchWithPrefix("bench"))
	if err != nil {
		_ = c.Terminate(context.Background())
		return nil, nil, err
	}
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startMySQL(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image: "mysql:8",
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "pass",
			"MYSQL_DATABASE":      "app",
			"MYSQL_USER":          "user",
			"MYSQL_PASSWORD":      "pass",
		},
		ExposedPorts: []string{"3306/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("3306/tcp").WithStartupTimeout(90*time.Second),
			wait.ForLog("ready for connections").WithOccurrence(2).WithStartupTimeout(90*time.Second),
		),
	}
	c, addr, err := startContainer(ctx, req, "3306/tcp")
	if err != nil {
		return nil, nil, err
	}
	dsn := "user:pass@tcp(" + addr + ")/app?parseTime=true"
	store, err := newSQLBenchStore("mysql", dsn, benchWithPrefix("bench"))
	if err != nil {
		_ = c.Terminate(context.Background())
		return nil, nil, err
	}
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func newSQLBenchStore(driverName, dsn string, opts ...benchStoreOption) (cachecore.Store, error) {
	cfg := sqlcore.Config{
		DriverName: driverName,
		DSN:        dsn,
		Table:      "cache_entries",
	}
	for _, opt := range opts {
		var benchCfg benchConfig
		benchCfg = opt(benchCfg)
		if benchCfg.DefaultTTL > 0 {
			cfg.DefaultTTL = benchCfg.DefaultTTL
		}
		if benchCfg.Prefix != "" {
			cfg.Prefix = benchCfg.Prefix
		}
	}
	if driverName == "sqlite" {
		return sqlitecache.New(sqlitecache.Config{
			BaseConfig: cachecore.BaseConfig{Prefix: cfg.Prefix, DefaultTTL: cfg.DefaultTTL},
			DSN:        cfg.DSN,
			Table:      cfg.Table,
		})
	}
	store, err := sqlcore.New(cfg)
	if err != nil {
		return nil, err
	}
	return store, nil
}

func startContainer(ctx context.Context, req testcontainers.ContainerRequest, port string) (testcontainers.Container, string, error) {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}
	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, "", err
	}
	mapped, err := c.MappedPort(ctx, nat.Port(port))
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, "", err
	}
	return c, host + ":" + mapped.Port(), nil
}

func natsCacheForURL(ctx context.Context, natsURL string, opts ...benchStoreOption) (*cache.Cache, func(), error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	bucket := "BENCH_" + strconv.FormatInt(time.Now().UnixNano()%1_000_000_000, 10)
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: bucket, History: 1})
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	cfg := natscache.Config{KeyValue: kv}
	for _, opt := range opts {
		var benchCfg benchConfig
		benchCfg = opt(benchCfg)
		if benchCfg.DefaultTTL > 0 {
			cfg.DefaultTTL = benchCfg.DefaultTTL
		}
		if benchCfg.Prefix != "" {
			cfg.Prefix = benchCfg.Prefix
		}
		if benchCfg.NATSBucketTTL {
			cfg.BucketTTL = true
		}
	}
	store := natscache.New(cfg)
	cleanup := func() {
		_ = js.DeleteKeyValue(bucket)
		_ = nc.Drain()
		nc.Close()
	}
	return cache.NewCache(store), cleanup, nil
}
