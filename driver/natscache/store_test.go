package natscache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/nats-io/nats.go"
)

type lockKeyValue struct {
	mu        sync.Mutex
	value     []byte
	revision  uint64
	getErr    error
	createErr error
	updateErr error
}

// Status reports a healthy test bucket.
func (k *lockKeyValue) Status() (nats.KeyValueStatus, error) { return nil, nil }

// Get returns the current test entry.
func (k *lockKeyValue) Get(key string) (nats.KeyValueEntry, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.getErr != nil {
		return nil, k.getErr
	}
	if k.value == nil {
		return nil, nats.ErrKeyNotFound
	}
	return lockKeyValueEntry{key: key, value: k.value, revision: k.revision}, nil
}

// Put replaces the current test value.
func (k *lockKeyValue) Put(_ string, value []byte) (uint64, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.put(value)
}

// put replaces the value while the caller holds the test store mutex.
func (k *lockKeyValue) put(value []byte) (uint64, error) {
	k.revision++
	k.value = append([]byte(nil), value...)
	return k.revision, nil
}

// Create records the value only when the test key is absent.
func (k *lockKeyValue) Create(_ string, value []byte) (uint64, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.createErr != nil {
		return 0, k.createErr
	}
	if k.value != nil {
		return 0, nats.ErrKeyExists
	}
	return k.put(value)
}

// Update replaces the value only at the expected revision.
func (k *lockKeyValue) Update(_ string, value []byte, revision uint64) (uint64, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.updateErr != nil {
		return 0, k.updateErr
	}
	if revision != k.revision {
		return 0, nats.ErrKeyExists
	}
	return k.put(value)
}

