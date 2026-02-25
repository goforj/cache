//go:build integration

package all

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/cachetest"
	"github.com/goforj/cache/driver/dynamocache"
	"github.com/goforj/cache/driver/memcachedcache"
	"github.com/goforj/cache/driver/mysqlcache"
	"github.com/goforj/cache/driver/natscache"
	"github.com/goforj/cache/driver/postgrescache"
	"github.com/goforj/cache/driver/rediscache"
	"github.com/goforj/cache/driver/sqlitecache"
	"github.com/nats-io/nats.go"
	goredis "github.com/redis/go-redis/v9"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type storeFactory struct {
	name string
	new  func(t *testing.T) (cachetest.Store, func())
	opts cachetest.Options
}

func TestStoreContract_AllDrivers(t *testing.T) {
	var fixtures []storeFactory

	if integrationDriverEnabled("null") {
		fixtures = append(fixtures, storeFactory{
			name: "null",
			new: func(t *testing.T) (cachetest.Store, func()) {
				return cache.NewNullStore(context.Background()), func() {}
			},
			opts: cachetest.Options{NullSemantics: true},
		})
	}

	if integrationDriverEnabled("file") {
		fixtures = append(fixtures, storeFactory{
			name: "file",
			new: func(t *testing.T) (cachetest.Store, func()) {
				return cache.NewFileStore(context.Background(), t.TempDir()), func() {}
			},
		})
	}

	if integrationDriverEnabled("memory") {
		fixtures = append(fixtures, storeFactory{
			name: "memory",
			new: func(t *testing.T) (cachetest.Store, func()) {
				return cache.NewMemoryStore(context.Background()), func() {}
			},
		})
	}

	if integrationDriverEnabled("dynamodb") || integrationDriverEnabled("dynamocache") {
		fixtures = append(fixtures, storeFactory{
			name: "dynamocache",
			new: func(t *testing.T) (cachetest.Store, func()) {
				ctx := context.Background()
				container, endpoint := startDynamoContainer(t, ctx)
				store, err := dynamocache.New(ctx, dynamocache.Config{
					BaseConfig: cachecore.BaseConfig{Prefix: "itest", DefaultTTL: 2 * time.Second},
					Endpoint:   endpoint,
					Region:     "us-east-1",
					Table:      "cache_entries",
				})
				if err != nil {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
					t.Fatalf("create dynamo store: %v", err)
				}
				cleanup := func() {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
				}
				return store, cleanup
			},
		})
	}

	if integrationDriverEnabled("redis") || integrationDriverEnabled("rediscache") {
		fixtures = append(fixtures, storeFactory{
			name: "rediscache",
			new: func(t *testing.T) (cachetest.Store, func()) {
				ctx := context.Background()
				container, addr := startRedisContainer(t, ctx)
				client := goredis.NewClient(&goredis.Options{Addr: addr})
				store := rediscache.New(rediscache.Config{
					BaseConfig: cachecore.BaseConfig{DefaultTTL: 2 * time.Second, Prefix: "itest"},
					Client:     client,
				})
				cleanup := func() {
					_ = client.Close()
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
				}
				return store, cleanup
			},
		})
	}

	if integrationDriverEnabled("memcached") || integrationDriverEnabled("memcachedcache") {
		fixtures = append(fixtures, storeFactory{
			name: "memcachedcache",
			new: func(t *testing.T) (cachetest.Store, func()) {
				ctx := context.Background()
				container, addr := startMemcachedContainer(t, ctx)
				store := memcachedcache.New(memcachedcache.Config{
					BaseConfig: cachecore.BaseConfig{DefaultTTL: 2 * time.Second, Prefix: "itest"},
					Addresses:  []string{addr},
				})
				cleanup := func() {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
				}
				return store, cleanup
			},
			opts: cachetest.Options{
				SkipCloneCheck: true,
				TTL:            time.Second,
				TTLWait:        1500 * time.Millisecond,
			},
		})
	}

	if integrationDriverEnabled("nats") || integrationDriverEnabled("natscache") {
		fixtures = append(fixtures, storeFactory{
			name: "natscache",
			new: func(t *testing.T) (cachetest.Store, func()) {
				ctx := context.Background()
				container, addr := startNATSContainer(t, ctx)
				nc, err := nats.Connect("nats://" + addr)
				if err != nil {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
					t.Fatalf("connect nats: %v", err)
				}
				js, err := nc.JetStream()
				if err != nil {
					_ = nc.Drain()
					nc.Close()
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
					t.Fatalf("jetstream nats: %v", err)
				}
				bucket := "cache_" + strings.NewReplacer("/", "_", ":", "_").Replace(t.Name())
				kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: bucket, History: 1})
				if err != nil {
					_ = nc.Drain()
					nc.Close()
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
					t.Fatalf("create nats kv bucket: %v", err)
				}
				store := natscache.New(natscache.Config{
					BaseConfig: cachecore.BaseConfig{DefaultTTL: 2 * time.Second, Prefix: "itest"},
					KeyValue:   kv,
				})
				cleanup := func() {
					_ = js.DeleteKeyValue(bucket)
					_ = nc.Drain()
					nc.Close()
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
				}
				return store, cleanup
			},
		})
	}

	if integrationDriverEnabled("sql") || integrationDriverEnabled("sqlcore") || integrationDriverEnabled("sqlcache") || integrationDriverEnabled("sql_sqlite") || integrationDriverEnabled("sqlcache_sqlite") || integrationDriverEnabled("sqlitecache") {
		fixtures = append(fixtures, storeFactory{
			name: "sqlitecache",
			new: func(t *testing.T) (cachetest.Store, func()) {
				store, err := sqlitecache.New(sqlitecache.Config{
					BaseConfig: cachecore.BaseConfig{DefaultTTL: 2 * time.Second, Prefix: "itest"},
					DSN:        "file::memory:?cache=shared",
					Table:      "cache_entries",
				})
				if err != nil {
					t.Fatalf("create sql sqlite store: %v", err)
				}
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("sql_postgres") || integrationDriverEnabled("sqlcache_postgres") || integrationDriverEnabled("postgrescache") {
		fixtures = append(fixtures, storeFactory{
			name: "postgrescache",
			new: func(t *testing.T) (cachetest.Store, func()) {
				ctx := context.Background()
				container, addr := startPostgresContainer(t, ctx)
				dsn := "postgres://user:pass@" + addr + "/app?sslmode=disable"
				store, err := retryStoreInit(5*time.Second, 100*time.Millisecond, func() (cachetest.Store, error) {
					return postgrescache.New(postgrescache.Config{
						BaseConfig: cachecore.BaseConfig{DefaultTTL: 2 * time.Second, Prefix: "itest"},
						DSN:        dsn,
						Table:      "cache_entries",
					})
				})
				if err != nil {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
					t.Fatalf("create sql postgres store: %v", err)
				}
				cleanup := func() {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
				}
				return store, cleanup
			},
		})
	}

	if integrationDriverEnabled("sql_mysql") || integrationDriverEnabled("sqlcache_mysql") || integrationDriverEnabled("mysqlcache") {
		fixtures = append(fixtures, storeFactory{
			name: "mysqlcache",
			new: func(t *testing.T) (cachetest.Store, func()) {
				ctx := context.Background()
				container, addr := startMySQLContainer(t, ctx)
				dsn := "user:pass@tcp(" + addr + ")/app?parseTime=true"
				store, err := retryStoreInit(5*time.Second, 100*time.Millisecond, func() (cachetest.Store, error) {
					return mysqlcache.New(mysqlcache.Config{
						BaseConfig: cachecore.BaseConfig{DefaultTTL: 2 * time.Second, Prefix: "itest"},
						DSN:        dsn,
						Table:      "cache_entries",
					})
				})
				if err != nil {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
					t.Fatalf("create sql mysql store: %v", err)
				}
				cleanup := func() {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(shutdownCtx)
				}
				return store, cleanup
			},
		})
	}

	if len(fixtures) == 0 {
		t.Skip("no integration drivers selected")
	}

	for _, fx := range fixtures {
		fx := fx
		t.Run(fx.name, func(t *testing.T) {
			store, cleanup := fx.new(t)
			t.Cleanup(cleanup)

			opts := fx.opts
			opts.CaseName = t.Name()
			cachetest.RunStoreContract(t, store, opts)
		})
	}
}

func integrationDriverEnabled(name string) bool {
	return selectedIntegrationDrivers()[strings.ToLower(name)]
}

func retryStoreInit(timeout, interval time.Duration, fn func() (cachetest.Store, error)) (cachetest.Store, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		store, err := fn()
		if err == nil {
			return store, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return nil, lastErr
		}
		time.Sleep(interval)
	}
}

func selectedIntegrationDrivers() map[string]bool {
	selected := map[string]bool{
		"null":              true,
		"file":              true,
		"memory":            true,
		"dynamodb":          true,
		"dynamocache":       true,
		"redis":             true,
		"rediscache":        true,
		"memcached":         true,
		"memcachedcache":    true,
		"nats":              true,
		"natscache":         true,
		"sql":               true,
		"sqlcore":           true,
		"sqlcache":          true,
		"sqlitecache":       true,
		"sql_sqlite":        true,
		"sqlcache_sqlite":   true,
		"postgrescache":     true,
		"sql_postgres":      true,
		"sqlcache_postgres": true,
		"mysqlcache":        true,
		"sql_mysql":         true,
		"sqlcache_mysql":    true,
	}
	value := strings.TrimSpace(strings.ToLower(os.Getenv("INTEGRATION_DRIVER")))
	if value == "" || value == "all" {
		return selected
	}
	for key := range selected {
		selected[key] = false
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		selected[part] = true
	}
	return selected
}

func startRedisContainer(t *testing.T, ctx context.Context) (testcontainers.Container, string) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-bookworm",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("redis container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("redis container port: %v", err)
	}
	return container, net.JoinHostPort(host, port.Port())
}

