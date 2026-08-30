package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// LockHandle provides ergonomic lock management on top of Cache lock helpers.
//
// Standard bundled backends bind each handle to an opaque owner token, so Release cannot
// delete a successor's lock after this handle's TTL expires. Reuse one handle
// for one ownership lifecycle at a time.
//
// @group Locking
type LockHandle struct {
	cache       *Cache
	key         string
	ttl         time.Duration
	held        *atomic.Bool
	operationMu *sync.Mutex
	owner       []byte
	ownerErr    error
	ctx         context.Context
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
	owner, ownerErr := newLockOwnerToken()
	return &LockHandle{
		cache:       c,
		key:         key,
		ttl:         ttl,
		held:        &atomic.Bool{},
		operationMu: &sync.Mutex{},
		owner:       owner,
		ownerErr:    ownerErr,
	}
}

// WithContext returns a derived handle that shares lock ownership state with l.
func (l *LockHandle) WithContext(ctx context.Context) *LockHandle {
	clone := *l
	clone.ctx = ctx
	return &clone
}

// context returns the handle context or the cache context when none was bound.
func (l *LockHandle) context() context.Context {
	if l.ctx == nil {
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
	l.operationMu.Lock()
	defer l.operationMu.Unlock()
	if l.ownerErr != nil {
		return false, l.ownerErr
	}
	if l.held.Load() {
		return false, nil
	}
	locked, err := l.cache.tryLockOwned(l.context(), l.key, l.owner, l.ttl)
	if locked && err == nil {
		l.held.Store(true)
	}
	return locked, err
}

// Release unlocks the key if this handle previously acquired and still owns it.
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
	l.operationMu.Lock()
	defer l.operationMu.Unlock()
	if !l.held.CompareAndSwap(true, false) {
		return nil
	}
	if err := l.cache.unlockOwned(l.context(), l.key, l.owner); err != nil {
		l.held.Store(true)
		return err
	}
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

// get shares the single-attempt ownership and release sequence with callback adapters.
func (l *LockHandle) get(ctx context.Context, fn func(context.Context) error) (bool, error) {
	l.operationMu.Lock()
	if l.ownerErr != nil {
		l.operationMu.Unlock()
		return false, l.ownerErr
	}
	if l.held.Load() {
		l.operationMu.Unlock()
		return false, nil
	}
	locked, err := l.cache.tryLockOwned(ctx, l.key, l.owner, l.ttl)
	if err != nil || !locked {
		l.operationMu.Unlock()
		return locked, err
	}
	l.held.Store(true)
	l.operationMu.Unlock()
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

// block shares the retrying ownership and release sequence with callback adapters.
func (l *LockHandle) block(ctx context.Context, retryInterval time.Duration, fn func(context.Context) error) (bool, error) {
	l.operationMu.Lock()
	if l.ownerErr != nil {
		l.operationMu.Unlock()
		return false, l.ownerErr
	}
	if l.held.Load() {
		l.operationMu.Unlock()
		return false, nil
	}
	locked, err := l.cache.lockOwned(ctx, l.key, l.owner, l.ttl, retryInterval)
	if err != nil || !locked {
		l.operationMu.Unlock()
		return locked, err
	}
	l.held.Store(true)
	l.operationMu.Unlock()
	defer func() { _ = l.WithContext(ctx).Release() }()
	if fn == nil {
		return true, errors.New("cache lock handle requires a callback")
	}
	return true, fn(ctx)
}
