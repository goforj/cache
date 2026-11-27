//go:build integration

package all

import (
	"context"
	"database/sql"
	"io"
	"testing"
	"time"

	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/memcachedcache"
	"github.com/goforj/cache/driver/mysqlcache"
	"github.com/goforj/cache/driver/natscache"
	"github.com/goforj/cache/driver/postgrescache"
	"github.com/goforj/cache/driver/sqlitecache"
	"github.com/nats-io/nats.go"
	"github.com/testcontainers/testcontainers-go"
)

// TestSQLReadiness verifies literal namespaces on every SQL engine and concurrent PostgreSQL initialization.
func TestSQLReadiness(t *testing.T) {
	ctx := context.Background()
	if integrationDriverEnabled("sqlitecache") {
		t.Run("sqlite", func(t *testing.T) {
			dsn := "file:" + t.TempDir() + "/cache.sqlite"
			checkSQLNamespaces(t, func(cfg cachecore.BaseConfig) (cachecore.Store, error) {
				return sqlitecache.New(sqlitecache.Config{BaseConfig: cfg, DSN: dsn})
			})
		})
	}
	if integrationDriverEnabled("mysqlcache") {
		t.Run("mysql", func(t *testing.T) {
			container, addr := startMySQLContainer(t, ctx)
			t.Cleanup(func() { terminateReadinessContainer(t, container) })
			dsn := "user:pass@tcp(" + addr + ")/app?parseTime=true"
			checkSQLNamespaces(t, func(cfg cachecore.BaseConfig) (cachecore.Store, error) {
				return mysqlcache.New(mysqlcache.Config{BaseConfig: cfg, DSN: dsn})
			})
		})
	}
	if integrationDriverEnabled("postgrescache") {
		t.Run("postgres", func(t *testing.T) {
			container, addr := startPostgresContainer(t, ctx)
			t.Cleanup(func() { terminateReadinessContainer(t, container) })
			dsn := "postgres://user:pass@" + addr + "/app?sslmode=disable"
			checkSQLNamespaces(t, func(cfg cachecore.BaseConfig) (cachecore.Store, error) {
				return postgrescache.New(postgrescache.Config{BaseConfig: cfg, DSN: dsn})
			})
			checkPostgresCounterInitialization(t, dsn)
		})
	}
}

// checkSQLNamespaces verifies flush and inspection both enforce literal prefix boundaries.
func checkSQLNamespaces(t *testing.T, open func(cachecore.BaseConfig) (cachecore.Store, error)) {
	t.Helper()
	ctx := context.Background()
	newStore := func(prefix string) cachecore.Store {
		store, err := retryStoreInit(5*time.Second, 100*time.Millisecond, func() (cachecore.Store, error) {
			return open(cachecore.BaseConfig{Prefix: prefix, Compression: cachecore.CompressionGzip})
		})
		if err != nil {
			t.Fatal(err)
		}
		closer := store.(io.Closer)
		t.Cleanup(func() { _ = closer.Close() })
		if err := store.Set(ctx, "item", []byte("value"), time.Minute); err != nil {
			t.Fatal(err)
		}
		return store
	}
	owned := newStore("cache_%")
	neighbor := newStore("cache_x")
	caseVariant := newStore("CACHE_%")
	entries, err := owned.(cachecore.Inspector).ListPage(ctx, cachecore.ListPageOptions{})
	if err != nil || len(entries.Entries) != 1 || entries.Entries[0].Key != "item" {
		t.Fatalf("own scope entries = %+v, err = %v", entries.Entries, err)
	}
	if err := owned.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		store cachecore.Store
		hit   bool
	}{{owned, false}, {neighbor, true}, {caseVariant, true}} {
		if _, hit, err := check.store.Get(ctx, "item"); err != nil || hit != check.hit {
			t.Fatalf("Get after Flush: hit=%v want=%v err=%v", hit, check.hit, err)
		}
	}
}

