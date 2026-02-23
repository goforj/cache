package memcachedcache

import (
	"context"
	"testing"
)

func TestNewNilAddrErrors(t *testing.T) {
	store := New(Config{})
	_, _, err := store.Get(context.Background(), "k")
	if err == nil {
		t.Fatalf("expected dial error")
	}
}
