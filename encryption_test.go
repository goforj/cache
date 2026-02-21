package cache

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
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

func TestEncryptingStoreDecryptPassthroughAndFailures(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	base := newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval)
	store, err := newEncryptingStore(base, key)
	if err != nil {
		t.Fatalf("encrypting store: %v", err)
	}
	es := store.(*encryptingStore)

	raw := []byte("plain-text")
	got, err := es.decrypt(raw)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("expected passthrough raw body, err=%v got=%q", err, string(got))
	}

	short := []byte("ENC")
	got, err = es.decrypt(short)
	if err != nil || !bytes.Equal(got, short) {
		t.Fatalf("expected passthrough short body, err=%v got=%q", err, string(got))
	}

	badPrefix := []byte("ENCXjunk")
	got, err = es.decrypt(badPrefix)
	if err != nil || !bytes.Equal(got, badPrefix) {
		t.Fatalf("expected passthrough bad prefix, err=%v got=%q", err, string(got))
	}

	badNonceLen := []byte("ENC1\x0f")
	if _, err := es.decrypt(badNonceLen); err == nil {
		t.Fatalf("expected decrypt failure for malformed nonce")
	}
}

func TestEncryptingStoreDecryptsLegacyFixture(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	base := newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval)
	store, err := newEncryptingStore(base, key)
	if err != nil {
		t.Fatalf("encrypting store: %v", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("new cipher failed: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("new gcm failed: %v", err)
	}
	nonce := bytes.Repeat([]byte{0xAB}, aead.NonceSize())
	ciphertext := aead.Seal(nil, nonce, []byte("legacy-encrypted"), nil)
	fixture := append([]byte("ENC1"), byte(len(nonce)))
	fixture = append(fixture, nonce...)
	fixture = append(fixture, ciphertext...)

	base.(*memoryStore).cache.Set("legacy", fixture, time.Minute)
	got, ok, err := store.Get(context.Background(), "legacy")
	if err != nil || !ok || string(got) != "legacy-encrypted" {
		t.Fatalf("decrypt legacy fixture failed: ok=%v err=%v val=%q", ok, err, string(got))
	}
}
