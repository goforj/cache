package cache

import (
	"context"
	"net"
	"testing"
	"time"
)

// memcachedInMemoryDial spins up a handler per connection using a shared map.
func memcachedInMemoryDial(data map[string][]byte) func(context.Context, string, string) (net.Conn, error) {
	return func(_ context.Context, _, _ string) (net.Conn, error) {
		server, client := net.Pipe()
		go handleMemcachedConn(server, data)
		return client, nil
	}
}

func TestMemcachedStoreFullCoverage(t *testing.T) {
	data := map[string][]byte{}
	orig := dialMemcached
	dialMemcached = memcachedInMemoryDial(data)
	defer func() { dialMemcached = orig }()

	ctx := context.Background()
	store := newMemcachedStore([]string{"ignored"}, time.Second, "p")

	if err := store.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "k")
	if err != nil || !ok || string(body) != "v" {
		t.Fatalf("get failed: ok=%v err=%v val=%s", ok, err, string(body))
	}

	created, err := store.Add(ctx, "k", []byte("v2"), 0)
	if err != nil || created {
		t.Fatalf("add duplicate unexpected: created=%v err=%v", created, err)
	}

	val, err := store.Increment(ctx, "cnt", 2, 0)
	if err != nil || val != 2 {
		t.Fatalf("increment failed: val=%d err=%v", val, err)
	}
	val, err = store.Decrement(ctx, "cnt", 1, 0)
	if err != nil || val != 1 {
		t.Fatalf("decrement failed: val=%d err=%v", val, err)
	}

	if err := store.Delete(ctx, "k"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := store.DeleteMany(ctx, "cnt"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
}
