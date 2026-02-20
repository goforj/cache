//go:build benchrender
// +build benchrender

package bench

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
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
	Driver   string
	Op       string
	NsOp     float64
	BytesOp  float64
	AllocsOp float64
	Ops      int64
}

// RenderBenchmarks is invoked by `go test -tags benchrender ./docs/bench` via TestRenderBenchmarks.
func RenderBenchmarks() {
	root := findRoot()
	ctx := context.Background()
	rows := runBenchmarks(ctx)

	if err := writeDashboard(root, rows); err != nil {
		panic(err)
	}
	table := renderTable(rows)

	readmePath := filepath.Join(root, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		panic(err)
	}
	out := injectTable(string(data), table)
	if err := os.WriteFile(readmePath, []byte(out), 0o644); err != nil {
		panic(err)
	}
	fmt.Println("✔ Benchmarks dashboard updated")
}

func runBenchmarks(ctx context.Context) map[string][]benchRow {
	drivers := []string{"memory", "file", "redis", "nats", "nats_bucket_ttl", "memcached", "dynamodb", "sql_postgres", "sql_mysql", "sql_sqlite"}
	ops := map[string]func(context.Context, *cache.Cache){
		"Set":    doSet,
		"Get":    doGet,
		"Delete": doDelete,
	}

	results := make(map[string][]benchRow)
	for _, driver := range drivers {
		opts := []cache.StoreOption{}
		baseDriver := driver
		if driver == "nats_bucket_ttl" {
			baseDriver = "nats"
			opts = append(opts, cache.WithNATSBucketTTL(true))
		}

		c, cleanup, ok := buildCache(ctx, baseDriver, opts...)
		if !ok {
			continue
		}
		if cleanup != nil {
			defer cleanup()
		}
		for opName, fn := range ops {
			ns, bytes, allocs, opCount := benchOp(ctx, c, fn)
			results[opName] = append(results[opName], benchRow{
				Driver:   driver,
				Op:       opName,
				NsOp:     ns,
				BytesOp:  bytes,
				AllocsOp: allocs,
				Ops:      opCount,
			})
		}
	}
	return results
}

