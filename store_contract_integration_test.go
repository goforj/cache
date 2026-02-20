//go:build integration

package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type storeFactory struct {
	name string
	new  func(t *testing.T, opts ...StoreOption) (Store, func())
}

type contractCase struct {
	name                   string
	opts                   []StoreOption
	verifyDefaultTTLExpiry bool
	verifyMaxValueLimit    bool
}

func TestStoreContract_AllDrivers(t *testing.T) {
	fixtures := integrationFixtures(t)
	cases := integrationContractCases()

	for _, fx := range fixtures {
		fx := fx
		t.Run(fx.name, func(t *testing.T) {
			for _, tc := range cases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					store, cleanup := fx.new(t, tc.opts...)
					t.Cleanup(cleanup)
					runStoreContractSuite(t, store, tc)
				})
			}
		})
	}
}

func runStoreContractSuite(t *testing.T, store Store, tc contractCase) {
	t.Helper()
	ctx := context.Background()
	noOp := store.Driver() == DriverNull

	// Memcached flush_all semantics can briefly affect keys written in the same
	// second as a prior flush in a previous subtest.
	if store.Driver() == DriverMemcached && tc.name != "baseline" {
		time.Sleep(1100 * time.Millisecond)
	}

	ttl, wait := contractTTL(store.Driver())

	// Set/Get returns clone and round-trips.
	if err := store.Set(ctx, "alpha", []byte("value"), 500*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "alpha")
	if err != nil {
		t.Fatalf("get failed: ok=%v err=%v", ok, err)
	}
	if noOp {
		if ok {
			t.Fatalf("expected null store miss on get")
		}
	} else {
		if !ok {
			t.Fatalf("expected key hit on get")
		}
		body[0] = 'X'
		body2, ok, err := store.Get(ctx, "alpha")
		if err != nil || !ok || string(body2) != "value" {
			t.Fatalf("expected stored value unchanged, got %q err=%v", string(body2), err)
		}
	}

	// TTL expiry.
	if err := store.Set(ctx, "ttl", []byte("v"), ttl); err != nil {
		t.Fatalf("set ttl failed: %v", err)
	}
	time.Sleep(wait)
	if _, ok, err := store.Get(ctx, "ttl"); err != nil || ok {
		t.Fatalf("expected ttl key expired; ok=%v err=%v", ok, err)
	}

	// Add only when missing.
	created, err := store.Add(ctx, "once", []byte("first"), time.Second)
	if err != nil {
		t.Fatalf("add first failed: created=%v err=%v", created, err)
	}
	created, err = store.Add(ctx, "once", []byte("second"), time.Second)
	if err != nil {
		t.Fatalf("add duplicate failed: %v", err)
	}
	if noOp {
		if !created {
			t.Fatalf("expected null add to report created=true")
		}
	} else if created {
		t.Fatalf("expected duplicate add to return created=false")
	}

	// Counters refresh TTL.
	value, err := store.Increment(ctx, "counter", 3, time.Second)
	if err != nil {
		t.Fatalf("increment failed: value=%d err=%v", value, err)
	}
	if noOp {
		if value != 0 {
			t.Fatalf("expected null increment to return 0, got %d", value)
		}
	} else if value != 3 {
		t.Fatalf("expected incremented value to be 3, got %d", value)
	}
	value, err = store.Decrement(ctx, "counter", 1, time.Second)
	if err != nil {
		t.Fatalf("decrement failed: value=%d err=%v", value, err)
	}
	if noOp {
		if value != 0 {
			t.Fatalf("expected null decrement to return 0, got %d", value)
		}
	} else if value != 2 {
		t.Fatalf("expected decremented value to be 2, got %d", value)
	}

	// Delete & DeleteMany.
	if err := store.Set(ctx, "a", []byte("1"), time.Second); err != nil {
		t.Fatalf("set a failed: %v", err)
	}
	if err := store.Set(ctx, "b", []byte("2"), time.Second); err != nil {
		t.Fatalf("set b failed: %v", err)
	}
	if err := store.Delete(ctx, "a"); err != nil {
		t.Fatalf("delete a failed: %v", err)
	}
	if err := store.DeleteMany(ctx, "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "a"); err != nil || ok {
		t.Fatalf("expected key a deleted")
	}
	if _, ok, err := store.Get(ctx, "b"); err != nil || ok {
		t.Fatalf("expected key b deleted")
	}

	// Flush clears all keys.
	if err := store.Set(ctx, "flush", []byte("x"), time.Second); err != nil {
		t.Fatalf("set flush failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "flush"); err != nil || ok {
		t.Fatalf("expected flush to clear key; ok=%v err=%v", ok, err)
	}

	// Typed remember across drivers.
	type payload struct{ Name string `json:"name"` }
	cache := NewCache(store)
	calls := 0
	val, err := Remember[payload](cache, "remember:typed", time.Minute, func() (payload, error) {
		calls++
		return payload{Name: "Ada"}, nil
	})
	if err != nil || val.Name != "Ada" {
		t.Fatalf("remember typed failed: %+v err=%v", val, err)
	}
	// Cached path should bypass callback.
	val, err = Remember[payload](cache, "remember:typed", time.Minute, func() (payload, error) {
		calls++
		return payload{Name: "Other"}, nil
	})
	if err != nil {
		t.Fatalf("remember typed second call failed: %+v err=%v", val, err)
	}
	if noOp {
		if calls != 2 || val.Name != "Other" {
			t.Fatalf("expected null remember to recompute, calls=%d val=%+v", calls, val)
		}
	} else if calls != 1 || val.Name != "Ada" {
		t.Fatalf("remember typed cache miss: calls=%d val=%+v err=%v", calls, val, err)
	}

	if tc.verifyDefaultTTLExpiry {
		if err := store.Set(ctx, "default_ttl", []byte("v"), 0); err != nil {
			t.Fatalf("set default ttl key failed: %v", err)
		}
		time.Sleep(defaultTTLWait(store.Driver()))
		if _, ok, err := store.Get(ctx, "default_ttl"); err != nil || ok {
			t.Fatalf("expected default ttl key expired; ok=%v err=%v", ok, err)
		}
	}

	if tc.verifyMaxValueLimit {
		tooLarge := []byte("abcdefghijklmnopqrstuvwxyz0123456789")
		err := store.Set(ctx, "too-large", tooLarge, time.Second)
		if !errors.Is(err, ErrValueTooLarge) {
			t.Fatalf("expected ErrValueTooLarge, got %v", err)
		}
	}
}

func contractTTL(driver Driver) (ttl time.Duration, wait time.Duration) {
	switch driver {
	case DriverMemcached:
		return time.Second, 1500 * time.Millisecond
	default:
		return 50 * time.Millisecond, 80 * time.Millisecond
	}
}

func defaultTTLWait(driver Driver) time.Duration {
	switch driver {
	case DriverMemcached:
		return 1500 * time.Millisecond
	default:
		return 120 * time.Millisecond
	}
}

func integrationContractCases() []contractCase {
	encryptionKey := []byte("0123456789abcdef0123456789abcdef")
	return []contractCase{
		{name: "baseline"},
		{
			name: "with_prefix",
			opts: []StoreOption{WithPrefix("itest_opt")},
		},
		{
			name: "with_compression",
			opts: []StoreOption{WithCompression(CompressionGzip)},
		},
		{
			name: "with_encryption",
			opts: []StoreOption{WithEncryptionKey(encryptionKey)},
		},
		{
			name: "with_max_value_bytes",
			opts: []StoreOption{WithMaxValueBytes(16)},
			verifyMaxValueLimit: true,
		},
		{
			name: "with_default_ttl",
			opts: []StoreOption{WithDefaultTTL(60 * time.Millisecond)},
			verifyDefaultTTLExpiry: true,
		},
	}
}

func applyStoreOptions(cfg StoreConfig, opts ...StoreOption) StoreConfig {
	for _, opt := range opts {
		cfg = opt(cfg)
	}
	return cfg
}

func integrationFixtures(t *testing.T) []storeFactory {
	t.Helper()

	var fixtures []storeFactory

	if integrationDriverEnabled("null") {
		fixtures = append(fixtures, storeFactory{
			name: "null",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				store := NewNullStore(context.Background(), opts...)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("file") {
		fixtures = append(fixtures, storeFactory{
			name: "file",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				dir := t.TempDir()
				cfg := applyStoreOptions(StoreConfig{
					Driver:     DriverFile,
					DefaultTTL: 2 * time.Second,
					FileDir:    dir,
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("memory") {
		fixtures = append(fixtures, storeFactory{
			name: "memory",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				cfg := applyStoreOptions(StoreConfig{
					Driver:                DriverMemory,
					DefaultTTL:            2 * time.Second,
					MemoryCleanupInterval: time.Second,
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("redis") {
		addr := integrationAddr("redis")
		if addr == "" {
			t.Fatalf("redis integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "redis",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				client := redis.NewClient(&redis.Options{Addr: addr})
				cfg := applyStoreOptions(StoreConfig{
					Driver:      DriverRedis,
					DefaultTTL:  2 * time.Second,
					Prefix:      "itest",
					RedisClient: client,
				}, opts...)
				store := NewStore(context.Background(), cfg)
				cleanup := func() { _ = client.Close() }
				return store, cleanup
			},
		})
	}

	if integrationDriverEnabled("memcached") {
		addr := integrationAddr("memcached")
		if addr == "" {
			t.Fatalf("memcached integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "memcached",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				cfg := applyStoreOptions(StoreConfig{
					Driver:             DriverMemcached,
					DefaultTTL:         2 * time.Second,
					Prefix:             "itest",
					MemcachedAddresses: []string{addr},
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("dynamodb") {
		endpoint := integrationAddr("dynamodb")
		if endpoint == "" {
			t.Fatalf("dynamodb integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "dynamodb",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				cfg := applyStoreOptions(StoreConfig{
					Driver:       DriverDynamo,
					DefaultTTL:   2 * time.Second,
					Prefix:       "itest",
					DynamoEndpoint: endpoint,
					DynamoRegion: "us-east-1",
					DynamoTable:  "cache_entries",
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("sql_sqlite") {
		fixtures = append(fixtures, storeFactory{
			name: "sql_sqlite",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				cfg := applyStoreOptions(StoreConfig{
					Driver:        DriverSQL,
					DefaultTTL:    2 * time.Second,
					SQLDriverName: "sqlite",
					SQLDSN:        "file::memory:?cache=shared",
					SQLTable:      "cache_entries",
					Prefix:        "itest",
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("sql_postgres") {
		addr := integrationAddr("sql_postgres")
		if addr == "" {
			t.Fatalf("sql_postgres integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "sql_postgres",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				dsn := "postgres://user:pass@" + addr + "/app?sslmode=disable"
				cfg := applyStoreOptions(StoreConfig{
					Driver:        DriverSQL,
					DefaultTTL:    2 * time.Second,
					SQLDriverName: "pgx",
					SQLDSN:        dsn,
					SQLTable:      "cache_entries",
					Prefix:        "itest",
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("sql_mysql") {
		addr := integrationAddr("sql_mysql")
		if addr == "" {
			t.Fatalf("sql_mysql integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "sql_mysql",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				dsn := "user:pass@tcp(" + addr + ")/app?parseTime=true"
				cfg := applyStoreOptions(StoreConfig{
					Driver:        DriverSQL,
					DefaultTTL:    2 * time.Second,
					SQLDriverName: "mysql",
					SQLDSN:        dsn,
					SQLTable:      "cache_entries",
					Prefix:        "itest",
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	return fixtures
}
