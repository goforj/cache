package cache

import (
	"context"
	"testing"

	"github.com/goforj/cache/cachecore"
)

func TestNewMemoryStoreWithConfig(t *testing.T) {
	store := NewMemoryStoreWithConfig(context.Background(), StoreConfig{})
	if store.Driver() != cachecore.DriverMemory {
		t.Fatalf("expected memory store, got %q", store.Driver())
	}
}

func TestNewNullStoreWithConfig(t *testing.T) {
	store := NewNullStoreWithConfig(context.Background(), StoreConfig{})
	if store.Driver() != cachecore.DriverNull {
		t.Fatalf("expected null store, got %q", store.Driver())
	}
}
