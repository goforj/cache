package cache

import (
	"context"
	"testing"
	"time"
)

func TestEncryptingStoreRoundTrip(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	base := newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval)
	store, err := newEncryptingStore(base, key)
	if err != nil {
		t.Fatalf("encrypting store: %v", err)
	}
	ctx := context.Background()
	if err := store.Set(ctx, "k", []byte("secret"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	got, ok, err := store.Get(ctx, "k")
	if err != nil || !ok || string(got) != "secret" {
		t.Fatalf("unexpected get: ok=%v err=%v val=%s", ok, err, string(got))
	}
}

func TestEncryptingStoreAddAndDecryptError(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	base := newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval)
	store, _ := newEncryptingStore(base, key)
	ctx := context.Background()
	created, err := store.Add(ctx, "once", []byte("v"), time.Minute)
	if err != nil || !created {
		t.Fatalf("add failed: %v created=%v", err, created)
	}
	// corrupt ciphertext
	base.(*memoryStore).cache.Set("once", []byte("ENC1bad"), time.Minute)
	if _, _, err := store.Get(ctx, "once"); err == nil {
		t.Fatalf("expected decrypt error")
	}
}

func TestEncryptingStoreUnsupportedKey(t *testing.T) {
	_, err := newEncryptingStore(newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval), []byte("short"))
	if err == nil {
		t.Fatalf("expected key error")
	}
}

func TestEncryptingStorePassThroughWhenDisabled(t *testing.T) {
	base := newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval)
	store, err := newEncryptingStore(base, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if store != base {
		t.Fatalf("expected identity when no key")
	}
}

func TestFactoryAppliesEncryption(t *testing.T) {
	ctx := context.Background()
	key := []byte("01234567890123456789012345678901")
	store := NewStoreWith(ctx, DriverMemory, WithEncryptionKey(key))
	if _, ok := store.(*encryptingStore); !ok {
		t.Fatalf("expected encrypting store wrapper")
	}
}
