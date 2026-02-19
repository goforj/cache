package cache

import (
	"context"
	"testing"
	"time"
)

func newSQLiteStore(t *testing.T) Store {
	t.Helper()
	dsn := "file::memory:?cache=shared"
	store, err := newSQLStore(StoreConfig{
		SQLDriverName: "sqlite",
		SQLDSN:        dsn,
		SQLTable:      "cache_entries",
		DefaultTTL:    time.Second,
		Prefix:        "p",
	})
	if err != nil {
		t.Fatalf("sqlite store create failed: %v", err)
	}
	return store
}

func TestSQLStoreBasics(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()

	if err := store.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "k")
	if err != nil || !ok || string(body) != "v" {
		t.Fatalf("get failed: ok=%v err=%v val=%s", ok, err, string(body))
	}

	created, err := store.Add(ctx, "k", []byte("v2"), time.Minute)
	if err != nil || created {
		t.Fatalf("add duplicate unexpected: created=%v err=%v", created, err)
	}

	val, err := store.Increment(ctx, "n", 2, time.Minute)
	if err != nil || val != 2 {
		t.Fatalf("increment failed: %v val=%d", err, val)
	}
	val, err = store.Decrement(ctx, "n", 1, time.Minute)
	if err != nil || val != 1 {
		t.Fatalf("decrement failed: %v val=%d", err, val)
	}

	if err := store.Delete(ctx, "k"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := store.DeleteMany(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
}

func TestSQLStoreTTLExpiry(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()
	if err := store.Set(ctx, "ttl", []byte("x"), 50*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	_, ok, err := store.Get(ctx, "ttl")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected ttl expiry")
	}
}