// checkPostgresCounterInitialization blocks both counter writes at the database before either can commit.
func checkPostgresCounterInitialization(t *testing.T, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := postgrescache.New(postgrescache.Config{DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer store.(io.Closer).Close()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const setup = `CREATE FUNCTION pause_cache_insert() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_advisory_xact_lock(784521); RETURN NEW; END $$;
		CREATE TRIGGER pause_insert BEFORE INSERT ON cache_entries FOR EACH ROW EXECUTE FUNCTION pause_cache_insert();`
	if _, err := db.ExecContext(ctx, setup); err != nil {
		t.Fatal(err)
	}
	gate, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback()
	if _, err := gate.ExecContext(ctx, "SELECT pg_advisory_xact_lock(784521)"); err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		value int64
		err   error
	}
	results := make(chan outcome, 2)
	for range cap(results) {
		go func() {
			value, err := store.Increment(ctx, "new-counter", 1, time.Minute)
			results <- outcome{value, err}
		}()
	}
	for {
		var waiting int
		err := db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE wait_event = 'advisory' AND query LIKE 'INSERT INTO cache_entries%'").Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting == cap(results) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := gate.Commit(); err != nil {
		t.Fatal(err)
	}

	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.value+second.value != 3 ||
		min(first.value, second.value) != 1 {
		t.Fatalf("concurrent increments = %+v, %+v; want 1 and 2", first, second)
	}
}

// TestMemcachedPeerCoordination verifies two clients agree on the lock owner and Flush reaches both peers.
func TestMemcachedPeerCoordination(t *testing.T) {
	if !integrationDriverEnabled("memcachedcache") {
		t.Skip("Memcached is outside the selected driver matrix")
	}
	ctx := context.Background()
	first, addr1 := startMemcachedContainer(t, ctx)
	t.Cleanup(func() { terminateReadinessContainer(t, first) })
	second, addr2 := startMemcachedContainer(t, ctx)
	t.Cleanup(func() { terminateReadinessContainer(t, second) })
	cluster := memcachedcache.New(memcachedcache.Config{Addresses: []string{addr1, addr2}, AllowServerFlush: true})
	peer1 := memcachedcache.New(memcachedcache.Config{Addresses: []string{addr1}})
	peer2 := memcachedcache.New(memcachedcache.Config{Addresses: []string{addr2}})
	reversed := memcachedcache.New(memcachedcache.Config{Addresses: []string{addr2, addr1}})
	for _, store := range []cachecore.Store{cluster, peer1, peer2, reversed} {
		t.Cleanup(func() { _ = store.(io.Closer).Close() })
	}
	owner := cache.NewCache(cluster).NewLockHandle("shared", time.Minute)
	contender := cache.NewCache(reversed).NewLockHandle("shared", time.Minute)
	if held, err := owner.Acquire(); err != nil || !held {
		t.Fatalf("owner acquire = %v, %v", held, err)
	}
	if held, err := contender.Acquire(); err != nil || held {
		t.Fatalf("contender acquire = %v, %v", held, err)
	}
	if err := owner.Release(); err != nil {
		t.Fatal(err)
	}
	if held, err := contender.Acquire(); err != nil || !held {
		t.Fatalf("contender acquire after release = %v, %v", held, err)
	}
	for _, peer := range []cachecore.Store{peer1, peer2} {
		if err := peer.Set(ctx, "item", []byte("value"), time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := cluster.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	for _, peer := range []cachecore.Store{peer1, peer2} {
		if _, hit, err := peer.Get(ctx, "item"); err != nil || hit {
			t.Fatalf("peer Get after Flush: hit=%v err=%v", hit, err)
		}
	}
}

type replacingNATSBucket struct {
	nats.KeyValue
	replace func(key string) error
}

// Get stages a concurrent replacement after JetStream has returned the obsolete revision.
func (kv *replacingNATSBucket) Get(key string) (nats.KeyValueEntry, error) {
	entry, err := kv.KeyValue.Get(key)
	if err != nil || kv.replace == nil {
		return entry, err
	}
	replace := kv.replace
	kv.replace = nil
	return entry, replace(key)
}

// TestNATSExpiredRevisionCoordination verifies reads and counters preserve concurrent JetStream writes.
func TestNATSExpiredRevisionCoordination(t *testing.T) {
	if !integrationDriverEnabled("natscache") {
		t.Skip("NATS is outside the selected driver matrix")
	}
	ctx := context.Background()
	container, addr := startNATSContainer(t, ctx)
	t.Cleanup(func() { terminateReadinessContainer(t, container) })
	conn, err := nats.Connect("nats://" + addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Close)
	js, err := conn.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "readiness"})
	if err != nil {
		t.Fatal(err)
	}
	bucket := &replacingNATSBucket{KeyValue: kv}
	store := natscache.New(natscache.Config{KeyValue: bucket})
	for _, key := range []string{"value", "counter"} {
		if err := store.Set(ctx, key, []byte("4"), time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(5 * time.Millisecond)
	replace := func(key string) error {
		_, err := kv.Put(key, []byte("10"))
		return err
	}
	bucket.replace = replace
	if _, hit, err := store.Get(ctx, "value"); err != nil || hit {
		t.Fatalf("expired Get: hit=%v err=%v", hit, err)
	}
	if value, hit, err := store.Get(ctx, "value"); err != nil || !hit || string(value) != "10" {
		t.Fatalf("newer Get: value=%q hit=%v err=%v", value, hit, err)
	}
	bucket.replace = replace
	if value, err := store.Increment(ctx, "counter", 1, time.Minute); err != nil || value != 11 {
		t.Fatalf("Increment after replacement = %d, %v; want 11", value, err)
	}
}

// terminateReadinessContainer bounds teardown even when a test operation has timed out.
func terminateReadinessContainer(t *testing.T, container testcontainers.Container) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := container.Terminate(ctx); err != nil {
		t.Errorf("terminate integration container: %v", err)
	}
}
