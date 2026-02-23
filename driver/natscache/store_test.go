package natscache

import (
	"context"
	"testing"
)

func TestNewNilKeyValueErrors(t *testing.T) {
	store := New(Config{})
	ctx := context.Background()
	if _, _, err := store.Get(ctx, "k"); err == nil {
		t.Fatalf("expected get error when key-value is nil")
	}
}
