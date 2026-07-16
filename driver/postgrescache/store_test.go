package postgrescache

import (
	"errors"
	"testing"

	"github.com/goforj/cache/cachecore"
)

// TestNewRequiresDSN verifies PostgreSQL construction rejects an empty data source name.
func TestNewRequiresDSN(t *testing.T) {
	store, err := New(Config{})
	if err == nil {
		t.Fatalf("expected error for missing dsn")
	}
	if store != nil {
		t.Fatalf("expected nil store on error")
	}
}

// TestNewForwardsShapingConfiguration verifies validation occurs before database setup.
func TestNewForwardsShapingConfiguration(t *testing.T) {
	store, err := New(Config{BaseConfig: cachecore.BaseConfig{EncryptionKey: []byte("short")}})
	if !errors.Is(err, cachecore.ErrEncryptionKey) || store != nil {
		t.Fatalf("New = (%v, %v), want nil and ErrEncryptionKey", store, err)
	}
}
