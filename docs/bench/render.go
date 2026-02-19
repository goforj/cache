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

func runBenchmarks(ctx context.Context) map[string][]benchRow {
	drivers := []string{"memory", "file", "redis", "memcached", "dynamodb", "sql_postgres", "sql_mysql", "sql_sqlite"}
	ops := map[string]func(context.Context, *cache.Cache){
		"Set": doSet,
		"Get": doGet,
		"Add": doAdd,
		"Inc": doInc,
		"Dec": doDec,
	}

	results := make(map[string][]benchRow)

	for opName, fn := range ops {
		for _, d := range drivers {
			c, cleanup, ok := buildCache(ctx, d)
			if !ok {
				continue
			}
			if cleanup != nil {
				defer cleanup()
			}
			ns, bytesOp, allocsOp, ops := benchOp(ctx, c, fn)
			results[opName] = append(results[opName], benchRow{Driver: d, Op: opName, NsOp: ns, BytesOp: bytesOp, AllocsOp: allocsOp, Ops: ops})
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

func doAdd(ctx context.Context, c *cache.Cache) {
	_, _ = c.Add("bench:add", []byte("1"), time.Minute)
}

func doInc(ctx context.Context, c *cache.Cache) {
	_, _ = c.Increment("bench:counter", 1, time.Minute)
}

func doDec(ctx context.Context, c *cache.Cache) {
	_, _ = c.Decrement("bench:counter", 1, time.Minute)
}

func renderTable(byOp map[string][]benchRow) string {
	if len(byOp) == 0 {
		return ""
	}

	ops := []string{"Set", "Get", "Add", "Inc", "Dec"}

	// Build lookup op -> driver -> row
	lookup := make(map[string]map[string]benchRow)
	for op, rows := range byOp {
		for _, r := range rows {
			if lookup[op] == nil {
				lookup[op] = make(map[string]benchRow)
			}
			lookup[op][r.Driver] = r
		}
	}

	var buf bytes.Buffer
	buf.WriteString(benchStart + "\n\n")
	buf.WriteString("| Driver | Op | N | ns/op | B/op | allocs/op |\n")
	buf.WriteString("|:------|:--:|---:|-----:|-----:|---------:|\n")
	for _, op := range ops {
		driverRows := lookup[op]
		if len(driverRows) == 0 {
			continue
		}
		var drivers []string
		for d := range driverRows {
			drivers = append(drivers, d)
		}
		sort.Strings(drivers)
		for _, d := range drivers {
			row := driverRows[d]
			buf.WriteString(fmt.Sprintf("| %s | %s | %d | %.0f | %.0f | %.0f |\n", d, op, row.Ops, row.NsOp, row.BytesOp, row.AllocsOp))
		}
	}
	buf.WriteString("\n" + benchEnd + "\n")
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
func buildCache(ctx context.Context, name string) (*cache.Cache, func(), bool) {
	switch name {
	case "memory":
		return cache.NewCache(cache.NewMemoryStore(ctx)), func() {}, true
	case "file":
		dir, _ := os.MkdirTemp("", "cache-bench-file-*")
		return cache.NewCache(cache.NewFileStore(ctx, dir)), func() { _ = os.RemoveAll(dir) }, true
	case "redis":
		if addr := os.Getenv("REDIS_ADDR"); addr != "" {
			client := redis.NewClient(&redis.Options{Addr: addr})
			store := cache.NewRedisStore(ctx, client)
			return cache.NewCache(store), func() { _ = client.Close() }, true
		}
		if c, cleanup, err := startRedis(ctx); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip redis:", err)
		}
	case "memcached":
		if addr := os.Getenv("MEMCACHED_ADDR"); addr != "" {
			store := cache.NewMemcachedStore(ctx, []string{addr})
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startMemcached(ctx); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip memcached:", err)
		}
	case "dynamodb":
		if endpoint := os.Getenv("DYNAMO_ENDPOINT"); endpoint != "" {
			store := cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverDynamo, DynamoEndpoint: endpoint, DynamoTable: "cache_entries", DynamoRegion: "us-east-1"})
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startDynamo(ctx); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip dynamodb:", err)
		}
	case "sql_postgres":
		if dsn := os.Getenv("BENCH_PG_DSN"); dsn != "" {
			store := cache.NewSQLStore(ctx, "pgx", dsn, "cache_entries")
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startPostgres(ctx); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip postgres:", err)
		}
	case "sql_mysql":
		if dsn := os.Getenv("BENCH_MYSQL_DSN"); dsn != "" {
			store := cache.NewSQLStore(ctx, "mysql", dsn, "cache_entries")
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startMySQL(ctx); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip mysql:", err)
		}
	case "sql_sqlite":
		dsn := "file:" + filepath.Join(os.TempDir(), "bench.sqlite") + "?cache=shared&mode=rwc"
		store := cache.NewSQLStore(ctx, "sqlite", dsn, "cache_entries")
		return cache.NewCache(store), func() {}, true
	}
	return nil, nil, false
}

// --- container helpers (simplified; duplicated from bench_test) ---

func startRedis(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second)}
	c, addr, err := startContainer(ctx, req, "6379/tcp")
	if err != nil {
		return nil, nil, err
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	store := cache.NewRedisStore(ctx, client)
	cleanup := func() { _ = client.Close(); _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startMemcached(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "memcached:alpine", ExposedPorts: []string{"11211/tcp"}, WaitingFor: wait.ForListeningPort("11211/tcp").WithStartupTimeout(30 * time.Second)}
	c, addr, err := startContainer(ctx, req, "11211/tcp")
	if err != nil {
		return nil, nil, err
	}
	store := cache.NewMemcachedStore(ctx, []string{addr})
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startDynamo(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "amazon/dynamodb-local:latest", ExposedPorts: []string{"8000/tcp"}, WaitingFor: wait.ForListeningPort("8000/tcp").WithStartupTimeout(45 * time.Second)}
	c, addr, err := startContainer(ctx, req, "8000/tcp")
	if err != nil {
		return nil, nil, err
	}
	endpoint := "http://" + addr
	store := cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverDynamo, DynamoEndpoint: endpoint, DynamoTable: "cache_entries", DynamoRegion: "us-east-1"})
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startPostgres(ctx context.Context) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "postgres:16-alpine", Env: map[string]string{"POSTGRES_PASSWORD": "pass", "POSTGRES_USER": "user", "POSTGRES_DB": "app"}, ExposedPorts: []string{"5432/tcp"}, WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second)}
	c, addr, err := startContainer(ctx, req, "5432/tcp")
	if err != nil {
		return nil, nil, err
	}
	dsn := "postgres://user:pass@" + addr + "/app?sslmode=disable"
	store := cache.NewSQLStore(ctx, "pgx", dsn, "cache_entries")
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startMySQL(ctx context.Context) (*cache.Cache, func(), error) {
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
	store := cache.NewSQLStore(ctx, "mysql", dsn, "cache_entries")
	cleanup := func() { _ = c.Terminate(context.Background()) }
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