func benchOp(ctx context.Context, c *cache.Cache, fn func(context.Context, *cache.Cache)) (nsPerOp, bytesPerOp, allocsPerOp float64, ops int64) {
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

func renderTable(byOp map[string][]benchRow) string {
	var buf bytes.Buffer
	buf.WriteString(benchStart + "\n\n")
	buf.WriteString("### Latency (ns/op)\n\n")
	buf.WriteString("![Cache benchmark latency chart](docs/bench/benchmarks_ns.svg)\n\n")
	buf.WriteString("### Iterations (N)\n\n")
	buf.WriteString("![Cache benchmark iteration chart](docs/bench/benchmarks_ops.svg)\n\n")
	buf.WriteString("### Allocated Bytes (B/op)\n\n")
	buf.WriteString("![Cache benchmark bytes chart](docs/bench/benchmarks_bytes.svg)\n\n")
	buf.WriteString("### Allocations (allocs/op)\n\n")
	buf.WriteString("![Cache benchmark allocs chart](docs/bench/benchmarks_allocs.svg)\n\n")
	buf.WriteString(benchEnd + "\n")
	return buf.String()
}

func writeDashboard(root string, byOp map[string][]benchRow) error {
	ops := []string{"Get", "Set", "Delete"}
	byDriver := map[string]map[string]float64{}
	for _, op := range ops {
		for _, row := range byOp[op] {
			if byDriver[row.Driver] == nil {
				byDriver[row.Driver] = map[string]float64{}
			}
			byDriver[row.Driver][op] = row.NsOp
		}
	}

	var drivers []string
	for driver := range byDriver {
		drivers = append(drivers, driver)
	}
	drivers = orderDrivers(drivers)
	if len(drivers) == 0 {
		return nil
	}
	if err := writeDashboardSVG(root, "benchmarks_ns.svg", "Cache Benchmark Latency", "ns/op", "linear", drivers, byDriver); err != nil {
		return err
	}
	if err := writeMetricSVG(root, "benchmarks_ops.svg", "Cache Benchmark Iterations", "N", "split", drivers, byOp, func(r benchRow) float64 {
		return float64(r.Ops)
	}); err != nil {
		return err
	}
	if err := writeMetricSVG(root, "benchmarks_bytes.svg", "Cache Benchmark Allocated Bytes", "B/op", "linear", drivers, byOp, func(r benchRow) float64 {
		return r.BytesOp
	}); err != nil {
		return err
	}
	return writeMetricSVG(root, "benchmarks_allocs.svg", "Cache Benchmark Allocations", "allocs/op", "linear", drivers, byOp, func(r benchRow) float64 {
		return r.AllocsOp
	})
}

func orderDrivers(drivers []string) []string {
	order := []string{
		"memory",
		"file",
		"sql_sqlite",
		"sql_postgres",
		"sql_mysql",
		"redis",
		"memcached",
		"nats",
		"nats_bucket_ttl",
		"dynamodb",
	}
	seen := make(map[string]bool, len(drivers))
	for _, d := range drivers {
		seen[d] = true
	}
	out := make([]string, 0, len(drivers))
	for _, d := range order {
		if seen[d] {
			out = append(out, d)
			delete(seen, d)
		}
	}
	for _, d := range drivers {
		if seen[d] {
			out = append(out, d)
			delete(seen, d)
		}
	}
	return out
}

func writeMetricSVG(root, fileName, title, yUnit, scale string, drivers []string, byOp map[string][]benchRow, valueFn func(benchRow) float64) error {
	ops := []string{"Get", "Set", "Delete"}
	byDriver := map[string]map[string]float64{}
	for _, op := range ops {
		for _, row := range byOp[op] {
			if byDriver[row.Driver] == nil {
				byDriver[row.Driver] = map[string]float64{}
			}
			byDriver[row.Driver][op] = valueFn(row)
		}
	}
	return writeDashboardSVG(root, fileName, title, yUnit, scale, drivers, byDriver)
}

func writeDashboardSVG(root, fileName, title, yUnit, scale string, drivers []string, byDriver map[string]map[string]float64) error {
	if scale == "split" {
		return writeDashboardSplitSVG(root, fileName, title, yUnit, drivers, byDriver)
	}
	const (
		width       = 1600
		height      = 920
		marginLeft  = 90
		marginRight = 50
		marginTop   = 90
		marginBot   = 210
	)
	ops := []string{"Get", "Set", "Delete"}
	colors := map[string]string{"Get": "#4e79a7", "Set": "#59a14f", "Delete": "#e15759"}
	plotW := width - marginLeft - marginRight
	plotH := height - marginTop - marginBot

	var svg bytes.Buffer
	svg.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	svg.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="` + strconv.Itoa(width) + `" height="` + strconv.Itoa(height) + `" viewBox="0 0 ` + strconv.Itoa(width) + ` ` + strconv.Itoa(height) + `">` + "\n")
	svg.WriteString(`<rect width="100%" height="100%" fill="#111827"/>` + "\n")
	svg.WriteString(`<text x="` + strconv.Itoa(width/2) + `" y="44" text-anchor="middle" fill="#f9fafb" font-size="34" font-family="Arial, sans-serif">` + title + ` (` + yUnit + `)</text>` + "\n")
	scaleLabel := "linear"
	if scale == "log" {
		scaleLabel = "log"
	}
	svg.WriteString(`<text x="` + strconv.Itoa(width/2) + `" y="72" text-anchor="middle" fill="#9ca3af" font-size="18" font-family="Arial, sans-serif">Stacked by operation (Get, Set, Delete), ` + scaleLabel + ` y-scale</text>` + "\n")

	axisX0 := marginLeft
	axisX1 := width - marginRight
	axisY1 := marginTop
	axisY0 := height - marginBot

	maxV := 0.0
	for _, driver := range drivers {
		total := 0.0
		for _, op := range ops {
			total += byDriver[driver][op]
		}
		if total > maxV {
			maxV = total
		}
	}
	if maxV <= 0 {
		maxV = 1
	}
	maxV *= 1.1
	maxMapped := mapValue(maxV, scale)
	if maxMapped <= 0 {
		maxMapped = 1
	}

	mapToY := func(v float64) int {
		if v < 0 {
			v = 0
		}
		p := mapValue(v, scale) / maxMapped
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		return axisY0 - int(p*float64(plotH))
	}

	yTicks := 5
	for i := 0; i <= yTicks; i++ {
		p := float64(i) / float64(yTicks)
		y := axisY0 - int(p*float64(plotH))
		v := int(unmapValue(p*maxMapped, scale))
		svg.WriteString(`<line x1="` + strconv.Itoa(axisX0) + `" y1="` + strconv.Itoa(y) + `" x2="` + strconv.Itoa(axisX1) + `" y2="` + strconv.Itoa(y) + `" stroke="#374151" stroke-width="1"/>` + "\n")
		svg.WriteString(`<text x="` + strconv.Itoa(axisX0-10) + `" y="` + strconv.Itoa(y+5) + `" text-anchor="end" fill="#d1d5db" font-size="13" font-family="Arial, sans-serif">` + strconv.Itoa(v) + `</text>` + "\n")
	}

	svg.WriteString(`<line x1="` + strconv.Itoa(axisX0) + `" y1="` + strconv.Itoa(axisY0) + `" x2="` + strconv.Itoa(axisX1) + `" y2="` + strconv.Itoa(axisY0) + `" stroke="#e5e7eb" stroke-width="2"/>` + "\n")
	svg.WriteString(`<line x1="` + strconv.Itoa(axisX0) + `" y1="` + strconv.Itoa(axisY0) + `" x2="` + strconv.Itoa(axisX0) + `" y2="` + strconv.Itoa(axisY1) + `" stroke="#e5e7eb" stroke-width="2"/>` + "\n")

	groupW := plotW / len(drivers)
	barW := groupW / 2
	if barW < 4 {
		barW = 4
	}
	for i, driver := range drivers {
		barX := axisX0 + i*groupW + (groupW-barW)/2
		cumulative := 0.0
		for _, op := range ops {
			v := byDriver[driver][op]
			y0 := mapToY(cumulative)
			cumulative += v
			y1 := mapToY(cumulative)
			y := y1
			h := y0 - y1
			if h < 1 && v > 0 {
				h = 1
			}
			svg.WriteString(`<rect x="` + strconv.Itoa(barX) + `" y="` + strconv.Itoa(y) + `" width="` + strconv.Itoa(barW) + `" height="` + strconv.Itoa(h) + `" fill="` + colors[op] + `"/>` + "\n")
		}
		labelX := axisX0 + i*groupW + groupW/2
		labelY := axisY0 + 102
		svg.WriteString(`<text x="` + strconv.Itoa(labelX) + `" y="` + strconv.Itoa(labelY) + `" text-anchor="middle" fill="#d1d5db" font-size="20" font-family="Arial, sans-serif" transform="rotate(-28,` + strconv.Itoa(labelX) + `,` + strconv.Itoa(labelY) + `)">` + driver + `</text>` + "\n")
	}

	legendX := axisX1 - 300
	legendY := 95
	for i, op := range ops {
		y := legendY + i*32
		svg.WriteString(`<rect x="` + strconv.Itoa(legendX) + `" y="` + strconv.Itoa(y-14) + `" width="20" height="20" fill="` + colors[op] + `"/>` + "\n")
		svg.WriteString(`<text x="` + strconv.Itoa(legendX+30) + `" y="` + strconv.Itoa(y+1) + `" fill="#f3f4f6" font-size="20" font-family="Arial, sans-serif">` + op + `</text>` + "\n")
	}

	svg.WriteString(`</svg>` + "\n")
	outPath := filepath.Join(root, "docs", "bench", fileName)
	return os.WriteFile(outPath, svg.Bytes(), 0o644)
}

func mapValue(v float64, scale string) float64 {
	if scale == "log" {
		return math.Log10(v + 1)
	}
	return v
}

func unmapValue(v float64, scale string) float64 {
	if scale == "log" {
		return math.Pow(10, v) - 1
	}
	return v
}

func writeDashboardSplitSVG(root, fileName, title, yUnit string, drivers []string, byDriver map[string]map[string]float64) error {
	const (
		width       = 1600
		height      = 1100
		marginLeft  = 90
		marginRight = 50
		marginTop   = 90
		marginBot   = 220
		panelGap    = 80
	)
	ops := []string{"Get", "Set", "Delete"}
	colors := map[string]string{"Get": "#4e79a7", "Set": "#59a14f", "Delete": "#e15759"}
	plotW := width - marginLeft - marginRight
	panelH := (height - marginTop - marginBot - panelGap) / 2

	totals := make(map[string]float64, len(drivers))
	outlier := ""
	outlierTotal := -1.0
	for _, d := range drivers {
		total := 0.0
		for _, op := range ops {
			total += byDriver[d][op]
		}
		totals[d] = total
		if total > outlierTotal {
			outlierTotal = total
			outlier = d
		}
	}
	var rest []string
	for _, d := range drivers {
		if d != outlier {
			rest = append(rest, d)
		}
	}

	var svg bytes.Buffer
	svg.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	svg.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="` + strconv.Itoa(width) + `" height="` + strconv.Itoa(height) + `" viewBox="0 0 ` + strconv.Itoa(width) + ` ` + strconv.Itoa(height) + `">` + "\n")
	svg.WriteString(`<rect width="100%" height="100%" fill="#111827"/>` + "\n")
	svg.WriteString(`<text x="` + strconv.Itoa(width/2) + `" y="44" text-anchor="middle" fill="#f9fafb" font-size="34" font-family="Arial, sans-serif">` + title + ` (` + yUnit + `)</text>` + "\n")
	svg.WriteString(`<text x="` + strconv.Itoa(width/2) + `" y="72" text-anchor="middle" fill="#9ca3af" font-size="18" font-family="Arial, sans-serif">Linear split view: top outlier, bottom remaining drivers</text>` + "\n")

	legendX := width - 350
	legendY := 95
	for i, op := range ops {
		y := legendY + i*32
		svg.WriteString(`<rect x="` + strconv.Itoa(legendX) + `" y="` + strconv.Itoa(y-14) + `" width="20" height="20" fill="` + colors[op] + `"/>` + "\n")
		svg.WriteString(`<text x="` + strconv.Itoa(legendX+30) + `" y="` + strconv.Itoa(y+1) + `" fill="#f3f4f6" font-size="20" font-family="Arial, sans-serif">` + op + `</text>` + "\n")
	}

	drawPanel := func(panelTitle string, panelDrivers []string, yTop int) {
		if len(panelDrivers) == 0 {
			return
		}
		axisX0 := marginLeft
		axisX1 := width - marginRight
		axisY1 := yTop
		axisY0 := yTop + panelH

		maxV := 0.0
		for _, d := range panelDrivers {
			if totals[d] > maxV {
				maxV = totals[d]
			}
		}
		if maxV <= 0 {
			maxV = 1
		}
		maxV *= 1.1

		svg.WriteString(`<text x="` + strconv.Itoa(axisX0) + `" y="` + strconv.Itoa(axisY1-12) + `" fill="#f3f4f6" font-size="22" font-family="Arial, sans-serif">` + panelTitle + `</text>` + "\n")
		for i := 0; i <= 5; i++ {
			p := float64(i) / 5.0
			y := axisY0 - int(p*float64(panelH))
			v := int(p * maxV)
			svg.WriteString(`<line x1="` + strconv.Itoa(axisX0) + `" y1="` + strconv.Itoa(y) + `" x2="` + strconv.Itoa(axisX1) + `" y2="` + strconv.Itoa(y) + `" stroke="#374151" stroke-width="1"/>` + "\n")
			svg.WriteString(`<text x="` + strconv.Itoa(axisX0-10) + `" y="` + strconv.Itoa(y+5) + `" text-anchor="end" fill="#d1d5db" font-size="13" font-family="Arial, sans-serif">` + strconv.Itoa(v) + `</text>` + "\n")
		}
		svg.WriteString(`<line x1="` + strconv.Itoa(axisX0) + `" y1="` + strconv.Itoa(axisY0) + `" x2="` + strconv.Itoa(axisX1) + `" y2="` + strconv.Itoa(axisY0) + `" stroke="#e5e7eb" stroke-width="2"/>` + "\n")
		svg.WriteString(`<line x1="` + strconv.Itoa(axisX0) + `" y1="` + strconv.Itoa(axisY0) + `" x2="` + strconv.Itoa(axisX0) + `" y2="` + strconv.Itoa(axisY1) + `" stroke="#e5e7eb" stroke-width="2"/>` + "\n")

		groupW := plotW / len(panelDrivers)
		barW := groupW / 2
		if barW < 8 {
			barW = 8
		}
		for i, d := range panelDrivers {
			barX := axisX0 + i*groupW + (groupW-barW)/2
			stackTop := axisY0
			for _, op := range ops {
				v := byDriver[d][op]
				h := int((v / maxV) * float64(panelH))
				if h < 1 && v > 0 {
					h = 1
				}
				y := stackTop - h
				svg.WriteString(`<rect x="` + strconv.Itoa(barX) + `" y="` + strconv.Itoa(y) + `" width="` + strconv.Itoa(barW) + `" height="` + strconv.Itoa(h) + `" fill="` + colors[op] + `"/>` + "\n")
				stackTop = y
			}
			labelX := axisX0 + i*groupW + groupW/2
			labelY := axisY0 + 102
			svg.WriteString(`<text x="` + strconv.Itoa(labelX) + `" y="` + strconv.Itoa(labelY) + `" text-anchor="middle" fill="#d1d5db" font-size="20" font-family="Arial, sans-serif" transform="rotate(-28,` + strconv.Itoa(labelX) + `,` + strconv.Itoa(labelY) + `)">` + d + `</text>` + "\n")
		}
	}

	drawPanel("Outlier: "+outlier, []string{outlier}, marginTop)
	drawPanel("Remaining drivers", rest, marginTop+panelH+panelGap)

	svg.WriteString(`</svg>` + "\n")
	outPath := filepath.Join(root, "docs", "bench", fileName)
	return os.WriteFile(outPath, svg.Bytes(), 0o644)
}

func injectTable(readme, table string) string {
	start := strings.Index(readme, benchStart)
	end := strings.Index(readme, benchEnd)
	if start == -1 || end == -1 || end < start {
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
	req := testcontainers.ContainerRequest{Image: "redis:7-bookworm", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second)}
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
	req := testcontainers.ContainerRequest{Image: "memcached:1.6-bookworm", ExposedPorts: []string{"11211/tcp"}, WaitingFor: wait.ForListeningPort("11211/tcp").WithStartupTimeout(30 * time.Second)}
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
	req := testcontainers.ContainerRequest{Image: "postgres:16-bookworm", Env: map[string]string{"POSTGRES_PASSWORD": "pass", "POSTGRES_USER": "user", "POSTGRES_DB": "app"}, ExposedPorts: []string{"5432/tcp"}, WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second)}
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
		Image:        "nats:2",
		Cmd:          []string{"-js"},
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor:   wait.ForLog("Server is ready").WithStartupTimeout(30 * time.Second),
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
