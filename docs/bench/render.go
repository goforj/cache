//go:build benchrender
// +build benchrender

package bench

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/nats-io/nats.go"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/goforj/cache"
	"github.com/redis/go-redis/v9"
)

const (
	benchStart = "<!-- bench:embed:start -->"
	benchEnd   = "<!-- bench:embed:end -->"
)

type benchRow struct {
	Scenario string
	Driver   string
	Op       string
	NsOp     float64
	BytesOp  float64
	AllocsOp float64
	Ops      int64
}

// RenderBenchmarks is invoked by `go test -tags benchrender ./docs/bench` via TestRenderBenchmarks.
func RenderBenchmarks() {
	ctx := context.Background()
	rows := runBenchmarks(ctx)

	table := renderTable(rows)

	readmePath := filepath.Join(findRoot(), "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		panic(err)
	}
	out := injectTable(string(data), table)
	if err := os.WriteFile(readmePath, []byte(out), 0o644); err != nil {
		panic(err)
	}
	fmt.Println("✔ Benchmarks table updated")
}

type benchScenario struct {
	Name  string
	Title string
	Opts  []cache.StoreOption
}

func runBenchmarks(ctx context.Context) map[string]map[string][]benchRow {
	drivers := []string{"memory", "file", "redis", "nats", "memcached", "dynamodb", "sql_postgres", "sql_mysql", "sql_sqlite"}
	scenarios := []benchScenario{
		{
			Name:  "baseline",
			Title: "Baseline",
		},
		{
			Name:  "compression",
			Title: "With Compression",
			Opts:  []cache.StoreOption{cache.WithCompression(cache.CompressionGzip)},
		},
		{
			Name:  "encryption",
			Title: "With Encryption",
			Opts:  []cache.StoreOption{cache.WithEncryptionKey([]byte("0123456789abcdef0123456789abcdef"))},
		},
	}
	ops := map[string]func(context.Context, *cache.Cache){
		"Set":    doSet,
		"Get":    doGet,
		"Delete": doDelete,
	}

	results := make(map[string]map[string][]benchRow)

	for _, scenario := range scenarios {
		results[scenario.Name] = map[string][]benchRow{}
		for _, d := range drivers {
			c, cleanup, ok := buildCache(ctx, d, scenario.Opts...)
			if !ok {
				continue
			}
			if cleanup != nil {
				defer cleanup()
			}
			for opName, fn := range ops {
				ns, bytesOp, allocsOp, ops := benchOp(ctx, c, fn)
				results[scenario.Name][opName] = append(
					results[scenario.Name][opName],
					benchRow{
						Scenario: scenario.Name,
						Driver:   d,
						Op:       opName,
						NsOp:     ns,
						BytesOp:  bytesOp,
						AllocsOp: allocsOp,
						Ops:      ops,
					},
				)
			}
		}
	}
	return results
}

func benchOp(ctx context.Context, c *cache.Cache, fn func(context.Context, *cache.Cache)) (nsPerOp, bytesPerOp, allocsPerOp float64, ops int64) {
	// Use testing.Benchmark to gather ns/op and allocation metrics.
	res := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fn(ctx, c)
		}
	})
	return float64(res.NsPerOp()), float64(res.AllocedBytesPerOp()), float64(res.AllocsPerOp()), int64(res.N)
}

func doSet(ctx context.Context, c *cache.Cache) {
	_ = c.Set("bench:key", []byte("v"), time.Minute)
}

func doGet(ctx context.Context, c *cache.Cache) {
	_, _, _ = c.Get("bench:key")
}

func doDelete(ctx context.Context, c *cache.Cache) {
	_ = c.Delete("bench:key")
}

