package natscache

import (
	"context"
	"errors"
	"testing"

	"github.com/goforj/cache/cachecore"
)

// TestNewNilKeyValueErrors verifies NATS construction rejects a nil key-value bucket.
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

// TestNewShapingConfigFailureFailsClosed verifies Store-only construction preserves the config error.
func TestNewShapingConfigFailureFailsClosed(t *testing.T) {
	store := New(Config{BaseConfig: cachecore.BaseConfig{EncryptionKey: []byte("short")}})
	if err := store.Ready(context.Background()); !errors.Is(err, cachecore.ErrEncryptionKey) {
		t.Fatalf("Ready error = %v, want ErrEncryptionKey", err)
	}
}
