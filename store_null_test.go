package cache

import (
	"context"
	"testing"
	"time"
)

func TestNullStoreNoOps(t *testing.T) {
	store := newNullStore()
	ctx := context.Background()

	if err := store.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set should be nil")
	}
	if ok, err := func() (bool, error) {
		_, ok, err := store.Get(ctx, "k")
		return ok, err
	}(); err != nil || ok {
		t.Fatalf("get should miss, err=%v ok=%v", err, ok)
	}
	if created, err := store.Add(ctx, "k", []byte("v"), time.Minute); err != nil || !created {
		t.Fatalf("add should succeed, err=%v created=%v", err, created)
	}
	if val, err := store.Increment(ctx, "k", 1, time.Minute); err != nil || val != 0 {
		t.Fatalf("increment should no-op, val=%d err=%v", val, err)
	}
	if err := store.Delete(ctx, "k"); err != nil {
		t.Fatalf("delete should be nil")
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush should be nil")
	}
}
