package memcachedcache

import (
	"context"
	"errors"
	"testing"

	"github.com/goforj/cache/cachecore"
)

// TestNewNilAddrErrors verifies Memcached construction rejects a nil server address.
func TestNewNilAddrErrors(t *testing.T) {
	store := New(Config{})
	if err := store.Ready(context.Background()); err == nil {
		t.Fatalf("expected ready dial error")
	}
	_, _, err := store.Get(context.Background(), "k")
	if err == nil {
		t.Fatalf("expected dial error")
	}
}

// TestNewShapingConfigFailureFailsClosed verifies Store-only construction preserves the config error.
func TestNewShapingConfigFailureFailsClosed(t *testing.T) {
	store := New(Config{BaseConfig: cachecore.BaseConfig{EncryptionKey: []byte("short")}})
	if err := store.Ready(context.Background()); !errors.Is(err, cachecore.ErrEncryptionKey) {
		t.Fatalf("Ready error = %v, want ErrEncryptionKey", err)
	}
}
