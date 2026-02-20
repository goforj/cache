package cache

import (
	"context"
	"testing"
)

func TestNewStoreMemory(t *testing.T) {
	store := NewStore(context.Background(), StoreConfig{Driver: DriverMemory})
	if store.Driver() != DriverMemory {
		t.Fatalf("expected memory store, got %q", store.Driver())
	}
}

func TestNewStoreRedis(t *testing.T) {
	store := NewStore(context.Background(), StoreConfig{Driver: DriverRedis})
	if store.Driver() != DriverRedis {
		t.Fatalf("expected redis store, got %q", store.Driver())
	}
}

func TestNewStoreNATS(t *testing.T) {
	store := NewStore(context.Background(), StoreConfig{
		Driver:       DriverNATS,
		NATSKeyValue: newStubNATSKeyValue("bucket"),
	})
	if store.Driver() != DriverNATS {
		t.Fatalf("expected nats store, got %q", store.Driver())
	}
}