func renderTable(byScenario map[string]map[string][]benchRow) string {
	if len(byScenario) == 0 {
		return ""
	}

	scenarios := []benchScenario{
		{Name: "baseline", Title: "Baseline"},
		{Name: "compression", Title: "With Compression"},
		{Name: "encryption", Title: "With Encryption"},
	}
	ops := []string{"Set", "Get", "Delete"}

	var buf bytes.Buffer
	buf.WriteString(benchStart + "\n\n")
	for _, scenario := range scenarios {
		byOp, ok := byScenario[scenario.Name]
		if !ok {
			continue
		}
		buf.WriteString(fmt.Sprintf("### %s\n\n", scenario.Title))
		for _, op := range ops {
			rows := byOp[op]
			if len(rows) == 0 {
				continue
			}
			driverRows := make(map[string]benchRow, len(rows))
			var drivers []string
			for _, row := range rows {
				driverRows[row.Driver] = row
			}
			for d := range driverRows {
				drivers = append(drivers, d)
			}
			sort.Strings(drivers)
			buf.WriteString(fmt.Sprintf("#### %s\n\n", op))
			buf.WriteString("| Driver | N | ns/op | B/op | allocs/op |\n")
			buf.WriteString("|:------|---:|-----:|-----:|---------:|\n")
			for _, d := range drivers {
				row := driverRows[d]
				buf.WriteString(fmt.Sprintf("| %s | %d | %.0f | %.0f | %.0f |\n", d, row.Ops, row.NsOp, row.BytesOp, row.AllocsOp))
			}
			buf.WriteString("\n")
		}
	}
	buf.WriteString(benchEnd + "\n")
	return buf.String()
}

func injectTable(readme, table string) string {
	start := strings.Index(readme, benchStart)
	end := strings.Index(readme, benchEnd)
	if start == -1 || end == -1 || end < start {
		// append if anchors missing
		return readme + table
	}
	prefix := strings.TrimRight(readme[:start], "\n") + "\n\n"
	suffix := "\n" + strings.TrimLeft(readme[end+len(benchEnd):], "\n")

	var out bytes.Buffer
	out.WriteString(prefix)
	out.WriteString(table)
	out.WriteString(suffix)
	return out.String()
}

// Simplified builder: uses env when present, otherwise best-effort testcontainers for redis/memcached/postgres/mysql.
func buildCache(ctx context.Context, name string, opts ...cache.StoreOption) (*cache.Cache, func(), bool) {
	switch name {
	case "memory":
		return cache.NewCache(cache.NewMemoryStore(ctx, opts...)), func() {}, true
	case "file":
		dir, _ := os.MkdirTemp("", "cache-bench-file-*")
		return cache.NewCache(cache.NewFileStore(ctx, dir, opts...)), func() { _ = os.RemoveAll(dir) }, true
	case "redis":
		if addr := os.Getenv("REDIS_ADDR"); addr != "" {
			client := redis.NewClient(&redis.Options{Addr: addr})
			store := cache.NewRedisStore(ctx, client, opts...)
			return cache.NewCache(store), func() { _ = client.Close() }, true
		}
		if c, cleanup, err := startRedis(ctx, opts...); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip redis:", err)
		}
	case "nats":
		if addr := os.Getenv("NATS_URL"); addr != "" {
			if c, cleanup, err := newNATSBenchCache(ctx, addr, opts...); err == nil {
				return c, cleanup, true
			}
		}
		if c, cleanup, err := startNATS(ctx, opts...); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip nats:", err)
		}
	case "memcached":
		if addr := os.Getenv("MEMCACHED_ADDR"); addr != "" {
			store := cache.NewMemcachedStore(ctx, []string{addr}, opts...)
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startMemcached(ctx, opts...); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip memcached:", err)
		}
	case "dynamodb":
		if endpoint := os.Getenv("DYNAMO_ENDPOINT"); endpoint != "" {
			cfg := applyBenchOptions(cache.StoreConfig{
				Driver:         cache.DriverDynamo,
				DynamoEndpoint: endpoint,
				DynamoTable:    "cache_entries",
				DynamoRegion:   "us-east-1",
			}, opts...)
			store := cache.NewStore(ctx, cfg)
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startDynamo(ctx, opts...); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip dynamodb:", err)
		}
	case "sql_postgres":
		if dsn := os.Getenv("BENCH_PG_DSN"); dsn != "" {
			store := cache.NewSQLStore(ctx, "pgx", dsn, "cache_entries", opts...)
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startPostgres(ctx, opts...); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip postgres:", err)
		}
	case "sql_mysql":
		if dsn := os.Getenv("BENCH_MYSQL_DSN"); dsn != "" {
			store := cache.NewSQLStore(ctx, "mysql", dsn, "cache_entries", opts...)
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startMySQL(ctx, opts...); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip mysql:", err)
		}
	case "sql_sqlite":
		dsn := "file:" + filepath.Join(os.TempDir(), "bench.sqlite") + "?cache=shared&mode=rwc"
		store := cache.NewSQLStore(ctx, "sqlite", dsn, "cache_entries", opts...)
		return cache.NewCache(store), func() {}, true
	}
	return nil, nil, false
}

