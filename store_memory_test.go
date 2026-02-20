package cache

import (
	"context"
	"math"
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

func TestMemoryStoreIncrementInvalidNumber(t *testing.T) {
	store := newMemoryStore(0, 0)
	ctx := context.Background()
	if err := store.Set(ctx, "num", []byte("NaN"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if _, err := store.Increment(ctx, "num", 1, time.Minute); err == nil {
		t.Fatalf("expected increment to fail on invalid value")
	}
}

func TestMemoryStoreAddUsesDefaultTTLWhenMissing(t *testing.T) {
	store := newMemoryStore(defaultCacheTTL, 0).(*memoryStore)
	ctx := context.Background()
	created, err := store.Add(ctx, "ttl-default", []byte("v"), 0)
	if err != nil || !created {
		t.Fatalf("add failed: created=%v err=%v", created, err)
	}
	if _, found := store.cache.Get("ttl-default"); !found {
		t.Fatalf("expected value stored")
	}
}

func TestMemoryStoreCleanupIntervalSweeps(t *testing.T) {
	store := newMemoryStore(5*time.Millisecond, 2*time.Millisecond)
	ctx := context.Background()
	if err := store.Set(ctx, "k", []byte("v"), 5*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, ok, _ := store.Get(ctx, "k"); ok {
		t.Fatalf("expected cleanup to evict expired key")
	}
}

func TestMemoryStoreReadInt64Variants(t *testing.T) {
	ms := newMemoryStore(0, 0).(*memoryStore)

	ms.cache.Set("nonbytes", "string", time.Minute)
	if _, ok, err := ms.Get(context.Background(), "nonbytes"); err != nil {
		t.Fatalf("get failed: %v", err)
	} else if ok {
		t.Fatalf("expected ok=false for non-byte payload")
	}

	ms.cache.Set("string", "7", time.Minute)
	if val, ok, err := ms.readInt64("string"); err != nil || !ok || val != 7 {
		t.Fatalf("string parse failed: val=%d ok=%v err=%v", val, ok, err)
	}

	ms.cache.Set("uint", uint64(math.MaxInt64)+1, time.Minute)
	if _, _, err := ms.readInt64("uint"); err == nil {
		t.Fatalf("expected overflow error")
	}

	ms.cache.Set("unknown", struct{}{}, time.Minute)
	if _, _, err := ms.readInt64("unknown"); err == nil {
		t.Fatalf("expected type error")
	}

	ms.cache.Set("int", int64(9), time.Minute)
	if val, ok, err := ms.readInt64("int"); err != nil || !ok || val != 9 {
		t.Fatalf("expected int64 value, got %d ok=%v err=%v", val, ok, err)
	}

	ms.cache.Set("u8", uint8(3), time.Minute)
	if val, ok, err := ms.readInt64("u8"); err != nil || !ok || val != 3 {
		t.Fatalf("expected uint8 conversion, got %d ok=%v err=%v", val, ok, err)
	}

	ms.cache.Set("int32", int32(4), time.Minute)
	if val, ok, err := ms.readInt64("int32"); err != nil || !ok || val != 4 {
		t.Fatalf("expected int32 conversion, got %d ok=%v err=%v", val, ok, err)
	}

	ms.cache.Set("int16", int16(2), time.Minute)
	if val, ok, err := ms.readInt64("int16"); err != nil || !ok || val != 2 {
		t.Fatalf("expected int16 conversion, got %d ok=%v err=%v", val, ok, err)
	}

	ms.cache.Set("int8", int8(1), time.Minute)
	if val, ok, err := ms.readInt64("int8"); err != nil || !ok || val != 1 {
		t.Fatalf("expected int8 conversion, got %d ok=%v err=%v", val, ok, err)
	}

	ms.cache.Set("uint32", uint32(6), time.Minute)
	if val, ok, err := ms.readInt64("uint32"); err != nil || !ok || val != 6 {
		t.Fatalf("expected uint32 conversion, got %d ok=%v err=%v", val, ok, err)
	}

	ms.cache.Set("uint", uint(8), time.Minute)
	if val, ok, err := ms.readInt64("uint"); err != nil || !ok || val != 8 {
		t.Fatalf("expected uint conversion, got %d ok=%v err=%v", val, ok, err)
	}

	ms.cache.Set("uint64-ok", uint64(10), time.Minute)
	if val, ok, err := ms.readInt64("uint64-ok"); err != nil || !ok || val != 10 {
		t.Fatalf("expected uint64 conversion, got %d ok=%v err=%v", val, ok, err)
	}

	if _, ok, err := ms.readInt64("missing"); err != nil || ok {
		t.Fatalf("expected missing key to return ok=false")
	}
}
