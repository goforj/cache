package cache

import (
	"context"
	"testing"
)

func TestMemcachedStoreNilAddrErrors(t *testing.T) {
	store := newMemcachedStore(nil, 0, "")
	_, _, err := store.Get(context.Background(), "k")
	if err == nil {
		t.Fatalf("expected dial error")
	}
}

// Integration coverage for memcached is handled in store_contract_integration_test via testcontainers.