func startDynamoContainer(t *testing.T, ctx context.Context) (testcontainers.Container, string) {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "amazon/dynamodb-local:latest",
		ExposedPorts: []string{"8000/tcp"},
		WaitingFor:   wait.ForListeningPort("8000/tcp").WithStartupTimeout(45 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start dynamodb-local container: %v", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("dynamodb-local container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "8000/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("dynamodb-local container port: %v", err)
	}
	return container, "http://" + net.JoinHostPort(host, port.Port())
}

func startMemcachedContainer(t *testing.T, ctx context.Context) (testcontainers.Container, string) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "memcached:1.6-bookworm",
		ExposedPorts: []string{"11211/tcp"},
		WaitingFor:   wait.ForListeningPort("11211/tcp").WithStartupTimeout(30 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start memcached container: %v", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("memcached container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "11211/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("memcached container port: %v", err)
	}
	return container, net.JoinHostPort(host, port.Port())
}

func startNATSContainer(t *testing.T, ctx context.Context) (testcontainers.Container, string) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "nats:2",
		Cmd:          []string{"-js"},
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor:   wait.ForLog("Server is ready").WithStartupTimeout(30 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start nats container: %v", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("nats container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "4222/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("nats container port: %v", err)
	}
	return container, net.JoinHostPort(host, port.Port())
}

func startPostgresContainer(t *testing.T, ctx context.Context) (testcontainers.Container, string) {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-bookworm",
		Env:          map[string]string{"POSTGRES_PASSWORD": "pass", "POSTGRES_USER": "user", "POSTGRES_DB": "app"},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("postgres container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("postgres container port: %v", err)
	}
	return container, net.JoinHostPort(host, port.Port())
}

func startMySQLContainer(t *testing.T, ctx context.Context) (testcontainers.Container, string) {
	t.Helper()
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
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start mysql container: %v", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("mysql container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "3306/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("mysql container port: %v", err)
	}
	return container, net.JoinHostPort(host, port.Port())
}
