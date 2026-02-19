package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoStorePropagatesErrors(t *testing.T) {
	ctx := context.Background()
	store := NewMemoStore(&errorStore{driver: DriverMemory, err: expectedErr})

	if _, _, err := store.Get(ctx, "k"); err == nil {
		t.Fatalf("expected get error")
	}
	if err := store.Set(ctx, "k", []byte("v"), time.Minute); err == nil {
		t.Fatalf("expected set error")
	}
	if _, err := store.Add(ctx, "k", []byte("v"), time.Minute); err == nil {
		t.Fatalf("expected add error")
	}
	if _, err := store.Increment(ctx, "k", 1, time.Minute); err == nil {
		t.Fatalf("expected increment error")
	}
	if _, err := store.Decrement(ctx, "k", 1, time.Minute); err == nil {
		t.Fatalf("expected decrement error")
	}
	if err := store.Delete(ctx, "k"); err == nil {
		t.Fatalf("expected delete error")
	}
	if err := store.DeleteMany(ctx, "k"); err == nil {
		t.Fatalf("expected delete many error")
	}
	if err := store.Flush(ctx); err == nil {
		t.Fatalf("expected flush error")
	}
}
