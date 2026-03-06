package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

// LockHandle provides ergonomic lock management on top of Cache lock helpers.
//
// It wraps TryLock/Lock/Unlock and adds callback-based helpers.
//
// Caveat:
//   - Release is a best-effort wrapper over Unlock and does not perform owner-token
//     validation. Do not assume ownership safety after lock expiry.
//
// @group Locking
type LockHandle struct {
	cache *Cache
	key   string
	ttl   time.Duration
	held  atomic.Bool
}

// NewLockHandle creates a reusable lock handle for a key/ttl pair.
// @group Locking
//
// Example: lock handle acquire/release
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	lock := c.NewLockHandle("job:sync", 10*time.Second)
//	locked, err := lock.Acquire()
//	fmt.Println(err == nil, locked) // true true
//	if locked {
//		_ = lock.Release()
//	}
func (c *Cache) NewLockHandle(key string, ttl time.Duration) *LockHandle {
	return &LockHandle{
		cache: c,
		key:   key,
		ttl:   ttl,
	}
}

// Acquire attempts to acquire the lock once (non-blocking).
// @group Locking
//
// Example: single acquire attempt
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	lock := c.NewLockHandle("job:sync", 10*time.Second)
//	locked, err := lock.Acquire()
//	fmt.Println(err == nil, locked) // true true
func (l *LockHandle) Acquire() (bool, error) {
	return l.AcquireContext(context.Background())
}

// AcquireContext is the context-aware variant of Acquire.
// @group Locking
func (l *LockHandle) AcquireContext(ctx context.Context) (bool, error) {
	locked, err := l.cache.TryLockContext(ctx, l.key, l.ttl)
	if locked && err == nil {
		l.held.Store(true)
	}
	return locked, err
}

// Release unlocks the key if this handle previously acquired it.
//
// It is safe to call multiple times; repeated calls become no-ops after the first
// successful release.
// @group Locking
//
// Example: release a held lock
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	lock := c.NewLockHandle("job:sync", 10*time.Second)
//	locked, _ := lock.Acquire()
//	if locked {
//		_ = lock.Release()
//	}
func (l *LockHandle) Release() error {
	return l.ReleaseContext(context.Background())
}

// ReleaseContext is the context-aware variant of Release.
// @group Locking
func (l *LockHandle) ReleaseContext(ctx context.Context) error {
	if !l.held.Load() {
		return nil
	}
	if err := l.cache.UnlockContext(ctx, l.key); err != nil {
		return err
	}
	l.held.Store(false)
	return nil
}

// Get acquires the lock once, runs fn if acquired, then releases automatically.
// @group Locking
//
// Example: acquire once and auto-release
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	lock := c.NewLockHandle("job:sync", 10*time.Second)
//	locked, err := lock.Get(func() error {
//		// do protected work
//		return nil
//	})
//	fmt.Println(err == nil, locked) // true true
func (l *LockHandle) Get(fn func() error) (bool, error) {
	return l.GetContext(context.Background(), func(context.Context) error {
		if fn == nil {
			return errors.New("cache lock handle requires a callback")
		}
		return fn()
	})
}

// GetContext is the context-aware variant of Get.
// @group Locking
func (l *LockHandle) GetContext(ctx context.Context, fn func(context.Context) error) (bool, error) {
	locked, err := l.AcquireContext(ctx)
	if err != nil || !locked {
		return locked, err
	}
	defer func() { _ = l.ReleaseContext(ctx) }()
	if fn == nil {
		return true, errors.New("cache lock handle requires a callback")
	}
	return true, fn(ctx)
}

// Block waits up to timeout to acquire the lock, runs fn if acquired, then releases.
//
// retryInterval <= 0 falls back to the cache default lock retry interval.
// @group Locking
//
// Example: wait for lock, then auto-release
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	lock := c.NewLockHandle("job:sync", 10*time.Second)
//	locked, err := lock.Block(500*time.Millisecond, 25*time.Millisecond, func() error {
//		// do protected work
//		return nil
//	})
//	fmt.Println(err == nil, locked) // true true
func (l *LockHandle) Block(timeout, retryInterval time.Duration, fn func() error) (bool, error) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return l.BlockContext(ctx, retryInterval, func(context.Context) error {
		if fn == nil {
			return errors.New("cache lock handle requires a callback")
		}
		return fn()
	})
}

// BlockContext is the context-aware variant of Block.
// @group Locking
func (l *LockHandle) BlockContext(ctx context.Context, retryInterval time.Duration, fn func(context.Context) error) (bool, error) {
	locked, err := l.cache.LockContext(ctx, l.key, l.ttl, retryInterval)
	if err != nil || !locked {
		return locked, err
	}
	l.held.Store(true)
	defer func() { _ = l.ReleaseContext(ctx) }()
	if fn == nil {
		return true, errors.New("cache lock handle requires a callback")
	}
	return true, fn(ctx)
}