// Delete removes the current test value.
func (k *lockKeyValue) Delete(_ string, _ ...nats.DeleteOpt) error {
	k.mu.Lock()
	defer k.mu.Unlock()
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

// racingLockKeyValue holds two reads at one revision so concurrent replacement is deterministic.
type racingLockKeyValue struct {
	*lockKeyValue
	reads atomic.Int32
	both  chan struct{}
}

// Get waits until both contenders have observed the same expired revision.
func (k *racingLockKeyValue) Get(key string) (nats.KeyValueEntry, error) {
	entry, err := k.lockKeyValue.Get(key)
	if k.reads.Add(1) == 2 {
		close(k.both)
	}
	<-k.both
	return entry, err
}

// TestNewNilKeyValueErrors verifies NATS construction rejects a nil key-value bucket.
func TestNewNilKeyValueErrors(t *testing.T) {
	cacheStore := New(Config{})
	ctx := context.Background()
	if err := cacheStore.Ready(ctx); err == nil {
		t.Fatalf("expected ready error when key-value is nil")
	}
	if _, _, err := cacheStore.Get(ctx, "k"); err == nil {
		t.Fatalf("expected get error when key-value is nil")
	}
	impl := cacheStore.(*store)
	if acquired, err := impl.LockAcquire(ctx, "k", []byte("owner"), time.Minute); acquired || err == nil {
		t.Fatalf("expected lock acquire error when key-value is nil")
	}
	if released, err := impl.LockRelease(ctx, "k", []byte("owner")); released || err == nil {
		t.Fatalf("expected lock release error when key-value is nil")
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
	if acquired, err := store.LockAcquire(ctx, "lock:key", owner, time.Minute); err != nil || acquired {
		t.Fatalf("contended LockAcquire() = %v, %v", acquired, err)
	}
	if released, err := store.LockRelease(ctx, "lock:key", []byte("stale")); err != nil || released {
		t.Fatalf("stale LockRelease() = %v, %v", released, err)
	}
	if released, err := store.LockRelease(ctx, "lock:key", owner); err != nil || !released {
		t.Fatalf("owner LockRelease() = %v, %v", released, err)
	}
}

// TestLockAcquireEnvelopeBranches verifies creation, live contention, decoding, and backend failures.
func TestLockAcquireEnvelopeBranches(t *testing.T) {
	t.Run("create and live contention", func(t *testing.T) {
		kv := &lockKeyValue{}
		store := &store{kv: kv, prefix: "p", defaultTTL: time.Minute}
		if acquired, err := store.LockAcquire(context.Background(), "lock:key", []byte("owner"), time.Minute); err != nil || !acquired {
			t.Fatalf("LockAcquire() = %v, %v", acquired, err)
		}
		if acquired, err := store.LockAcquire(context.Background(), "lock:key", []byte("other"), time.Minute); err != nil || acquired {
			t.Fatalf("live LockAcquire() = %v, %v", acquired, err)
		}
	})

	t.Run("get failure", func(t *testing.T) {
		expected := errors.New("get failed")
		store := &store{kv: &lockKeyValue{getErr: expected}, prefix: "p", defaultTTL: time.Minute}
		if acquired, err := store.LockAcquire(context.Background(), "lock:key", []byte("owner"), time.Minute); acquired || !errors.Is(err, expected) {
			t.Fatalf("LockAcquire() = %v, %v, want get failure", acquired, err)
		}
	})

	t.Run("create failure", func(t *testing.T) {
		expected := errors.New("create failed")
		store := &store{kv: &lockKeyValue{createErr: expected}, prefix: "p", defaultTTL: time.Minute}
		if acquired, err := store.LockAcquire(context.Background(), "lock:key", []byte("owner"), time.Minute); acquired || !errors.Is(err, expected) {
			t.Fatalf("LockAcquire() = %v, %v, want create failure", acquired, err)
		}
	})

	t.Run("corrupt envelope", func(t *testing.T) {
		kv := &lockKeyValue{value: []byte("{"), revision: 1}
		store := &store{kv: kv, prefix: "p", defaultTTL: time.Minute}
		if acquired, err := store.LockAcquire(context.Background(), "lock:key", []byte("owner"), time.Minute); acquired || err == nil {
			t.Fatalf("LockAcquire() = %v, %v, want decode failure", acquired, err)
		}
	})

	t.Run("expired update failure", func(t *testing.T) {
		expected := errors.New("update failed")
		kv := &lockKeyValue{}
		store := &store{kv: kv, prefix: "p", defaultTTL: time.Minute}
		expired := store.encodeEnvelope([]byte("expired"), time.Millisecond)
		if _, err := kv.Put(store.cacheKey("lock:key"), expired); err != nil {
			t.Fatalf("seed expired lock: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
		kv.mu.Lock()
		kv.updateErr = expected
		kv.mu.Unlock()
		if acquired, err := store.LockAcquire(context.Background(), "lock:key", []byte("owner"), time.Minute); acquired || !errors.Is(err, expected) {
			t.Fatalf("LockAcquire() = %v, %v, want update failure", acquired, err)
		}
	})
}

// TestLockAcquireReplacesExpiredEnvelopeByRevision verifies only one contender can claim an expired lock.
func TestLockAcquireReplacesExpiredEnvelopeByRevision(t *testing.T) {
	base := &lockKeyValue{}
	kv := &racingLockKeyValue{lockKeyValue: base, both: make(chan struct{})}
	store := &store{kv: kv, prefix: "p", defaultTTL: time.Minute}
	expired := store.encodeEnvelope([]byte("expired"), time.Millisecond)
	if _, err := base.Put(store.cacheKey("lock:key"), expired); err != nil {
		t.Fatalf("seed expired lock: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	type result struct {
		owner    string
		acquired bool
		err      error
	}
	results := make(chan result, 2)
	for _, owner := range []string{"first", "second"} {
		owner := owner
		go func() {
			acquired, err := store.LockAcquire(context.Background(), "lock:key", []byte(owner), time.Minute)
			results <- result{owner: owner, acquired: acquired, err: err}
		}()
	}
	winners := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("LockAcquire(%s) error = %v", result.owner, result.err)
		}
		if result.acquired {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("successful expired-lock contenders = %d, want 1", winners)
	}
}
