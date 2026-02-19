package cache

import (
	"context"
	"testing"
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
