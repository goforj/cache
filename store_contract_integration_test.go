//go:build integration

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type storeFactory struct {
	name string
	new  func(t *testing.T) (Store, func())
}

func TestStoreContract_AllDrivers(t *testing.T) {
	fixtures := integrationFixtures(t)

	for _, fx := range fixtures {
		fx := fx
		t.Run(fx.name, func(t *testing.T) {
			store, cleanup := fx.new(t)
			t.Cleanup(cleanup)
			runStoreContractSuite(t, store)
		})
	}
}

func runStoreContractSuite(t *testing.T, store Store) {
	t.Helper()
	ctx := context.Background()

	ttl, wait := contractTTL(store.Driver())

	// Set/Get returns clone and round-trips.
	if err := store.Set(ctx, "alpha", []byte("value"), 500*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "alpha")
	if err != nil || !ok {
		t.Fatalf("get failed: ok=%v err=%v", ok, err)
	}
	body[0] = 'X'
	body2, ok, err := store.Get(ctx, "alpha")
	if err != nil || !ok || string(body2) != "value" {
		t.Fatalf("expected stored value unchanged, got %q err=%v", string(body2), err)
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
	if err != nil || !created {
		t.Fatalf("add first failed: created=%v err=%v", created, err)
	}
	created, err = store.Add(ctx, "once", []byte("second"), time.Second)
	if err != nil {
		t.Fatalf("add duplicate failed: %v", err)
	}
	if created {
		t.Fatalf("expected duplicate add to return created=false")
	}

	// Counters refresh TTL.
	value, err := store.Increment(ctx, "counter", 3, time.Second)
	if err != nil || value != 3 {
		t.Fatalf("increment failed: value=%d err=%v", value, err)
	}
	value, err = store.Decrement(ctx, "counter", 1, time.Second)
	if err != nil || value != 2 {
		t.Fatalf("decrement failed: value=%d err=%v", value, err)
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
}

func contractTTL(driver Driver) (ttl time.Duration, wait time.Duration) {
	switch driver {
	case DriverMemcached:
		return time.Second, 1500 * time.Millisecond
	default:
		return 50 * time.Millisecond, 80 * time.Millisecond
	}
}

func integrationFixtures(t *testing.T) []storeFactory {
	t.Helper()

	var fixtures []storeFactory

	if integrationDriverEnabled("file") {
		fixtures = append(fixtures, storeFactory{
			name: "file",
			new: func(t *testing.T) (Store, func()) {
				dir := t.TempDir()
				store := NewStore(context.Background(), StoreConfig{
					Driver:     DriverFile,
					DefaultTTL: 2 * time.Second,
					FileDir:    dir,
				})
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("memory") {
		fixtures = append(fixtures, storeFactory{
			name: "memory",
			new: func(t *testing.T) (Store, func()) {
				store := NewStore(context.Background(), StoreConfig{
					Driver:                DriverMemory,
					DefaultTTL:            2 * time.Second,
					MemoryCleanupInterval: time.Second,
				})
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
			new: func(t *testing.T) (Store, func()) {
				client := redis.NewClient(&redis.Options{Addr: addr})
				store := NewStore(context.Background(), StoreConfig{
					Driver:      DriverRedis,
					DefaultTTL:  2 * time.Second,
					Prefix:      "itest",
					RedisClient: client,
				})
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
			new: func(t *testing.T) (Store, func()) {
				store := NewStore(context.Background(), StoreConfig{
					Driver:             DriverMemcached,
					DefaultTTL:         2 * time.Second,
					Prefix:             "itest",
					MemcachedAddresses: []string{addr},
				})
				return store, func() {}
			},
		})
	}

	return fixtures
}
