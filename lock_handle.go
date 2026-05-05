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
	ctx   context.Context
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

func (l *LockHandle) WithContext(ctx context.Context) *LockHandle {
	clone := *l
	clone.ctx = ctx
	return &clone
}

func (l *LockHandle) context() context.Context {
	if l == nil || l.ctx == nil {
		return l.cache.context()
	}
	return l.ctx
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
	locked, err := l.cache.tryLock(l.context(), l.key, l.ttl)
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
	if !l.held.Load() {
		return nil
	}
	if err := l.cache.unlock(l.context(), l.key); err != nil {
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
	return l.get(l.context(), func(context.Context) error {
		if fn == nil {
			return errors.New("cache lock handle requires a callback")
		}
		return fn()
	})
}

func (l *LockHandle) get(ctx context.Context, fn func(context.Context) error) (bool, error) {
	locked, err := l.cache.tryLock(ctx, l.key, l.ttl)
	if err != nil || !locked {
		return locked, err
	}
	l.held.Store(true)
	defer func() { _ = l.WithContext(ctx).Release() }()
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
	ctx := l.context()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return l.block(ctx, retryInterval, func(context.Context) error {
		if fn == nil {
			return errors.New("cache lock handle requires a callback")
		}
		return fn()
	})
}

func (l *LockHandle) block(ctx context.Context, retryInterval time.Duration, fn func(context.Context) error) (bool, error) {
	locked, err := l.cache.lock(ctx, l.key, l.ttl, retryInterval)
	if err != nil || !locked {
		return locked, err
	}
	l.held.Store(true)
	defer func() { _ = l.WithContext(ctx).Release() }()
	if fn == nil {
		return true, errors.New("cache lock handle requires a callback")
	}
	return true, fn(ctx)
}
