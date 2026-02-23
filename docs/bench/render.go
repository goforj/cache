//go:build benchrender
// +build benchrender

package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/dynamocache"
	"github.com/goforj/cache/driver/memcachedcache"
	"github.com/goforj/cache/driver/natscache"
	"github.com/goforj/cache/driver/rediscache"
	"github.com/goforj/cache/driver/sqlcore"
	"github.com/goforj/cache/driver/sqlitecache"
	"github.com/redis/go-redis/v9"
)

const (
	benchStart = "<!-- bench:embed:start -->"
	benchEnd   = "<!-- bench:embed:end -->"
	benchRows  = "benchmarks_rows.json"
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
	rowsPath := filepath.Join(root, "docs", "bench", benchRows)

	var rows map[string][]benchRow
	if os.Getenv("BENCH_RENDER_ONLY") == "1" {
		loaded, err := loadBenchmarkRows(rowsPath)
		if err != nil {
			panic(fmt.Errorf("render-only mode requires snapshot at %s: %w", rowsPath, err))
		}
		rows = loaded
	} else {
		rows = runBenchmarks(ctx)
		if err := saveBenchmarkRows(rowsPath, rows); err != nil {
			panic(err)
		}
	}

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
	drivers := []string{"memory", "file", "redis", "memcached", "sql_postgres", "sql_mysql", "sql_sqlite"}
	ops := map[string]struct {
		setup func(context.Context, *cache.Cache)
		run   func(context.Context, *cache.Cache)
	}{
		"set_bytes":        {run: doSetBytes},
		"set_string":       {run: doSetString},
		"set_typed_string": {run: doSetTypedString},
		"set_typed_struct": {run: doSetTypedStruct},
		"get_bytes":        {setup: setupGetBytes, run: doGetBytes},
		"get_string":       {setup: setupGetString, run: doGetString},
		"get_typed_string": {setup: setupGetTypedString, run: doGetTypedString},
		"get_typed_struct": {setup: setupGetTypedStruct, run: doGetTypedStruct},
		"delete":           {setup: setupDelete, run: doDelete},
	}

	results := make(map[string][]benchRow)
	for _, driver := range drivers {
		c, cleanup, ok := buildCache(ctx, driver)
		if !ok {
			continue
		}
		if cleanup != nil {
			defer cleanup()
		}
		for opName, op := range ops {
			if op.setup != nil {
				op.setup(ctx, c)
			}
			ns, bytes, allocs, opCount := benchOp(ctx, c, op.run)
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

	// Run NATS variants as a paired benchmark from one shared server URL so
	// nats and nats_bucket_ttl appear together in rendered charts.
	natsURL, natsServerCleanup, ok := natsBenchURL(ctx)
	if ok {
		if natsServerCleanup != nil {
			defer natsServerCleanup()
		}
		variants := []struct {
			name string
			opts []benchStoreOption
		}{
			{name: "nats"},
			{name: "nats_bucket_ttl", opts: []benchStoreOption{benchWithNATSBucketTTL(true)}},
		}
		var pairRows []benchRow
		pairOK := true
		for _, variant := range variants {
			c, cleanup, err := newNATSBenchCache(ctx, natsURL, variant.opts...)
			if err != nil {
				pairOK = false
				if cleanup != nil {
					cleanup()
				}
				fmt.Fprintln(os.Stderr, "benchrender: skip nats pair:", err)
				break
			}
			if cleanup != nil {
				defer cleanup()
			}
			for opName, op := range ops {
				if op.setup != nil {
					op.setup(ctx, c)
				}
				ns, bytes, allocs, opCount := benchOp(ctx, c, op.run)
				pairRows = append(pairRows, benchRow{
					Driver:   variant.name,
					Op:       opName,
					NsOp:     ns,
					BytesOp:  bytes,
					AllocsOp: allocs,
					Ops:      opCount,
				})
			}
		}
		if pairOK {
			for _, row := range pairRows {
				results[row.Op] = append(results[row.Op], row)
			}
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

type benchProfile struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
}

func doSetBytes(ctx context.Context, c *cache.Cache) {
	_ = c.SetBytes("bench:key", []byte("v"), time.Minute)
}

func doSetString(ctx context.Context, c *cache.Cache) {
	_ = c.SetString("bench:key", "v", time.Minute)
}

func doSetTypedString(ctx context.Context, c *cache.Cache) {
	_ = cache.Set(c, "bench:key", "v", time.Minute)
}

func doSetTypedStruct(ctx context.Context, c *cache.Cache) {
	_ = cache.Set(c, "bench:key", benchProfile{Name: "Ada", Level: 7}, time.Minute)
}

func setupGetBytes(ctx context.Context, c *cache.Cache) {
	_ = c.SetBytes("bench:key", []byte("v"), time.Minute)
}

func setupGetString(ctx context.Context, c *cache.Cache) {
	_ = c.SetString("bench:key", "v", time.Minute)
}

func setupGetTypedString(ctx context.Context, c *cache.Cache) {
	_ = cache.Set(c, "bench:key", "v", time.Minute)
}

func setupGetTypedStruct(ctx context.Context, c *cache.Cache) {
	_ = cache.Set(c, "bench:key", benchProfile{Name: "Ada", Level: 7}, time.Minute)
}

func setupDelete(ctx context.Context, c *cache.Cache) {
	_ = c.SetBytes("bench:key", []byte("v"), time.Minute)
}

func doGetBytes(ctx context.Context, c *cache.Cache) {
	_, _, _ = c.GetBytes("bench:key")
}

func doGetString(ctx context.Context, c *cache.Cache) {
	_, _, _ = c.GetString("bench:key")
}

func doGetTypedString(ctx context.Context, c *cache.Cache) {
	_, _, _ = cache.Get[string](c, "bench:key")
}

func doGetTypedStruct(ctx context.Context, c *cache.Cache) {
	_, _, _ = cache.Get[benchProfile](c, "bench:key")
}

func doDelete(ctx context.Context, c *cache.Cache) {
	_ = c.Delete("bench:key")
}

func natsBenchURL(ctx context.Context) (string, func(), bool) {
	if addr := os.Getenv("NATS_URL"); addr != "" {
		return addr, func() {}, true
	}
	req := testcontainers.ContainerRequest{
		Image:        "nats:2",
		Cmd:          []string{"-js"},
		ExposedPorts: []string{"4222/tcp"},
	}
	c, addr, err := startContainer(ctx, req, "4222/tcp")
	if err != nil {
		fmt.Fprintln(os.Stderr, "benchrender: skip nats pair:", err)
		return "", nil, false
	}
	url := "nats://" + addr
	nc, err := connectNATSWithRetry(url, 15*time.Second)
	if err != nil {
		_ = c.Terminate(context.Background())
		fmt.Fprintln(os.Stderr, "benchrender: skip nats pair:", err)
		return "", nil, false
	}
	nc.Close()
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return url, cleanup, true
}

func renderTable(byOp map[string][]benchRow) string {
	var buf bytes.Buffer
	buf.WriteString(benchStart + "\n\n")
	buf.WriteString("Note: DynamoDB is intentionally omitted from these local charts because emulator-based numbers are not representative of real AWS latency.\n\n")
	buf.WriteString("NATS variants in these charts:\n\n")
	buf.WriteString("- `nats`: per-key TTL semantics using a binary envelope (`magic/expiresAt/value`). This preserves per-key expiry parity with other drivers, with modest metadata overhead.\n")
	buf.WriteString("- `nats_bucket_ttl`: bucket-level TTL mode (`WithNATSBucketTTL(true)`), raw value path; faster but different expiry semantics.\n\n")
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
	ops := benchChartOps()
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
	if err := writeDashboardSVG(root, "benchmarks_ns.svg", "Cache Benchmark Latency", "ns/op", "split", drivers, byDriver); err != nil {
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
	ops := benchChartOps()
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
		height      = 690
		marginLeft  = 180
		marginRight = 200
		marginTop   = 90
		marginBot   = 130
	)
	ops := benchChartOps()
	colors := benchOpColors()
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
	svg.WriteString(`<text x="` + strconv.Itoa(width/2) + `" y="84" text-anchor="middle" fill="#9ca3af" font-size="30" font-family="Arial, sans-serif">Grouped by operation, ` + scaleLabel + ` y-scale, ` + metricPreference(yUnit) + `</text>` + "\n")

	axisX0 := marginLeft
	axisX1 := width - marginRight
	axisY1 := marginTop
	axisY0 := height - marginBot

	maxV := 0.0
	for _, driver := range drivers {
		for _, op := range ops {
			if byDriver[driver][op] > maxV {
				maxV = byDriver[driver][op]
			}
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
		svg.WriteString(`<text x="` + strconv.Itoa(axisX0-10) + `" y="` + strconv.Itoa(y+10) + `" text-anchor="end" fill="#d1d5db" font-size="30" font-family="Arial, sans-serif">` + strconv.Itoa(v) + `</text>` + "\n")
	}

	svg.WriteString(`<line x1="` + strconv.Itoa(axisX0) + `" y1="` + strconv.Itoa(axisY0) + `" x2="` + strconv.Itoa(axisX1) + `" y2="` + strconv.Itoa(axisY0) + `" stroke="#e5e7eb" stroke-width="2"/>` + "\n")
	svg.WriteString(`<line x1="` + strconv.Itoa(axisX0) + `" y1="` + strconv.Itoa(axisY0) + `" x2="` + strconv.Itoa(axisX0) + `" y2="` + strconv.Itoa(axisY1) + `" stroke="#e5e7eb" stroke-width="2"/>` + "\n")

	groupW := plotW / len(drivers)
	groupGap := groupW / 6
	if groupGap < 10 {
		groupGap = 10
	}
	innerGap := groupW / 22
	if innerGap < 2 {
		innerGap = 2
	}
	usableW := groupW - groupGap
	if usableW < len(ops)*3+innerGap*4 {
		usableW = len(ops)*3 + innerGap*4
	}
	barW := (usableW - innerGap*4) / len(ops)
	barW = (barW * 85) / 100
	if barW < 3 {
		barW = 3
	}
	for i, driver := range drivers {
		groupX := axisX0 + i*groupW
		clusterW := len(ops)*barW + (len(ops)-1)*innerGap
		groupStart := groupX + (groupW-clusterW)/2
		for j, op := range ops {
			v := byDriver[driver][op]
			y := mapToY(v)
			h := axisY0 - y
			if h < 1 && v > 0 {
				h = 1
			}
			barX := groupStart + j*(barW+innerGap)
			svg.WriteString(`<rect x="` + strconv.Itoa(barX) + `" y="` + strconv.Itoa(y) + `" width="` + strconv.Itoa(barW) + `" height="` + strconv.Itoa(h) + `" fill="` + colors[op] + `"/>` + "\n")
		}
		labelX := axisX0 + i*groupW + groupW/2
		labelY := axisY0 + 82
		svg.WriteString(`<text x="` + strconv.Itoa(labelX) + `" y="` + strconv.Itoa(labelY) + `" text-anchor="middle" fill="#d1d5db" font-size="30" font-family="Arial, sans-serif">` + displayDriverName(driver) + `</text>` + "\n")
	}

	legendX := axisX1 + 20
	legendY := 95
	for i, op := range ops {
		y := legendY + i*32
		svg.WriteString(`<rect x="` + strconv.Itoa(legendX) + `" y="` + strconv.Itoa(y-14) + `" width="20" height="20" fill="` + colors[op] + `"/>` + "\n")
		svg.WriteString(`<text x="` + strconv.Itoa(legendX+30) + `" y="` + strconv.Itoa(y+1) + `" fill="#f3f4f6" font-size="28" font-family="Arial, sans-serif">` + benchOpLabel(op) + `</text>` + "\n")
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
		height      = 825
		marginLeft  = 180
		marginRight = 200
		marginTop   = 150
		marginBot   = 150
		panelGap    = 80
	)
	ops := benchChartOps()
	colors := benchOpColors()
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
	svg.WriteString(`<text x="` + strconv.Itoa(width/2) + `" y="96" text-anchor="middle" fill="#9ca3af" font-size="30" font-family="Arial, sans-serif">Linear split view (grouped bars): top outlier, bottom remaining drivers, ` + metricPreference(yUnit) + `</text>` + "\n")

	plotRight := width - marginRight
	legendX := plotRight + 20
	legendY := 130
	for i, op := range ops {
		y := legendY + i*32
		svg.WriteString(`<rect x="` + strconv.Itoa(legendX) + `" y="` + strconv.Itoa(y-14) + `" width="20" height="20" fill="` + colors[op] + `"/>` + "\n")
		svg.WriteString(`<text x="` + strconv.Itoa(legendX+30) + `" y="` + strconv.Itoa(y+1) + `" fill="#f3f4f6" font-size="28" font-family="Arial, sans-serif">` + benchOpLabel(op) + `</text>` + "\n")
	}

	drawPanel := func(panelTitle string, panelDrivers []string, yTop int) {
		if len(panelDrivers) == 0 {
			return
		}
		axisX0 := marginLeft
		axisX1 := plotRight
		axisY1 := yTop
		axisY0 := yTop + panelH

		maxV := 0.0
		for _, d := range panelDrivers {
			for _, op := range ops {
				if byDriver[d][op] > maxV {
					maxV = byDriver[d][op]
				}
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
			svg.WriteString(`<text x="` + strconv.Itoa(axisX0-10) + `" y="` + strconv.Itoa(y+10) + `" text-anchor="end" fill="#d1d5db" font-size="30" font-family="Arial, sans-serif">` + strconv.Itoa(v) + `</text>` + "\n")
		}
		svg.WriteString(`<line x1="` + strconv.Itoa(axisX0) + `" y1="` + strconv.Itoa(axisY0) + `" x2="` + strconv.Itoa(axisX1) + `" y2="` + strconv.Itoa(axisY0) + `" stroke="#e5e7eb" stroke-width="2"/>` + "\n")
		svg.WriteString(`<line x1="` + strconv.Itoa(axisX0) + `" y1="` + strconv.Itoa(axisY0) + `" x2="` + strconv.Itoa(axisX0) + `" y2="` + strconv.Itoa(axisY1) + `" stroke="#e5e7eb" stroke-width="2"/>` + "\n")

		groupW := plotW / len(panelDrivers)
		groupGap := groupW / 6
		if groupGap < 10 {
			groupGap = 10
		}
		innerGap := groupW / 22
		if innerGap < 2 {
			innerGap = 2
		}
		usableW := groupW - groupGap
		if usableW < len(ops)*3+innerGap*4 {
			usableW = len(ops)*3 + innerGap*4
		}
		barW := (usableW - innerGap*4) / len(ops)
		barW = (barW * 85) / 100
		if barW < 3 {
			barW = 3
		}
		for i, d := range panelDrivers {
			groupX := axisX0 + i*groupW
			clusterW := len(ops)*barW + (len(ops)-1)*innerGap
			groupStart := groupX + (groupW-clusterW)/2
			for j, op := range ops {
				v := byDriver[d][op]
				h := int((v / maxV) * float64(panelH))
				if h < 1 && v > 0 {
					h = 1
				}
				y := axisY0 - h
				barX := groupStart + j*(barW+innerGap)
				svg.WriteString(`<rect x="` + strconv.Itoa(barX) + `" y="` + strconv.Itoa(y) + `" width="` + strconv.Itoa(barW) + `" height="` + strconv.Itoa(h) + `" fill="` + colors[op] + `"/>` + "\n")
			}
			labelX := axisX0 + i*groupW + groupW/2
			labelY := axisY0 + 82
			svg.WriteString(`<text x="` + strconv.Itoa(labelX) + `" y="` + strconv.Itoa(labelY) + `" text-anchor="middle" fill="#d1d5db" font-size="30" font-family="Arial, sans-serif">` + displayDriverName(d) + `</text>` + "\n")
		}
	}

	drawPanel("Outlier: "+outlier, []string{outlier}, marginTop)
	drawPanel("Remaining drivers", rest, marginTop+panelH+panelGap)

	svg.WriteString(`</svg>` + "\n")
	outPath := filepath.Join(root, "docs", "bench", fileName)
	return os.WriteFile(outPath, svg.Bytes(), 0o644)
}

func benchChartOps() []string {
	return []string{
		"get_bytes",
		"get_string",
		"get_typed_string",
		"get_typed_struct",
		"set_bytes",
		"set_string",
		"set_typed_string",
		"set_typed_struct",
		"delete",
	}
}

func benchOpLabel(op string) string {
	switch op {
	case "get_bytes":
		return "GetBytes"
	case "get_string":
		return "GetString"
	case "get_typed_string":
		return "Get[string]"
	case "get_typed_struct":
		return "Get[T]"
	case "set_bytes":
		return "SetBytes"
	case "set_string":
		return "SetString"
	case "set_typed_string":
		return "Set[string]"
	case "set_typed_struct":
		return "Set[T]"
	case "delete":
		return "Delete"
	default:
		return op
	}
}

func benchOpColors() map[string]string {
	return map[string]string{
		"get_bytes":        "#4e79a7",
		"get_string":       "#6ea6d8",
		"get_typed_string": "#9fc5e8",
		"get_typed_struct": "#cfe2f3",
		"set_bytes":        "#59a14f",
		"set_string":       "#7bc96f",
		"set_typed_string": "#a8d08d",
		"set_typed_struct": "#c6e0b4",
		"delete":           "#e15759",
	}
}

func metricPreference(yUnit string) string {
	switch yUnit {
	case "N":
		return "higher is better"
	default:
		return "lower is better"
	}
}

func displayDriverName(name string) string {
	switch name {
	case "sql_sqlite":
		return "sqlite"
	case "sql_postgres":
		return "postgres"
	case "sql_mysql":
		return "mysql"
	default:
		return name
	}
}

func saveBenchmarkRows(path string, rows map[string][]benchRow) error {
	body, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func loadBenchmarkRows(path string) (map[string][]benchRow, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows map[string][]benchRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, err
	}
	return rows, nil
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
func buildCache(ctx context.Context, name string, opts ...benchStoreOption) (*cache.Cache, func(), bool) {
	switch name {
	case "memory":
		benchCfg := applyBenchOptions(benchConfig{}, opts...)
		return cache.NewCache(cache.NewMemoryStoreWithConfig(ctx, cache.StoreConfig{
			BaseConfig:            benchCfg.BaseConfig,
			MemoryCleanupInterval: 0,
		})), func() {}, true
	case "file":
		dir, _ := os.MkdirTemp("", "cache-bench-file-*")
		benchCfg := applyBenchOptions(benchConfig{FileDir: dir}, opts...)
		return cache.NewCache(cache.NewFileStoreWithConfig(ctx, cache.StoreConfig{
			BaseConfig: benchCfg.BaseConfig,
			FileDir:    benchCfg.FileDir,
		})), func() { _ = os.RemoveAll(dir) }, true
	case "redis":
		if addr := os.Getenv("REDIS_ADDR"); addr != "" {
			client := redis.NewClient(&redis.Options{Addr: addr})
			cfg := rediscache.Config{Client: client}
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
			store := rediscache.New(cfg)
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
			cfg := memcachedcache.Config{Addresses: []string{addr}}
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
			store := memcachedcache.New(cfg)
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startMemcached(ctx, opts...); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip memcached:", err)
		}
	case "dynamodb":
		if endpoint := os.Getenv("DYNAMO_ENDPOINT"); endpoint != "" {
			store, err := newDynamoRenderStore(ctx, endpoint, opts...)
			if err != nil {
				fmt.Fprintln(os.Stderr, "benchrender: skip dynamodb:", err)
				break
			}
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startDynamo(ctx, opts...); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip dynamodb:", err)
		}
	case "sql_postgres":
		if dsn := os.Getenv("BENCH_PG_DSN"); dsn != "" {
			store, err := newSQLRenderStore("pgx", dsn, opts...)
			if err != nil {
				fmt.Fprintln(os.Stderr, "benchrender: skip postgres:", err)
				break
			}
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startPostgres(ctx, opts...); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip postgres:", err)
		}
	case "sql_mysql":
		if dsn := os.Getenv("BENCH_MYSQL_DSN"); dsn != "" {
			store, err := newSQLRenderStore("mysql", dsn, opts...)
			if err != nil {
				fmt.Fprintln(os.Stderr, "benchrender: skip mysql:", err)
				break
			}
			return cache.NewCache(store), func() {}, true
		}
		if c, cleanup, err := startMySQL(ctx, opts...); err == nil {
			return c, cleanup, true
		} else {
			fmt.Fprintln(os.Stderr, "benchrender: skip mysql:", err)
		}
	case "sql_sqlite":
		dsn := "file:" + filepath.Join(os.TempDir(), "bench.sqlite") + "?cache=shared&mode=rwc"
		store, err := newSQLRenderStore("sqlite", dsn, opts...)
		if err != nil {
			fmt.Fprintln(os.Stderr, "benchrender: skip sqlite:", err)
			break
		}
		return cache.NewCache(store), func() {}, true
	}
	return nil, nil, false
}

// --- container helpers (simplified; duplicated from bench_test) ---

func startRedis(ctx context.Context, opts ...benchStoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "redis:7-bookworm", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second)}
	c, addr, err := startContainer(ctx, req, "6379/tcp")
	if err != nil {
		return nil, nil, err
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	cfg := rediscache.Config{Client: client}
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
	store := rediscache.New(cfg)
	cleanup := func() { _ = client.Close(); _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startMemcached(ctx context.Context, opts ...benchStoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "memcached:1.6-bookworm", ExposedPorts: []string{"11211/tcp"}, WaitingFor: wait.ForListeningPort("11211/tcp").WithStartupTimeout(30 * time.Second)}
	c, addr, err := startContainer(ctx, req, "11211/tcp")
	if err != nil {
		return nil, nil, err
	}
	cfg := memcachedcache.Config{Addresses: []string{addr}}
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
	store := memcachedcache.New(cfg)
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startDynamo(ctx context.Context, opts ...benchStoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "amazon/dynamodb-local:latest", ExposedPorts: []string{"8000/tcp"}, WaitingFor: wait.ForListeningPort("8000/tcp").WithStartupTimeout(45 * time.Second)}
	c, addr, err := startContainer(ctx, req, "8000/tcp")
	if err != nil {
		return nil, nil, err
	}
	endpoint := "http://" + addr
	store, err := newDynamoRenderStore(ctx, endpoint, opts...)
	if err != nil {
		_ = c.Terminate(context.Background())
		return nil, nil, err
	}
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startPostgres(ctx context.Context, opts ...benchStoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{Image: "postgres:16-bookworm", Env: map[string]string{"POSTGRES_PASSWORD": "pass", "POSTGRES_USER": "user", "POSTGRES_DB": "app"}, ExposedPorts: []string{"5432/tcp"}, WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second)}
	c, addr, err := startContainer(ctx, req, "5432/tcp")
	if err != nil {
		return nil, nil, err
	}
	dsn := "postgres://user:pass@" + addr + "/app?sslmode=disable"
	store, err := newSQLRenderStore("pgx", dsn, opts...)
	if err != nil {
		_ = c.Terminate(context.Background())
		return nil, nil, err
	}
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startMySQL(ctx context.Context, opts ...benchStoreOption) (*cache.Cache, func(), error) {
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
	store, err := newSQLRenderStore("mysql", dsn, opts...)
	if err != nil {
		_ = c.Terminate(context.Background())
		return nil, nil, err
	}
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return cache.NewCache(store), cleanup, nil
}

func startNATS(ctx context.Context, opts ...benchStoreOption) (*cache.Cache, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "nats:2",
		Cmd:          []string{"-js"},
		ExposedPorts: []string{"4222/tcp"},
	}
	c, addr, err := startContainer(ctx, req, "4222/tcp")
	if err != nil {
		return nil, nil, err
	}
	natsURL := "nats://" + addr
	nc, err := connectNATSWithRetry(natsURL, 15*time.Second)
	if err != nil {
		_ = c.Terminate(context.Background())
		return nil, nil, err
	}
	nc.Close()
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

func newNATSBenchCache(ctx context.Context, natsURL string, opts ...benchStoreOption) (*cache.Cache, func(), error) {
	nc, err := connectNATSWithRetry(natsURL, 5*time.Second)
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

func newSQLRenderStore(driverName, dsn string, opts ...benchStoreOption) (cachecore.Store, error) {
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
	return sqlcore.New(cfg)
}

func newDynamoRenderStore(ctx context.Context, endpoint string, opts ...benchStoreOption) (cachecore.Store, error) {
	cfg := dynamocache.Config{
		Endpoint: endpoint,
		Region:   "us-east-1",
		Table:    "cache_entries",
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

func connectNATSWithRetry(url string, timeout time.Duration) (*nats.Conn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		nc, err := nats.Connect(url)
		if err == nil {
			return nc, nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("nats connect timeout")
	}
	return nil, lastErr
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
			if _, err := os.Stat(filepath.Join(cwd, "factory.go")); err == nil {
				if _, err := os.Stat(filepath.Join(cwd, "README.md")); err == nil {
					return cwd
				}
			}
		}
		if _, err := os.Stat(filepath.Join(cwd, "factory.go")); err == nil {
			if _, err := os.Stat(filepath.Join(cwd, "README.md")); err == nil {
				return cwd
			}
		}
		next := filepath.Dir(cwd)
		if next == cwd {
			panic("cache repo root not found")
		}
		cwd = next
	}
}

func applyBenchOptions(cfg benchConfig, opts ...benchStoreOption) benchConfig {
	for _, opt := range opts {
		cfg = opt(cfg)
	}
	return cfg
}
