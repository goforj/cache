package cache

import (
	"context"
	"net"
	"testing"
	"time"
)

// Covers NOT_FOUND and NOT_STORED paths via in-memory handler.
func TestMemcachedIncrementNotFoundPath(t *testing.T) {
	orig := dialMemcached
	dialMemcached = memcachedInMemoryDial(map[string][]byte{})
	defer func() { dialMemcached = orig }()

	store := newMemcachedStore([]string{"ignored"}, time.Second, "")
	ctx := context.Background()

	// Increment missing key triggers NOT_FOUND then initializes.
	val, err := store.Increment(ctx, "cnt", 1, time.Second)
	if err != nil || val != 1 {
		t.Fatalf("increment missing failed: val=%d err=%v", val, err)
	}

	// Add existing returns NOT_STORED path.
	if ok, err := store.Add(ctx, "cnt", []byte("x"), time.Second); err != nil || ok {
		t.Fatalf("expected add duplicate to be false; ok=%v err=%v", ok, err)
	}
}

// helper shared with coverage tests
func memcachedInMemoryDial(data map[string][]byte) func(context.Context, string, string) (net.Conn, error) {
	return func(_ context.Context, _, _ string) (net.Conn, error) {
		server, client := net.Pipe()
		go handleMemcachedConn(server, data)
		return client, nil
	}
}

func TestMemcachedIncrementNegativeDelta(t *testing.T) {
	data := map[string][]byte{}
	orig := dialMemcached
	dialMemcached = memcachedInMemoryDial(data)
	defer func() { dialMemcached = orig }()

	store := newMemcachedStore([]string{"ignored"}, time.Second, "p")
	ctx := context.Background()

	if _, err := store.Increment(ctx, "n", -2, time.Second); err != nil {
		t.Fatalf("negative increment via decrement failed: %v", err)
	}
}

func TestMemcachedDeleteManyEmpty(t *testing.T) {
	store := newMemcachedStore(nil, time.Second, "p")
	if err := store.DeleteMany(context.Background()); err != nil {
		t.Fatalf("empty delete many should be nil: %v", err)
	}
}

func TestMemcachedCacheKeyEmptyPrefix(t *testing.T) {
	ms := &memcachedStore{prefix: ""}
	if ms.cacheKey("k") != "k" {
		t.Fatalf("expected raw key when prefix empty")
	}
}

func TestMemcachedAddSuccessPath(t *testing.T) {
	data := map[string][]byte{}
	orig := dialMemcached
	dialMemcached = memcachedInMemoryDial(data)
	defer func() { dialMemcached = orig }()

	store := newMemcachedStore([]string{"ignored"}, time.Second, "p")
	created, err := store.Add(context.Background(), "fresh", []byte("v"), time.Second)
	if err != nil || !created {
		t.Fatalf("expected add success, created=%v err=%v", created, err)
	}
}

func TestMemcachedIncrementDefaultsTTL(t *testing.T) {
	data := map[string][]byte{}
	orig := dialMemcached
	dialMemcached = memcachedInMemoryDial(data)
	defer func() { dialMemcached = orig }()

	store := newMemcachedStore([]string{"ignored"}, time.Second, "p")
	if _, err := store.Increment(context.Background(), "ttl", 1, 0); err != nil {
		t.Fatalf("increment with default ttl failed: %v", err)
	}
}
