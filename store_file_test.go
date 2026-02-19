package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTempFileStore(t *testing.T) Store {
	t.Helper()
	dir := t.TempDir()
	return newFileStore(dir, 0)
}

func TestFileStoreSetGetDelete(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()

	body := []byte("hello")
	if err := store.Set(ctx, "alpha", body, 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body[0] = 'x' // ensure clone

	got, ok, err := store.Get(ctx, "alpha")
	if err != nil || !ok || string(got) != "hello" {
		t.Fatalf("unexpected get: ok=%v err=%v val=%s", ok, err, string(got))
	}

	if err := store.Delete(ctx, "alpha"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	_, ok, err = store.Get(ctx, "alpha")
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}
	if ok {
		t.Fatalf("expected missing after delete")
	}
}

func TestFileStoreTTLExpiry(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()

	if err := store.Set(ctx, "ttl", []byte("v"), 50*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	_, ok, err := store.Get(ctx, "ttl")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected ttl to expire")
	}
}

func TestFileStoreAddIncrementDecrement(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()

	created, err := store.Add(ctx, "once", []byte("first"), time.Minute)
	if err != nil || !created {
		t.Fatalf("add failed: created=%v err=%v", created, err)
	}
	created, err = store.Add(ctx, "once", []byte("second"), time.Minute)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if created {
		t.Fatalf("expected duplicate add to be ignored")
	}

	val, err := store.Increment(ctx, "counter", 2, time.Minute)
	if err != nil || val != 2 {
		t.Fatalf("increment failed: val=%d err=%v", val, err)
	}
	val, err = store.Decrement(ctx, "counter", 1, time.Minute)
	if err != nil || val != 1 {
		t.Fatalf("decrement failed: val=%d err=%v", val, err)
	}
}

func TestFileStoreFlushAndDeleteMany(t *testing.T) {
	store := newTempFileStore(t)
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
		t.Fatalf("expected deleted key")
	}

	if err := store.Set(ctx, "c", []byte("3"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "c"); err != nil || ok {
		t.Fatalf("expected flushed key missing")
	}
}

func TestFileStoreIncrementNonNumeric(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()

	if err := store.Set(ctx, "num", []byte("NaN"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if _, err := store.Increment(ctx, "num", 1, time.Minute); err == nil {
		t.Fatalf("expected numeric error")
	}
}

func TestFileStoreUsesSpecifiedDir(t *testing.T) {
	dir := t.TempDir()
	store := newFileStore(dir, 0)
	ctx := context.Background()
	if err := store.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one file in dir, got %d", len(entries))
	}
	if filepath.Dir(filepath.Join(dir, entries[0].Name())) != dir {
		t.Fatalf("expected file in supplied dir")
	}
}
