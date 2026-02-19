package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRedisStoreNilClientErrors(t *testing.T) {
	store := newRedisStore(nil, 0, "")
	if _, _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatalf("expected get error when redis client is nil")
	}
	if err := store.Set(context.Background(), "k", []byte("v"), 0); err == nil {
		t.Fatalf("expected set error when redis client is nil")
	}
	if err := store.Delete(context.Background(), "k"); err == nil {
		t.Fatalf("expected delete error when redis client is nil")
	}
	if _, err := store.Add(context.Background(), "k", []byte("v"), 0); err == nil {
		t.Fatalf("expected add error when redis client is nil")
	}
	if _, err := store.Increment(context.Background(), "k", 1, 0); err == nil {
		t.Fatalf("expected increment error when redis client is nil")
	}
	if _, err := store.Decrement(context.Background(), "k", 1, 0); err == nil {
		t.Fatalf("expected decrement error when redis client is nil")
	}
	if err := store.DeleteMany(context.Background(), "a", "b"); err == nil {
		t.Fatalf("expected delete many error when redis client is nil")
	}
	if err := store.Flush(context.Background()); err == nil {
		t.Fatalf("expected flush error when redis client is nil")
	}
}

func TestRedisStoreOperationsWithStubClient(t *testing.T) {
	ctx := context.Background()
	client := newStubRedisClient()
	store := newRedisStore(client, 0, "pfx")

	if err := store.Set(ctx, "alpha", []byte("one"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "alpha")
	if err != nil || !ok || string(body) != "one" {
		t.Fatalf("unexpected get result: ok=%v err=%v body=%s", ok, err, string(body))
	}

	created, err := store.Add(ctx, "alpha", []byte("two"), 0)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if created {
		t.Fatalf("expected add false when key exists")
	}

	val, err := store.Increment(ctx, "counter", 2, time.Second)
	if err != nil || val != 2 {
		t.Fatalf("increment failed: val=%d err=%v", val, err)
	}
	val, err = store.Decrement(ctx, "counter", 1, time.Second)
	if err != nil || val != 1 {
		t.Fatalf("decrement failed: val=%d err=%v", val, err)
	}
	if ttl, ok := client.ttl["pfx:counter"]; !ok || ttl.Before(time.Now().Add(900*time.Millisecond)) {
		t.Fatalf("expected expire to be set, got %v", ttl)
	}

	if val, err := store.Increment(ctx, "defaultttl", 1, 0); err != nil || val != 1 {
		t.Fatalf("increment with default ttl failed: val=%d err=%v", val, err)
	}
	if ttl, ok := client.ttl["pfx:defaultttl"]; !ok || ttl.Before(time.Now().Add(defaultCacheTTL-time.Second)) {
		t.Fatalf("expected default ttl to be applied, got %v", ttl)
	}

	if err := store.Delete(ctx, "alpha"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := store.DeleteMany(ctx); err != nil { // no-op path
		t.Fatalf("delete many empty failed: %v", err)
	}
	// delete many with keys
	if err := store.Set(ctx, "a", []byte("1"), 0); err != nil {
		t.Fatalf("set a failed: %v", err)
	}
	if err := store.Set(ctx, "b", []byte("2"), 0); err != nil {
		t.Fatalf("set b failed: %v", err)
	}
	if err := store.DeleteMany(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}

	if err := store.Set(ctx, "flushme", []byte("x"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "flushme"); err != nil || ok {
		t.Fatalf("expected flushed key to be gone")
	}
}

func TestRedisStoreGetMissing(t *testing.T) {
	ctx := context.Background()
	client := newStubRedisClient()
	store := newRedisStore(client, 0, "pfx")
	_, ok, err := store.Get(ctx, "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing key")
	}
}

func TestRedisStoreExpireError(t *testing.T) {
	ctx := context.Background()
	client := newStubRedisClient()
	client.expireErr = errors.New("fail expire")
	store := newRedisStore(client, 0, "pfx")
	if _, err := store.Increment(ctx, "k", 1, time.Second); err == nil {
		t.Fatalf("expected expire error")
	}
}

func TestRedisStoreErrorPropagation(t *testing.T) {
	ctx := context.Background()
	client := newStubRedisClient()
	client.getErr = errors.New("get")
	store := newRedisStore(client, 0, "pfx")
	if _, _, err := store.Get(ctx, "k"); err == nil {
		t.Fatalf("expected get error")
	}

	client = newStubRedisClient()
	client.setNXErr = errors.New("setnx")
	store = newRedisStore(client, 0, "pfx")
	if _, err := store.Add(ctx, "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected add error")
	}

	client = newStubRedisClient()
	client.incrErr = errors.New("incr")
	store = newRedisStore(client, 0, "pfx")
	if _, err := store.Increment(ctx, "k", 1, time.Second); err == nil {
		t.Fatalf("expected incr error")
	}

	client = newStubRedisClient()
	client.scanErr = errors.New("scan")
	store = newRedisStore(client, 0, "pfx")
	if err := store.Flush(ctx); err == nil {
		t.Fatalf("expected flush scan error")
	}

	client = newStubRedisClient()
	client.delErr = errors.New("del")
	client.store["pfx:a"] = "1"
	store = newRedisStore(client, 0, "pfx")
	if err := store.Flush(ctx); err == nil {
		t.Fatalf("expected flush delete error")
	}
}
