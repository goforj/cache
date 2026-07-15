package cache

import (
	"context"
	"testing"

	"github.com/goforj/cache/cachecore"
)

// TestNewMemoryStoreWithConfig verifies the memory factory applies store configuration.
func TestNewMemoryStoreWithConfig(t *testing.T) {
	store := NewMemoryStoreWithConfig(context.Background(), StoreConfig{})
	if store.Driver() != cachecore.DriverMemory {
		t.Fatalf("expected memory store, got %q", store.Driver())
	}
}

// TestNewNullStoreWithConfig verifies the null factory accepts store configuration.
func TestNewNullStoreWithConfig(t *testing.T) {
	store := NewNullStoreWithConfig(context.Background(), StoreConfig{})
	if store.Driver() != cachecore.DriverNull {
		t.Fatalf("expected null store, got %q", store.Driver())
	}
}

// TestRequiredStoreConstructorsFailFast verifies invalid dependency wiring is rejected at construction.
func TestRequiredStoreConstructorsFailFast(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func()
	}{
		{name: "cache", call: func() { NewCache(nil) }},
		{name: "memo", call: func() { NewMemoStore(nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected constructor to panic for a nil required store")
				}
			}()
			tc.call()
		})
	}
}
