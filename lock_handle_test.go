package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

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

func TestLockHandleContextCancellation(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	lock := c.NewLockHandle("lh:ctx:acquire", time.Second)
	locked, err := lock.AcquireCtx(ctx)
	// memory store is context-agnostic; AcquireCtx may succeed because TryLockCtx
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
	locked, err = waiter.BlockCtx(blockCtx, 10*time.Millisecond, func(context.Context) error { return nil })
	if err == nil || locked {
		t.Fatalf("expected canceled block, locked=%v err=%v", locked, err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

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
