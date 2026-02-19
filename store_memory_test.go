package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreSetGetDelete(t *testing.T) {
	store := newMemoryStore(0, 0)

	key := "alpha"
	body := []byte("hello")
	if err := store.Set(context.Background(), key, body, 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	body[0] = 'x'

	got, ok, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected value in cache")
	}
	if string(got) != "hello" {
		t.Fatalf("expected cached clone to be unchanged, got %q", got)
	}

	if err := store.Delete(context.Background(), key); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	_, ok, err = store.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}
	if ok {
		t.Fatalf("expected deleted key to be missing")
	}
}

func TestMemoryStoreHonorsExplicitTTL(t *testing.T) {
	store := newMemoryStore(0, 0)
	if err := store.Set(context.Background(), "ttl-key", []byte("value"), 50*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	_, ok, err := store.Get(context.Background(), "ttl-key")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected ttl-key to expire")
	}
}

func TestMemoryStoreAddAndNumericOperations(t *testing.T) {
	store := newMemoryStore(0, 0)
	ctx := context.Background()

	created, err := store.Add(ctx, "once", []byte("first"), time.Minute)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if !created {
		t.Fatalf("expected key creation")
	}
	created, err = store.Add(ctx, "once", []byte("second"), time.Minute)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if created {
		t.Fatalf("expected duplicate add to be ignored")
	}

	value, err := store.Increment(ctx, "counter", 5, time.Minute)
	if err != nil {
		t.Fatalf("increment failed: %v", err)
	}
	if value != 5 {
		t.Fatalf("expected value=5, got %d", value)
	}

	value, err = store.Decrement(ctx, "counter", 2, time.Minute)
	if err != nil {
		t.Fatalf("decrement failed: %v", err)
	}
	if value != 3 {
		t.Fatalf("expected value=3, got %d", value)
	}
}

func TestMemoryStoreDeleteManyAndFlush(t *testing.T) {
	store := newMemoryStore(0, 0)
	ctx := context.Background()

	if err := store.Set(ctx, "a", []byte("1"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.Set(ctx, "b", []byte("2"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.DeleteMany(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "a"); err != nil || ok {
		t.Fatalf("expected key a removed")
	}

	if err := store.Set(ctx, "c", []byte("3"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "c"); err != nil || ok {
		t.Fatalf("expected key c removed after flush")
	}
}
