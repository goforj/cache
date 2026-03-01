package natscache

import (
	"context"
	"testing"
)

func TestNewNilKeyValueErrors(t *testing.T) {
	store := New(Config{})
	ctx := context.Background()
	if err := store.Ready(ctx); err == nil {
		t.Fatalf("expected ready error when key-value is nil")
	}
	if _, _, err := store.Get(ctx, "k"); err == nil {
		t.Fatalf("expected get error when key-value is nil")
	}
}
