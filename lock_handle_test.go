package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

type blockingReleaseStore struct {
	cachecore.Store
	locks           lockStore
	releaseStarted  chan struct{}
	releaseContinue chan struct{}
}

// LockAcquire delegates to the wrapped ownership-capable store.
func (s *blockingReleaseStore) LockAcquire(ctx context.Context, key string, owner []byte, ttl time.Duration) (bool, error) {
	return s.locks.LockAcquire(ctx, key, owner, ttl)
}

// LockRelease pauses before delegation so tests can attempt concurrent handle reuse.
func (s *blockingReleaseStore) LockRelease(ctx context.Context, key string, owner []byte) (bool, error) {
	close(s.releaseStarted)
	<-s.releaseContinue
	return s.locks.LockRelease(ctx, key, owner)
}

// TestLockHandleAcquireRelease verifies lock handles acquire once and release their key.
func TestLockHandleAcquireRelease(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	lock := c.NewLockHandle("lh:acquire", time.Second)

	locked, err := lock.Acquire()
	if err != nil || !locked {
		t.Fatalf("expected acquire success, locked=%v err=%v", locked, err)
	}

	other := c.NewLockHandle("lh:acquire", time.Second)
	locked, err = other.Acquire()
	if err != nil || locked {
		t.Fatalf("expected contention miss, locked=%v err=%v", locked, err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("release failed: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("second release should be no-op, got %v", err)
	}

	locked, err = other.Acquire()
	if err != nil || !locked {
		t.Fatalf("expected acquire after release, locked=%v err=%v", locked, err)
	}
}

// TestLockHandleWithContextSharesOwnership verifies derived handles cannot leave stale ownership behind.
func TestLockHandleWithContextSharesOwnership(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	owner := c.NewLockHandle("lh:derived", time.Second)
	locked, err := owner.Acquire()
	if err != nil || !locked {
		t.Fatalf("expected owner acquire success, locked=%v err=%v", locked, err)
	}

	derived := owner.WithContext(context.Background())
	if err := derived.Release(); err != nil {
		t.Fatalf("derived release failed: %v", err)
	}

	next := c.NewLockHandle("lh:derived", time.Second)
	locked, err = next.Acquire()
	if err != nil || !locked {
		t.Fatalf("expected next owner acquire success, locked=%v err=%v", locked, err)
	}
	if err := owner.Release(); err != nil {
		t.Fatalf("stale owner release failed: %v", err)
	}

	contender := c.NewLockHandle("lh:derived", time.Second)
	locked, err = contender.Acquire()
	if err != nil || locked {
		t.Fatalf("stale ownership deleted the next owner's lock, locked=%v err=%v", locked, err)
	}
	if err := next.Release(); err != nil {
		t.Fatalf("next owner release failed: %v", err)
	}
}

// TestLockHandleExpiredOwnerCannotReleaseSuccessor verifies handle identity survives backend TTL turnover.
func TestLockHandleExpiredOwnerCannotReleaseSuccessor(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	first := c.NewLockHandle("lh:expired-owner", 20*time.Millisecond)
	if locked, err := first.Acquire(); err != nil || !locked {
		t.Fatalf("first owner acquire failed: locked=%v err=%v", locked, err)
	}
	second := c.NewLockHandle("lh:expired-owner", time.Second)
	deadline := time.Now().Add(time.Second)
	for {
		locked, err := second.Acquire()
		if err != nil {
			t.Fatalf("second owner acquire failed: %v", err)
		}
		if locked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second owner did not acquire after expiration")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("stale owner release failed: %v", err)
	}
	contender := c.NewLockHandle("lh:expired-owner", time.Second)
	if locked, err := contender.Acquire(); err != nil || locked {
		t.Fatalf("stale owner removed successor lock: locked=%v err=%v", locked, err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("second owner release failed: %v", err)
	}
}

// TestLockHandleSerializesReleaseAndReuse verifies one handle cannot reacquire while its prior release is in flight.
func TestLockHandleSerializesReleaseAndReuse(t *testing.T) {
	base := NewMemoryStore(context.Background())
	store := &blockingReleaseStore{
		Store:           base,
		locks:           base.(lockStore),
		releaseStarted:  make(chan struct{}),
		releaseContinue: make(chan struct{}),
	}
	handle := NewCache(store).NewLockHandle("lh:serialized", time.Second)
	if locked, err := handle.Acquire(); err != nil || !locked {
		t.Fatalf("initial Acquire() = %v, %v", locked, err)
	}
	releaseDone := make(chan error, 1)
	go func() { releaseDone <- handle.Release() }()
	select {
	case <-store.releaseStarted:
	case <-time.After(time.Second):
		t.Fatal("Release did not reach the backing store")
	}
	acquireDone := make(chan bool, 1)
	go func() {
		locked, _ := handle.Acquire()
		acquireDone <- locked
	}()
	select {
	case <-acquireDone:
		close(store.releaseContinue)
		t.Fatal("Acquire returned while Release still owned the handle operation")
	case <-time.After(20 * time.Millisecond):
	}
	close(store.releaseContinue)
	select {
	case err := <-releaseDone:
		if err != nil {
			t.Fatalf("Release() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Release did not finish")
	}
	select {
	case locked := <-acquireDone:
		if !locked {
			t.Fatal("Acquire did not succeed after serialized release")
		}
	case <-time.After(time.Second):
		t.Fatal("Acquire did not resume after serialized release")
	}
}

// TestLockHandleOwnershipSurvivesValueShaping verifies private lock metadata bypasses value envelopes.
func TestLockHandleOwnershipSurvivesValueShaping(t *testing.T) {
	store := NewMemoryStoreWithConfig(context.Background(), StoreConfig{
		BaseConfig: cachecore.BaseConfig{
			Compression:   CompressionGzip,
			EncryptionKey: []byte("0123456789abcdef0123456789abcdef"),
		},
	})
	c := NewCache(store)
	first := c.NewLockHandle("lh:shaped", 20*time.Millisecond)
	if locked, err := first.Acquire(); err != nil || !locked {
		t.Fatalf("first owner acquire failed: locked=%v err=%v", locked, err)
	}
	time.Sleep(30 * time.Millisecond)
	second := c.NewLockHandle("lh:shaped", time.Second)
	if locked, err := second.Acquire(); err != nil || !locked {
		t.Fatalf("second owner acquire failed: locked=%v err=%v", locked, err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("stale owner release failed: %v", err)
	}
	contender := c.NewLockHandle("lh:shaped", time.Second)
	if locked, err := contender.Acquire(); err != nil || locked {
		t.Fatalf("stale shaped owner removed successor lock: locked=%v err=%v", locked, err)
	}
}

// TestLockHandleGetAutoReleasesOnSuccessAndError verifies Get releases locks after successful and failed callbacks.
func TestLockHandleGetAutoReleasesOnSuccessAndError(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	lock := c.NewLockHandle("lh:get", time.Second)

	var calls atomic.Int64
	locked, err := lock.Get(func() error {
		calls.Add(1)
		return nil
	})
	if err != nil || !locked {
		t.Fatalf("expected get lock callback success, locked=%v err=%v", locked, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected callback call count 1, got %d", calls.Load())
	}
	if lockedAgain, err := c.TryLock("lh:get", time.Second); err != nil || !lockedAgain {
		t.Fatalf("expected lock released after callback success, locked=%v err=%v", lockedAgain, err)
	}

	lockErr := c.NewLockHandle("lh:get:err", time.Second)
	expected := errors.New("boom")
	locked, err = lockErr.Get(func() error {
		calls.Add(1)
		return expected
	})
	if !locked || !errors.Is(err, expected) {
		t.Fatalf("expected callback error propagation, locked=%v err=%v", locked, err)
	}
	if lockedAgain, err := c.TryLock("lh:get:err", time.Second); err != nil || !lockedAgain {
		t.Fatalf("expected lock released after callback error, locked=%v err=%v", lockedAgain, err)
	}
}

// TestLockHandleBlockWaitsAndTimesOut verifies Block retries acquisition until success or timeout.
func TestLockHandleBlockWaitsAndTimesOut(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))

	seed := c.NewLockHandle("lh:block", 500*time.Millisecond)
	locked, err := seed.Acquire()
	if err != nil || !locked {
		t.Fatalf("seed acquire failed: locked=%v err=%v", locked, err)
	}

	go func() {
		time.Sleep(80 * time.Millisecond)
		_ = seed.Release()
	}()

	waiter := c.NewLockHandle("lh:block", time.Second)
	var called atomic.Int64
	locked, err = waiter.Block(time.Second, 10*time.Millisecond, func() error {
		called.Add(1)
		return nil
	})
	if err != nil || !locked {
		t.Fatalf("expected block acquire success, locked=%v err=%v", locked, err)
	}
	if called.Load() != 1 {
		t.Fatalf("expected callback exactly once, got %d", called.Load())
	}

	timeoutSeed := c.NewLockHandle("lh:block:timeout", time.Second)
	locked, err = timeoutSeed.Acquire()
	if err != nil || !locked {
		t.Fatalf("timeout seed acquire failed: locked=%v err=%v", locked, err)
	}

	timeoutWaiter := c.NewLockHandle("lh:block:timeout", time.Second)
	start := time.Now()
	locked, err = timeoutWaiter.Block(60*time.Millisecond, 10*time.Millisecond, func() error { return nil })
	elapsed := time.Since(start)
	if err == nil || locked {
		t.Fatalf("expected block timeout, locked=%v err=%v", locked, err)
	}
	if elapsed < 50*time.Millisecond || elapsed > 250*time.Millisecond {
		t.Fatalf("unexpected block timeout timing: %v", elapsed)
	}
}

// TestLockHandleContextCancellation verifies canceled contexts stop lock acquisition promptly.
func TestLockHandleContextCancellation(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	lock := c.NewLockHandle("lh:ctx:acquire", time.Second)
	locked, err := lock.WithContext(ctx).Acquire()
	// memory store is context-agnostic; bound-context Acquire may succeed because tryLock
	// delegates to store.Add, which ignores ctx for local drivers.
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected acquire ctx error: %v", err)
	}
	if locked {
		_ = lock.Release()
	}

	holder := c.NewLockHandle("lh:ctx:block", time.Second)
	locked, err = holder.Acquire()
	if err != nil || !locked {
		t.Fatalf("holder acquire failed: locked=%v err=%v", locked, err)
	}

	blockCtx, cancelBlock := context.WithCancel(context.Background())
	cancelBlock()
	waiter := c.NewLockHandle("lh:ctx:block", time.Second)
	waiter = waiter.WithContext(blockCtx)
	locked, err = waiter.block(blockCtx, 10*time.Millisecond, func(context.Context) error { return nil })
	if err == nil || locked {
		t.Fatalf("expected canceled block, locked=%v err=%v", locked, err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

// TestLockHandleNilCallbackValidation verifies lock helpers reject nil callbacks.
func TestLockHandleNilCallbackValidation(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))

	lock := c.NewLockHandle("lh:nil:get", time.Second)
	locked, err := lock.Get(nil)
	if err == nil || !locked {
		t.Fatalf("expected nil callback error after acquire, locked=%v err=%v", locked, err)
	}
	if relocked, err := c.TryLock("lh:nil:get", time.Second); err != nil || !relocked {
		t.Fatalf("expected auto-release on nil callback error, locked=%v err=%v", relocked, err)
	}

	lock2 := c.NewLockHandle("lh:nil:block", time.Second)
	locked, err = lock2.Block(200*time.Millisecond, 10*time.Millisecond, nil)
	if err == nil || !locked {
		t.Fatalf("expected nil callback error after block acquire, locked=%v err=%v", locked, err)
	}
	if relocked, err := c.TryLock("lh:nil:block", time.Second); err != nil || !relocked {
		t.Fatalf("expected auto-release after nil callback block error, locked=%v err=%v", relocked, err)
	}
}

// TestLockHandleGetAndBlockReturnFalseWhenNotAcquired verifies callbacks do not run without ownership.
func TestLockHandleGetAndBlockReturnFalseWhenNotAcquired(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	holder := c.NewLockHandle("lh:busy", time.Second)
	if locked, err := holder.Acquire(); err != nil || !locked {
		t.Fatalf("holder acquire failed: locked=%v err=%v", locked, err)
	}

	getter := c.NewLockHandle("lh:busy", time.Second)
	var getCalls atomic.Int64
	locked, err := getter.Get(func() error {
		getCalls.Add(1)
		return nil
	})
	if err != nil || locked {
		t.Fatalf("expected get contention miss, locked=%v err=%v", locked, err)
	}
	if getCalls.Load() != 0 {
		t.Fatalf("callback should not run when get lock not acquired")
	}

	waiter := c.NewLockHandle("lh:busy", time.Second)
	locked, err = waiter.Block(40*time.Millisecond, 10*time.Millisecond, func() error {
		t.Fatalf("callback should not run on block timeout")
		return nil
	})
	if err == nil || locked {
		t.Fatalf("expected block timeout miss, locked=%v err=%v", locked, err)
	}
}