// --- container helpers (simplified; duplicated from bench_test) ---

func startRedis(ctx context.Context, opts ...cache.StoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second)}
	c, addr, err := startContainer(ctx, req, "6379/tcp")
	if err != nil {
		return nil, nil, err
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	store := cache.NewRedisStore(ctx, client, opts...)
	cleanup := func() { _ = client.Close(); _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startMemcached(ctx context.Context, opts ...cache.StoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "memcached:alpine", ExposedPorts: []string{"11211/tcp"}, WaitingFor: wait.ForListeningPort("11211/tcp").WithStartupTimeout(30 * time.Second)}
	c, addr, err := startContainer(ctx, req, "11211/tcp")
	if err != nil {
		return nil, nil, err
	}
	store := cache.NewMemcachedStore(ctx, []string{addr}, opts...)
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startDynamo(ctx context.Context, opts ...cache.StoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "amazon/dynamodb-local:latest", ExposedPorts: []string{"8000/tcp"}, WaitingFor: wait.ForListeningPort("8000/tcp").WithStartupTimeout(45 * time.Second)}
	c, addr, err := startContainer(ctx, req, "8000/tcp")
	if err != nil {
		return nil, nil, err
	}
	endpoint := "http://" + addr
	cfg := applyBenchOptions(cache.StoreConfig{
		Driver:         cache.DriverDynamo,
		DynamoEndpoint: endpoint,
		DynamoTable:    "cache_entries",
		DynamoRegion:   "us-east-1",
	}, opts...)
	store := cache.NewStore(ctx, cfg)
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startPostgres(ctx context.Context, opts ...cache.StoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "postgres:16-alpine", Env: map[string]string{"POSTGRES_PASSWORD": "pass", "POSTGRES_USER": "user", "POSTGRES_DB": "app"}, ExposedPorts: []string{"5432/tcp"}, WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second)}
	c, addr, err := startContainer(ctx, req, "5432/tcp")
	if err != nil {
		return nil, nil, err
	}
	dsn := "postgres://user:pass@" + addr + "/app?sslmode=disable"
	store := cache.NewSQLStore(ctx, "pgx", dsn, "cache_entries", opts...)
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startMySQL(ctx context.Context, opts ...cache.StoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "mysql:8",
		Env:          map[string]string{"MYSQL_ROOT_PASSWORD": "pass", "MYSQL_DATABASE": "app", "MYSQL_USER": "user", "MYSQL_PASSWORD": "pass"},
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
	store := cache.NewSQLStore(ctx, "mysql", dsn, "cache_entries", opts...)
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startNATS(ctx context.Context, opts ...cache.StoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "nats:2-alpine",
		Cmd:          []string{"-js"},
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor:   wait.ForListeningPort("4222/tcp").WithStartupTimeout(30 * time.Second),
	}
	c, addr, err := startContainer(ctx, req, "4222/tcp")
	if err != nil {
		return nil, nil, err
	}
	natsURL := "nats://" + addr
	cacheValue, cacheCleanup, err := newNATSBenchCache(ctx, natsURL, opts...)
	if err != nil {
		_ = c.Terminate(context.Background())
		return nil, nil, err
	}
	cleanup := func() {
		cacheCleanup()
		_ = c.Terminate(context.Background())
	}
	return cacheValue, cleanup, nil
}

func newNATSBenchCache(ctx context.Context, natsURL string, opts ...cache.StoreOption) (*cache.Cache, func(), error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	bucket := fmt.Sprintf("BENCH_%d", time.Now().UnixNano()%1_000_000_000)
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: bucket, History: 1})
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	store := cache.NewNATSStore(ctx, kv, opts...)
	cleanup := func() {
		_ = js.DeleteKeyValue(bucket)
		_ = nc.Drain()
		nc.Close()
	}
	return cache.NewCache(store), cleanup, nil
}

func startContainer(ctx context.Context, req testcontainers.ContainerRequest, port string) (testcontainers.Container, string, error) {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
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

func findRoot() string {
	cwd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd
		}
		next := filepath.Dir(cwd)
		if next == cwd {
			panic("go.mod not found")
		}
		cwd = next
	}
}

func applyBenchOptions(cfg cache.StoreConfig, opts ...cache.StoreOption) cache.StoreConfig {
	for _, opt := range opts {
		cfg = opt(cfg)
	}
	return cfg
}
