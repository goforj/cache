//go:build bench
// +build bench

package bench

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"io"
	"log"

	"github.com/docker/go-connections/nat"
	mysql "github.com/go-sql-driver/mysql"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/goforj/cache"
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

	var cases []benchCase

	cases = append(cases, benchCase{
		name: "memory",
		new: func(testing.TB) (*cache.Cache, func()) {
			return cache.NewCache(cache.NewMemoryStore(ctx)), func() {}
		},
	})

	cases = append(cases, benchCase{
		name: "file",
		new: func(tb testing.TB) (*cache.Cache, func()) {
			dir := tb.TempDir()
			return cache.NewCache(cache.NewFileStore(ctx, dir)), func() {}
		},
	})

	// Redis
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		cases = append(cases, redisCase(ctx, addr))
	} else if c, cleanup, err := startRedis(ctx); err == nil {
		cases = append(cases, benchCase{name: "redis", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
	}

	// Memcached
	if addr := os.Getenv("MEMCACHED_ADDR"); addr != "" {
		cases = append(cases, memcachedCase(ctx, addr))
	} else if c, cleanup, err := startMemcached(ctx); err == nil {
		cases = append(cases, benchCase{name: "memcached", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
	}

	// DynamoDB
	if endpoint := os.Getenv("DYNAMO_ENDPOINT"); endpoint != "" {
		cases = append(cases, dynamoCase(ctx, endpoint))
	} else if c, cleanup, err := startDynamo(ctx); err == nil {
		cases = append(cases, benchCase{name: "dynamodb", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
	}

	// SQL: Postgres and MySQL
	if dsn := os.Getenv("BENCH_PG_DSN"); dsn != "" {
		cases = append(cases, postgresCase(ctx, dsn))
	} else if c, cleanup, err := startPostgres(ctx); err == nil {
		cases = append(cases, benchCase{name: "sql_postgres", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
	}

	if dsn := os.Getenv("BENCH_MYSQL_DSN"); dsn != "" {
		cases = append(cases, mysqlCase(ctx, dsn))
	} else if c, cleanup, err := startMySQL(ctx); err == nil {
		cases = append(cases, benchCase{name: "sql_mysql", new: func(testing.TB) (*cache.Cache, func()) { return c, cleanup }})
	}

	// SQLite in-memory is always available.
	cases = append(cases, benchCase{
		name: "sql_sqlite",
		new: func(tb testing.TB) (*cache.Cache, func()) {
			dsn := "file:" + filepath.Join(tb.TempDir(), "bench.sqlite") + "?cache=shared&mode=rwc"
			store := cache.NewSQLStore(ctx, "sqlite", dsn, "cache_entries", cache.WithPrefix("bench"))
			return cache.NewCache(store), func() {}
		},
	})

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
	b.ReportAllocs()

	key := "bench:key"
	val := []byte("value")

	_ = c.Set(key, val, time.Minute)
	_, _, _ = c.Get(key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Set(key, val, time.Minute)
		_, _, _ = c.Get(key)
	}
}

// --- case helpers ----

func redisCase(ctx context.Context, addr string) benchCase {
	return benchCase{
		name: "redis",
		new: func(testing.TB) (*cache.Cache, func()) {
			client := redis.NewClient(&redis.Options{Addr: addr})
			store := cache.NewRedisStore(ctx, client)
			return cache.NewCache(store), func() { _ = client.Close() }
		},
	}
}

func memcachedCase(ctx context.Context, addr string) benchCase {
	return benchCase{
		name: "memcached",
		new: func(testing.TB) (*cache.Cache, func()) {
			store := cache.NewMemcachedStore(ctx, []string{addr})
			return cache.NewCache(store), func() {}
		},
	}
}

func dynamoCase(ctx context.Context, endpoint string) benchCase {
	return benchCase{
		name: "dynamodb",
		new: func(testing.TB) (*cache.Cache, func()) {
			store := cache.NewStore(ctx, cache.StoreConfig{
				Driver:         cache.DriverDynamo,
				DynamoEndpoint: endpoint,
				DynamoTable:    "cache_entries",
				DynamoRegion:   "us-east-1",
				DefaultTTL:     time.Minute,
			})
			return cache.NewCache(store), func() {}
		},
	}
}

func postgresCase(ctx context.Context, dsn string) benchCase {
	return benchCase{
		name: "sql_postgres",
		new: func(testing.TB) (*cache.Cache, func()) {
			store := cache.NewSQLStore(ctx, "pgx", dsn, "cache_entries", cache.WithPrefix("bench"))
			return cache.NewCache(store), func() {}
		},
	}
}

func mysqlCase(ctx context.Context, dsn string) benchCase {
	return benchCase{
		name: "sql_mysql",
		new: func(testing.TB) (*cache.Cache, func()) {
			store := cache.NewSQLStore(ctx, "mysql", dsn, "cache_entries", cache.WithPrefix("bench"))
			return cache.NewCache(store), func() {}
		},
	}
}

// --- testcontainers fallbacks (best effort) ---

func startRedis(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}
	c, addr, err := startContainer(ctx, req, "6379/tcp")
	if err != nil {
		return nil, nil, err
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	store := cache.NewRedisStore(ctx, client)
	cleanup := func() {
		_ = client.Close()
		_ = c.Terminate(context.Background())
	}
	return cache.NewCache(store), cleanup, nil
}

func startMemcached(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "memcached:alpine",
		ExposedPorts: []string{"11211/tcp"},
		WaitingFor:   wait.ForListeningPort("11211/tcp").WithStartupTimeout(30 * time.Second),
	}
	c, addr, err := startContainer(ctx, req, "11211/tcp")
	if err != nil {
		return nil, nil, err
	}
	store := cache.NewMemcachedStore(ctx, []string{addr})
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
	store := cache.NewStore(ctx, cache.StoreConfig{
		Driver:         cache.DriverDynamo,
		DynamoEndpoint: endpoint,
		DynamoTable:    "cache_entries",
		DynamoRegion:   "us-east-1",
		DefaultTTL:     time.Minute,
	})
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startPostgres(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		Env:          map[string]string{"POSTGRES_PASSWORD": "pass", "POSTGRES_USER": "user", "POSTGRES_DB": "app"},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}
	c, addr, err := startContainer(ctx, req, "5432/tcp")
	if err != nil {
		return nil, nil, err
	}
	dsn := "postgres://user:pass@" + addr + "/app?sslmode=disable"
	store := cache.NewSQLStore(ctx, "pgx", dsn, "cache_entries", cache.WithPrefix("bench"))
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
	store := cache.NewSQLStore(ctx, "mysql", dsn, "cache_entries", cache.WithPrefix("bench"))
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
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
