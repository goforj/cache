package natscache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/nats-io/nats.go"
)

type lockKeyValue struct {
	value    []byte
	revision uint64
}

// Status reports a healthy test bucket.
func (k *lockKeyValue) Status() (nats.KeyValueStatus, error) { return nil, nil }

// Get returns the current test entry.
func (k *lockKeyValue) Get(key string) (nats.KeyValueEntry, error) {
	if k.value == nil {
		return nil, nats.ErrKeyNotFound
	}
	return lockKeyValueEntry{key: key, value: k.value, revision: k.revision}, nil
}

// Put replaces the current test value.
func (k *lockKeyValue) Put(_ string, value []byte) (uint64, error) {
	k.revision++
	k.value = append([]byte(nil), value...)
	return k.revision, nil
}

// Create records the value only when the test key is absent.
func (k *lockKeyValue) Create(_ string, value []byte) (uint64, error) {
	if k.value != nil {
		return 0, nats.ErrKeyExists
	}
	return k.Put("", value)
}

// Update replaces the value only at the expected revision.
func (k *lockKeyValue) Update(_ string, value []byte, revision uint64) (uint64, error) {
	if revision != k.revision {
		return 0, nats.ErrKeyExists
	}
	return k.Put("", value)
}

// Delete removes the current test value.
func (k *lockKeyValue) Delete(_ string, _ ...nats.DeleteOpt) error {
	if k.value == nil {
		return nats.ErrKeyNotFound
	}
	k.revision++
	k.value = nil
	return nil
}

// Purge removes the current test value.
func (k *lockKeyValue) Purge(key string, opts ...nats.DeleteOpt) error {
	return k.Delete(key, opts...)
}

// ListKeys is unused by lock tests.
func (k *lockKeyValue) ListKeys(...nats.WatchOpt) (nats.KeyLister, error) { return nil, nil }

type lockKeyValueEntry struct {
	key      string
	value    []byte
	revision uint64
}

// Bucket identifies the test bucket.
func (e lockKeyValueEntry) Bucket() string { return "test" }

// Key returns the test key.
func (e lockKeyValueEntry) Key() string { return e.key }

// Value returns an owned test value.
func (e lockKeyValueEntry) Value() []byte { return append([]byte(nil), e.value...) }

// Revision returns the test revision.
func (e lockKeyValueEntry) Revision() uint64 { return e.revision }

// Created returns a stable test timestamp.
func (e lockKeyValueEntry) Created() time.Time { return time.Time{} }

// Delta reports no pending revisions.
func (e lockKeyValueEntry) Delta() uint64 { return 0 }

// Operation reports a normal value write.
func (e lockKeyValueEntry) Operation() nats.KeyValueOp { return nats.KeyValuePut }

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

// TestLockReleaseRequiresCurrentOwner verifies NATS never deletes a successor's lock value.
func TestLockReleaseRequiresCurrentOwner(t *testing.T) {
	kv := &lockKeyValue{}
	store := &store{kv: kv, prefix: "p", defaultTTL: time.Minute, bucketTTL: true}
	ctx := context.Background()
	owner := []byte("owner")
	if acquired, err := store.LockAcquire(ctx, "lock:key", owner, time.Minute); err != nil || !acquired {
		t.Fatalf("LockAcquire() = %v, %v", acquired, err)
	}
	if released, err := store.LockRelease(ctx, "lock:key", []byte("stale")); err != nil || released {
		t.Fatalf("stale LockRelease() = %v, %v", released, err)
	}
	if released, err := store.LockRelease(ctx, "lock:key", owner); err != nil || !released {
		t.Fatalf("owner LockRelease() = %v, %v", released, err)
	}
}
