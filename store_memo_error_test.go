package cache

import (
	"context"
	"testing"
	"time"
)

type errorStore struct{ err error }

func (e errorStore) Driver() Driver                                    { return DriverMemory }
func (e errorStore) Get(context.Context, string) ([]byte, bool, error) { return nil, false, e.err }
func (e errorStore) Set(context.Context, string, []byte, time.Duration) error {
	return e.err
}
func (e errorStore) Add(context.Context, string, []byte, time.Duration) (bool, error) {
	return false, e.err
}
func (e errorStore) Increment(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, e.err
}
func (e errorStore) Decrement(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, e.err
}
func (e errorStore) Delete(context.Context, string) error        { return e.err }
func (e errorStore) DeleteMany(context.Context, ...string) error { return e.err }
func (e errorStore) Flush(context.Context) error                 { return e.err }

func TestMemoStorePropagatesErrors(t *testing.T) {
	ctx := context.Background()
	store := NewMemoStore(errorStore{err: expectedErr})

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
