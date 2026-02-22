//go:build integration

package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

type storeFactory struct {
	name string
	new  func(t *testing.T, opts ...StoreOption) (Store, func())
}

type contractCase struct {
	name                   string
	opts                   []StoreOption
	verifyDefaultTTLExpiry bool
	verifyMaxValueLimit    bool
}

func TestStoreContract_AllDrivers(t *testing.T) {
	fixtures := integrationFixtures(t)
	cases := integrationContractCases()

	for _, fx := range fixtures {
		fx := fx
		t.Run(fx.name, func(t *testing.T) {
			for _, tc := range cases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					store, cleanup := fx.new(t, tc.opts...)
					t.Cleanup(cleanup)
					runStoreContractSuite(t, store, tc)
				})
			}
			runDriverFactoryInvariantSuite(t, fx)
		})
	}
}

func runStoreContractSuite(t *testing.T, store Store, tc contractCase) {
	t.Helper()
	ctx := context.Background()
	noOp := store.Driver() == DriverNull
	skipCloneCheck := store.Driver() == DriverMemcached

	// Memcached flush_all semantics can briefly affect keys written in the same
	// second as a prior flush in a previous subtest.
	if store.Driver() == DriverMemcached && tc.name != "baseline" {
		time.Sleep(1100 * time.Millisecond)
	}

	ttl, wait := contractTTL(store.Driver())
	caseKey := func(base string) string { return tc.name + ":" + base }

	// Set/Get returns clone and round-trips.
	if err := store.Set(ctx, caseKey("alpha"), []byte("value"), 500*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, caseKey("alpha"))
	if err != nil {
		t.Fatalf("get failed: ok=%v err=%v", ok, err)
	}
	if noOp {
		if ok {
			t.Fatalf("expected null store miss on get")
		}
	} else {
		if !ok {
			t.Fatalf("expected key hit on get")
		}
		if !skipCloneCheck {
			body[0] = 'X'
			body2, ok, err := store.Get(ctx, caseKey("alpha"))
			if err != nil || !ok || string(body2) != "value" {
				t.Fatalf("expected stored value unchanged, got %q err=%v", string(body2), err)
			}
		}
	}

	// TTL expiry.
	if err := store.Set(ctx, caseKey("ttl"), []byte("v"), ttl); err != nil {
		t.Fatalf("set ttl failed: %v", err)
	}
	time.Sleep(wait)
	if _, ok, err := store.Get(ctx, caseKey("ttl")); err != nil || ok {
		t.Fatalf("expected ttl key expired; ok=%v err=%v", ok, err)
	}

	// Add only when missing.
	created, err := store.Add(ctx, caseKey("once"), []byte("first"), time.Second)
	if err != nil {
		t.Fatalf("add first failed: created=%v err=%v", created, err)
	}
	created, err = store.Add(ctx, caseKey("once"), []byte("second"), time.Second)
	if err != nil {
		t.Fatalf("add duplicate failed: %v", err)
	}
	if noOp {
		if !created {
			t.Fatalf("expected null add to report created=true")
		}
	} else if created {
		t.Fatalf("expected duplicate add to return created=false")
	}

	// Counters refresh TTL.
	value, err := store.Increment(ctx, caseKey("counter"), 3, time.Second)
	if err != nil {
		t.Fatalf("increment failed: value=%d err=%v", value, err)
	}
	if noOp {
		if value != 0 {
			t.Fatalf("expected null increment to return 0, got %d", value)
		}
	} else if value != 3 {
		t.Fatalf("expected incremented value to be 3, got %d", value)
	}
	value, err = store.Decrement(ctx, caseKey("counter"), 1, time.Second)
	if err != nil {
		t.Fatalf("decrement failed: value=%d err=%v", value, err)
	}
	if noOp {
		if value != 0 {
			t.Fatalf("expected null decrement to return 0, got %d", value)
		}
	} else if value != 2 {
		t.Fatalf("expected decremented value to be 2, got %d", value)
	}

	// Delete & DeleteMany.
	if err := store.Set(ctx, caseKey("a"), []byte("1"), time.Second); err != nil {
		t.Fatalf("set a failed: %v", err)
	}
	if err := store.Set(ctx, caseKey("b"), []byte("2"), time.Second); err != nil {
		t.Fatalf("set b failed: %v", err)
	}
	if err := store.Delete(ctx, caseKey("a")); err != nil {
		t.Fatalf("delete a failed: %v", err)
	}
	if err := store.DeleteMany(ctx, caseKey("b")); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, caseKey("a")); err != nil || ok {
		t.Fatalf("expected key a deleted")
	}
	if _, ok, err := store.Get(ctx, caseKey("b")); err != nil || ok {
		t.Fatalf("expected key b deleted")
	}

	// Flush clears all keys.
	if err := store.Set(ctx, caseKey("flush"), []byte("x"), time.Second); err != nil {
		t.Fatalf("set flush failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, caseKey("flush")); err != nil || ok {
		t.Fatalf("expected flush to clear key; ok=%v err=%v", ok, err)
	}

	// Typed remember across drivers.
	type payload struct {
		Name string `json:"name"`
	}
	cache := NewCache(store)
	calls := 0
	val, err := Remember[payload](cache, caseKey("remember:typed"), time.Minute, func() (payload, error) {
		calls++
		return payload{Name: "Ada"}, nil
	})
	if err != nil || val.Name != "Ada" {
		t.Fatalf("remember typed failed: %+v err=%v", val, err)
	}
	// Cached path should bypass callback.
	val, err = Remember[payload](cache, caseKey("remember:typed"), time.Minute, func() (payload, error) {
		calls++
		return payload{Name: "Other"}, nil
	})
	if err != nil {
		t.Fatalf("remember typed second call failed: %+v err=%v", val, err)
	}
	if noOp {
		if calls != 2 || val.Name != "Other" {
			t.Fatalf("expected null remember to recompute, calls=%d val=%+v", calls, val)
		}
	} else if calls != 1 || val.Name != "Ada" {
		t.Fatalf("remember typed cache miss: calls=%d val=%+v err=%v", calls, val, err)
	}

	if tc.verifyDefaultTTLExpiry {
		runDefaultTTLWriteOpInvariant(t, store, caseKey)
	}

	if tc.verifyMaxValueLimit {
		limitSized := []byte("1234567890abcdef")
		if err := store.Set(ctx, caseKey("at-limit"), limitSized, time.Second); err != nil {
			t.Fatalf("expected value at max limit to succeed, got %v", err)
		}
		if store.Driver() != DriverNull {
			body, ok, err := store.Get(ctx, caseKey("at-limit"))
			if err != nil || !ok || string(body) != string(limitSized) {
				t.Fatalf("expected round-trip for value at max limit; ok=%v body=%q err=%v", ok, string(body), err)
			}
		}

		tooLarge := []byte("abcdefghijklmnopqrstuvwxyz0123456789")
		err = store.Set(ctx, caseKey("too-large"), tooLarge, time.Second)
		if !errors.Is(err, ErrValueTooLarge) {
			t.Fatalf("expected ErrValueTooLarge, got %v", err)
		}
	}

	// Baseline-only cache helper edge cases still run across every driver,
	// without multiplying test time by every option permutation.
	if tc.name == "baseline" {
		runCacheHelperInvariantSuite(t, store, caseKey)
	}
}

func contractTTL(driver Driver) (ttl time.Duration, wait time.Duration) {
	switch driver {
	case DriverMemcached:
		return time.Second, 1500 * time.Millisecond
	default:
		return 50 * time.Millisecond, 80 * time.Millisecond
	}
}

func defaultTTLWait(driver Driver) time.Duration {
	switch driver {
	case DriverMemcached, DriverRedis:
		// Memcached TTL is second-granularity. Redis Set/Add support sub-second TTL,
		// but counter paths (Increment/Decrement) refresh TTL via EXPIRE, which is
		// second-granularity in the current implementation.
		return 1500 * time.Millisecond
	default:
		return 120 * time.Millisecond
	}
}

func runDefaultTTLWriteOpInvariant(t *testing.T, store Store, caseKey func(string) string) {
	t.Helper()
	ctx := context.Background()

	type opCase struct {
		name string
		run  func(key string, ttl time.Duration) error
	}
	ops := []opCase{
		{
			name: "set",
			run: func(key string, ttl time.Duration) error {
				return store.Set(ctx, key, []byte("v"), ttl)
			},
		},
		{
			name: "add",
			run: func(key string, ttl time.Duration) error {
				_, err := store.Add(ctx, key, []byte("v"), ttl)
				return err
			},
		},
		{
			name: "increment",
			run: func(key string, ttl time.Duration) error {
				_, err := store.Increment(ctx, key, 1, ttl)
				return err
			},
		},
		{
			name: "decrement",
			run: func(key string, ttl time.Duration) error {
				_, err := store.Decrement(ctx, key, 1, ttl)
				return err
			},
		},
	}

	var keys []string
	for _, ttl := range []time.Duration{0, -1 * time.Second} {
		for _, op := range ops {
			key := caseKey(fmt.Sprintf("default_ttl:%s:%d", op.name, ttl.Nanoseconds()))
			if err := op.run(key, ttl); err != nil {
				t.Fatalf("%s default ttl write failed for ttl=%v: %v", op.name, ttl, err)
			}
			keys = append(keys, key)
		}
	}

	time.Sleep(defaultTTLWait(store.Driver()))
	for _, key := range keys {
		if _, ok, err := store.Get(ctx, key); err != nil || ok {
			t.Fatalf("expected default ttl key expired for key=%q; ok=%v err=%v", key, ok, err)
		}
	}
}

func runCacheHelperInvariantSuite(t *testing.T, store Store, caseKey func(string) string) {
	t.Helper()
	cache := NewCache(store)
	noOp := store.Driver() == DriverNull

	t.Run("remember_stale_non_positive_stale_ttl_falls_back_to_primary_ttl", func(t *testing.T) {
		key := caseKey("edge:remember_stale:ttl_fallback")

		val, usedStale, err := cache.RememberStaleBytes(key, 2*time.Second, 0, func() ([]byte, error) {
			return []byte("seed"), nil
		})
		if err != nil {
			t.Fatalf("seed remember stale failed: %v", err)
		}
		if noOp {
			if usedStale || string(val) != "seed" {
				t.Fatalf("null store seed semantics mismatch: usedStale=%v val=%q", usedStale, string(val))
			}
			return
		}
		if usedStale || string(val) != "seed" {
			t.Fatalf("seed remember stale mismatch: usedStale=%v val=%q", usedStale, string(val))
		}
		if err := cache.Delete(key); err != nil {
			t.Fatalf("delete fresh key failed: %v", err)
		}

		expected := errors.New("upstream down")
		val, usedStale, err = cache.RememberStaleBytes(key, 2*time.Second, 0, func() ([]byte, error) {
			return nil, expected
		})
		if err != nil || !usedStale || string(val) != "seed" {
			t.Fatalf("expected stale fallback via ttl-derived stale ttl, usedStale=%v val=%q err=%v", usedStale, string(val), err)
		}
	})

	t.Run("remember_stale_no_stale_key_when_both_ttls_non_positive", func(t *testing.T) {
		key := caseKey("edge:remember_stale:no_stale_write")

		val, usedStale, err := cache.RememberStaleBytes(key, 0, 0, func() ([]byte, error) {
			return []byte("seed"), nil
		})
		if err != nil {
			t.Fatalf("seed remember stale failed: %v", err)
		}
		if usedStale || string(val) != "seed" {
			t.Fatalf("seed remember stale mismatch: usedStale=%v val=%q", usedStale, string(val))
		}
		if noOp {
			expected := errors.New("upstream down")
			_, usedStale, err = cache.RememberStaleBytes(key, 0, 0, func() ([]byte, error) {
				return nil, expected
			})
			if !errors.Is(err, expected) || usedStale {
				t.Fatalf("expected null store recompute error without stale fallback, usedStale=%v err=%v", usedStale, err)
			}
			return
		}

		if _, ok, err := cache.Get(key + staleSuffix); err != nil {
			t.Fatalf("unexpected stale key read error: %v", err)
		} else if ok {
			t.Fatalf("expected no stale key when both ttl inputs are non-positive")
		}
		if err := cache.Delete(key); err != nil {
			t.Fatalf("delete fresh key failed: %v", err)
		}

		expected := errors.New("upstream down")
		_, usedStale, err = cache.RememberStaleBytes(key, 0, 0, func() ([]byte, error) {
			return nil, expected
		})
		if usedStale {
			t.Fatalf("expected no stale fallback when stale key was never written")
		}
		if !errors.Is(err, expected) {
			t.Fatalf("expected original callback error, got %v", err)
		}
	})

	t.Run("refresh_ahead_hit_skips_async_refresh_without_valid_metadata", func(t *testing.T) {
		if noOp {
			t.Skip("null store does not persist seeded cache hits")
		}

		for _, malformedMeta := range []bool{false, true} {
			suffix := "missing_meta"
			if malformedMeta {
				suffix = "malformed_meta"
			}
			key := caseKey("edge:refresh_ahead:" + suffix)
			if err := cache.Set(key, []byte("cached"), 2*time.Second); err != nil {
				t.Fatalf("seed value failed: %v", err)
			}
			if malformedMeta {
				if err := cache.Set(key+refreshMetaSuffix, []byte("not-an-int"), 2*time.Second); err != nil {
					t.Fatalf("seed malformed metadata failed: %v", err)
				}
			}

			var calls atomic.Int64
			body, err := cache.RefreshAhead(key, 2*time.Second, time.Second, func() ([]byte, error) {
				calls.Add(1)
				return []byte("refreshed"), nil
			})
			if err != nil {
				t.Fatalf("refresh ahead failed: %v", err)
			}
			if got := string(body); got != "cached" {
				t.Fatalf("expected cached value on hit, got %q", got)
			}

			time.Sleep(150 * time.Millisecond)
			if got := calls.Load(); got != 0 {
				t.Fatalf("expected no async refresh callback without valid metadata, got %d", got)
			}
		}
	})

	runLockHelperInvariantSuite(t, cache, store.Driver(), caseKey, noOp)
	runRateLimitHelperInvariantSuite(t, cache, store.Driver(), caseKey, noOp)
	runRefreshAheadHelperInvariantSuite(t, cache, store.Driver(), caseKey, noOp)
	runRememberStaleDeeperInvariantSuite(t, cache, store.Driver(), caseKey, noOp)
	runBatchHelperInvariantSuite(t, cache, store.Driver(), caseKey, noOp)
	runCounterHelperInvariantSuite(t, cache, store.Driver(), caseKey, noOp)
}

func runLockHelperInvariantSuite(t *testing.T, cache *Cache, driver Driver, caseKey func(string) string, noOp bool) {
	t.Helper()

	t.Run("lock_unlock_missing_key_safe", func(t *testing.T) {
		if err := cache.Unlock(caseKey("lock:missing")); err != nil {
			t.Fatalf("unlock missing key should be safe, got %v", err)
		}
	})

	t.Run("try_lock_single_winner_under_contention", func(t *testing.T) {
		key := caseKey("lock:contend")
		ttl := lockTTLFor(driver)
		const workers = 12
		start := make(chan struct{})
		var wg sync.WaitGroup
		var winners atomic.Int64
		errs := make(chan error, workers)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				locked, err := cache.TryLock(key, ttl)
				if err != nil {
					errs <- err
					return
				}
				if locked {
					winners.Add(1)
				}
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("unexpected try lock error: %v", err)
		}
		got := winners.Load()
		if noOp {
			if got != workers {
				t.Fatalf("null store should report all try locks as acquired, got %d/%d", got, workers)
			}
			return
		}
		if driver == DriverFile {
			if got < 1 {
				t.Fatalf("expected at least one try-lock winner for file backend, got %d", got)
			}
			return
		}
		if got != 1 {
			t.Fatalf("expected single try-lock winner, got %d", got)
		}
	})

	if noOp {
		return
	}

	t.Run("lock_timeout_and_context_cancellation", func(t *testing.T) {
		holdTTL := lockTTLFor(driver)
		timeoutKey := caseKey("lock:timeout")
		locked, err := cache.TryLock(timeoutKey, holdTTL)
		if err != nil || !locked {
			t.Fatalf("seed lock failed: locked=%v err=%v", locked, err)
		}

		locked, err = cache.Lock(timeoutKey, holdTTL, 80*time.Millisecond)
		if err == nil || locked {
			t.Fatalf("expected lock timeout, got locked=%v err=%v", locked, err)
		}

		cancelKey := caseKey("lock:canceled")
		locked, err = cache.TryLock(cancelKey, holdTTL)
		if err != nil || !locked {
			t.Fatalf("seed cancel lock failed: locked=%v err=%v", locked, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		locked, err = cache.LockCtx(ctx, cancelKey, holdTTL, 10*time.Millisecond)
		if err == nil || locked {
			t.Fatalf("expected lock ctx cancellation, got locked=%v err=%v", locked, err)
		}
	})

	t.Run("lock_ttl_expiry_allows_reacquire", func(t *testing.T) {
		key := caseKey("lock:expiry")
		ttl := lockTTLFor(driver)
		locked, err := cache.TryLock(key, ttl)
		if err != nil || !locked {
			t.Fatalf("seed try lock failed: locked=%v err=%v", locked, err)
		}
		time.Sleep(lockTTLWaitForExpiry(driver))
		locked, err = cache.TryLock(key, ttl)
		if err != nil || !locked {
			t.Fatalf("expected try lock after ttl expiry, locked=%v err=%v", locked, err)
		}
	})
}

func runRateLimitHelperInvariantSuite(t *testing.T, cache *Cache, driver Driver, caseKey func(string) string, noOp bool) {
	t.Helper()

	t.Run("rate_limit_monotonic_count_and_remaining_floor", func(t *testing.T) {
		key := caseKey("rl:monotonic")
		limit := int64(3)
		window := rateLimitWindowFor(driver)
		alignRateLimitWindowStart(window)
		var prevCount int64
		for i := 0; i < 4; i++ {
			allowed, count, remaining, resetAt, err := cache.RateLimitWithRemaining(key, limit, window)
			if err != nil {
				t.Fatalf("rate limit call %d failed: %v", i+1, err)
			}
			if remaining < 0 {
				t.Fatalf("remaining should never be negative, got %d", remaining)
			}
			if !resetAt.After(time.Now().Add(-window)) {
				t.Fatalf("resetAt should be near-future, got %v", resetAt)
			}
			if noOp {
				if count != 0 || !allowed {
					t.Fatalf("null store rate limit should stay allowed with count=0, got allowed=%v count=%d", allowed, count)
				}
				continue
			}
			if count <= prevCount {
				t.Fatalf("count must be monotonic within window: prev=%d got=%d", prevCount, count)
			}
			prevCount = count
			if i < 3 && !allowed {
				t.Fatalf("expected allowed before limit exceeded at call %d", i+1)
			}
			if i == 3 && allowed {
				t.Fatalf("expected deny after limit exceeded")
			}
		}
	})

	t.Run("rate_limit_window_rollover_resets_count", func(t *testing.T) {
		key := caseKey("rl:reset")
		window := rateLimitWindowFor(driver)
		allowed, count, err := cache.RateLimit(key, 1, window)
		if err != nil {
			t.Fatalf("first rate limit call failed: %v", err)
		}
		if !noOp && (!allowed || count != 1) {
			t.Fatalf("expected first call allowed count=1, got allowed=%v count=%d", allowed, count)
		}
		allowed, count, err = cache.RateLimit(key, 1, window)
		if err != nil {
			t.Fatalf("second rate limit call failed: %v", err)
		}
		if !noOp && (allowed || count < 2) {
			t.Fatalf("expected second call denied in same window, got allowed=%v count=%d", allowed, count)
		}

		time.Sleep(rateLimitResetWaitFor(driver, window))

		allowed, count, err = cache.RateLimit(key, 1, window)
		if err != nil {
			t.Fatalf("post-reset rate limit call failed: %v", err)
		}
		if noOp {
			if !allowed || count != 0 {
				t.Fatalf("null store expected allowed count=0 after rollover, got allowed=%v count=%d", allowed, count)
			}
			return
		}
		if !allowed || count != 1 {
			t.Fatalf("expected count reset after window rollover, got allowed=%v count=%d", allowed, count)
		}
	})
}

func runRefreshAheadHelperInvariantSuite(t *testing.T, cache *Cache, driver Driver, caseKey func(string) string, noOp bool) {
	t.Helper()
	if noOp {
		return
	}
	ttl, refreshAhead, nearExpirySleep, asyncWait := refreshAheadProfile(driver)

	t.Run("refresh_ahead_miss_writes_value_and_metadata", func(t *testing.T) {
		key := caseKey("ra:miss-meta")
		body, err := cache.RefreshAhead(key, ttl, refreshAhead, func() ([]byte, error) {
			return []byte("v1"), nil
		})
		if err != nil || string(body) != "v1" {
			t.Fatalf("refresh ahead miss failed: body=%q err=%v", string(body), err)
		}
		if _, ok, err := cache.Get(key); err != nil || !ok {
			t.Fatalf("expected value written on miss, ok=%v err=%v", ok, err)
		}
		meta, ok, err := cache.Get(key + refreshMetaSuffix)
		if err != nil || !ok {
			t.Fatalf("expected metadata written on miss, ok=%v err=%v", ok, err)
		}
		if len(meta) == 0 {
			t.Fatalf("expected non-empty refresh metadata")
		}
	})

	t.Run("refresh_ahead_hit_async_success_updates_value", func(t *testing.T) {
		key := caseKey("ra:async-success")
		calls := int64(0)
		_, err := cache.RefreshAhead(key, ttl, refreshAhead, func() ([]byte, error) {
			atomic.AddInt64(&calls, 1)
			return []byte("v1"), nil
		})
		if err != nil {
			t.Fatalf("seed refresh ahead failed: %v", err)
		}
		time.Sleep(nearExpirySleep)

		done := make(chan struct{}, 1)
		body, err := cache.RefreshAhead(key, ttl, refreshAhead, func() ([]byte, error) {
			atomic.AddInt64(&calls, 1)
			select {
			case done <- struct{}{}:
			default:
			}
			return []byte("v2"), nil
		})
		if err != nil {
			t.Fatalf("refresh ahead hit failed: %v", err)
		}
		if got := string(body); got != "v1" {
			t.Fatalf("expected immediate cached value on hit, got %q", got)
		}
		select {
		case <-done:
		case <-time.After(asyncWait):
			t.Fatalf("expected async refresh callback to run")
		}

		deadline := time.Now().Add(asyncWait)
		for {
			got, ok, err := cache.Get(key)
			if err == nil && ok && string(got) == "v2" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("expected refreshed value v2, ok=%v body=%q err=%v", ok, string(got), err)
			}
			time.Sleep(10 * time.Millisecond)
		}
		if c := atomic.LoadInt64(&calls); c < 2 {
			t.Fatalf("expected callback to run twice (seed + async), got %d", c)
		}
	})

	t.Run("refresh_ahead_async_error_keeps_existing_value", func(t *testing.T) {
		key := caseKey("ra:async-error")
		_, err := cache.RefreshAhead(key, ttl, refreshAhead, func() ([]byte, error) {
			return []byte("v1"), nil
		})
		if err != nil {
			t.Fatalf("seed refresh ahead failed: %v", err)
		}
		time.Sleep(nearExpirySleep)

		done := make(chan struct{}, 1)
		body, err := cache.RefreshAhead(key, ttl, refreshAhead, func() ([]byte, error) {
			select {
			case done <- struct{}{}:
			default:
			}
			return nil, errors.New("upstream failed")
		})
		if err != nil {
			t.Fatalf("refresh ahead hit should return cached value even if async callback fails: %v", err)
		}
		if string(body) != "v1" {
			t.Fatalf("expected cached v1 on hit, got %q", string(body))
		}
		select {
		case <-done:
		case <-time.After(asyncWait):
			t.Fatalf("expected async failing callback to run")
		}
		time.Sleep(50 * time.Millisecond)
		got, ok, err := cache.Get(key)
		if err != nil || !ok || string(got) != "v1" {
			t.Fatalf("expected existing value to remain after async error, ok=%v body=%q err=%v", ok, string(got), err)
		}
	})
}

func runRememberStaleDeeperInvariantSuite(t *testing.T, cache *Cache, driver Driver, caseKey func(string) string, noOp bool) {
	t.Helper()

	t.Run("remember_stale_fresh_and_stale_expire_independently", func(t *testing.T) {
		if noOp {
			t.Skip("null store does not persist stale/fresh keys")
		}
		freshTTL, staleTTL, waitFreshExpire, waitStaleExpire := rememberStaleTTLProfile(driver)
		key := caseKey("stale:independent-expiry")
		val, usedStale, err := cache.RememberStaleBytes(key, freshTTL, staleTTL, func() ([]byte, error) {
			return []byte("seed"), nil
		})
		if err != nil || usedStale || string(val) != "seed" {
			t.Fatalf("seed remember stale failed: usedStale=%v val=%q err=%v", usedStale, string(val), err)
		}
		time.Sleep(waitFreshExpire)

		val, usedStale, err = cache.RememberStaleBytes(key, freshTTL, staleTTL, func() ([]byte, error) {
			return nil, errors.New("upstream down")
		})
		if err != nil || !usedStale || string(val) != "seed" {
			t.Fatalf("expected stale fallback after fresh expiry, usedStale=%v val=%q err=%v", usedStale, string(val), err)
		}

		time.Sleep(waitStaleExpire)
		expected := errors.New("upstream down again")
		_, usedStale, err = cache.RememberStaleBytes(key, freshTTL, staleTTL, func() ([]byte, error) {
			return nil, expected
		})
		if usedStale {
			t.Fatalf("expected stale to expire independently")
		}
		if !errors.Is(err, expected) {
			t.Fatalf("expected upstream error after stale expiry, got %v", err)
		}
	})

	t.Run("remember_stale_joins_loader_and_stale_read_errors", func(t *testing.T) {
		if noOp {
			t.Skip("null store does not persist stale key")
		}
		key := caseKey("stale:join-errors")
		seedVal, usedStale, err := cache.RememberStaleBytes(key, time.Second, 2*time.Second, func() ([]byte, error) {
			return []byte("seed"), nil
		})
		if err != nil || usedStale || string(seedVal) != "seed" {
			t.Fatalf("seed remember stale failed: usedStale=%v val=%q err=%v", usedStale, string(seedVal), err)
		}
		if err := cache.Delete(key); err != nil {
			t.Fatalf("delete fresh key failed: %v", err)
		}

		staleGetErr := errors.New("stale read failed")
		wrapped := &getErrorInjectStore{
			inner:  cache.Store(),
			errKey: key + staleSuffix,
			err:    staleGetErr,
		}
		wrappedCache := NewCache(wrapped)
		loaderErr := errors.New("loader failed")
		_, usedStale, err = wrappedCache.RememberStaleBytes(key, time.Second, 2*time.Second, func() ([]byte, error) {
			return nil, loaderErr
		})
		if usedStale {
			t.Fatalf("expected no stale fallback when stale read errors")
		}
		if !errors.Is(err, loaderErr) || !errors.Is(err, staleGetErr) {
			t.Fatalf("expected joined loader+stale errors, got %v", err)
		}
	})
}

func runBatchHelperInvariantSuite(t *testing.T, cache *Cache, driver Driver, caseKey func(string) string, noOp bool) {
	t.Helper()

	t.Run("batch_get_partial_miss_and_empty_inputs", func(t *testing.T) {
		if err := cache.BatchSet(map[string][]byte{
			caseKey("batch:a"): []byte("1"),
			caseKey("batch:b"): []byte("2"),
		}, time.Second); err != nil {
			t.Fatalf("batch set failed: %v", err)
		}
		got, err := cache.BatchGet(caseKey("batch:a"), caseKey("batch:b"), caseKey("batch:missing"))
		if err != nil {
			t.Fatalf("batch get failed: %v", err)
		}
		if noOp {
			if len(got) != 0 {
				t.Fatalf("null store batch get should be empty, got %v", got)
			}
		} else {
			if string(got[caseKey("batch:a")]) != "1" || string(got[caseKey("batch:b")]) != "2" {
				t.Fatalf("unexpected batch get values: %v", got)
			}
			if _, ok := got[caseKey("batch:missing")]; ok {
				t.Fatalf("missing key should be omitted")
			}
		}
		empty, err := cache.BatchGet()
		if err != nil || len(empty) != 0 {
			t.Fatalf("empty batch get should return empty map, len=%d err=%v", len(empty), err)
		}
		if err := cache.BatchSet(map[string][]byte{}, time.Second); err != nil {
			t.Fatalf("empty batch set should succeed, got %v", err)
		}
	})

	t.Run("batch_set_uses_cache_default_ttl_when_non_positive", func(t *testing.T) {
		if noOp {
			t.Skip("null store does not persist values")
		}
		defaultTTL, wait := batchDefaultTTLProfile(driver)
		bc := NewCacheWithTTL(cache.Store(), defaultTTL)
		key := caseKey("batch:default-ttl")
		if err := bc.BatchSet(map[string][]byte{key: []byte("v")}, 0); err != nil {
			t.Fatalf("batch set with default ttl failed: %v", err)
		}
		if _, ok, err := bc.Get(key); err != nil || !ok {
			t.Fatalf("expected batch-set key before expiry, ok=%v err=%v", ok, err)
		}
		time.Sleep(wait)
		if _, ok, err := bc.Get(key); err != nil || ok {
			t.Fatalf("expected batch-set key expired via cache default ttl, ok=%v err=%v", ok, err)
		}
	})
}

func runCounterHelperInvariantSuite(t *testing.T, cache *Cache, driver Driver, caseKey func(string) string, noOp bool) {
	t.Helper()

	t.Run("counter_init_and_signed_delta_semantics", func(t *testing.T) {
		key := caseKey("counter:signed")
		v, err := cache.Increment(key, 3, time.Second)
		if err != nil {
			t.Fatalf("increment failed: %v", err)
		}
		if noOp {
			if v != 0 {
				t.Fatalf("null store increment expected 0, got %d", v)
			}
			return
		}
		if v != 3 {
			t.Fatalf("expected 3 after first increment, got %d", v)
		}
		v, err = cache.Increment(key, -1, time.Second)
		if err != nil || v != 2 {
			t.Fatalf("increment negative delta failed: v=%d err=%v", v, err)
		}
		v, err = cache.Decrement(key, 1, time.Second)
		if err != nil || v != 1 {
			t.Fatalf("decrement failed: v=%d err=%v", v, err)
		}
		v, err = cache.Decrement(key, -2, time.Second)
		if err != nil || v != 3 {
			t.Fatalf("decrement negative delta should increment: v=%d err=%v", v, err)
		}
		v, err = cache.Increment(key, 0, time.Second)
		if err != nil || v != 3 {
			t.Fatalf("zero delta should preserve count: v=%d err=%v", v, err)
		}
	})

	t.Run("counter_ttl_refresh_extends_lifetime", func(t *testing.T) {
		if noOp {
			t.Skip("null store does not persist counters")
		}
		ttl, beforeRefresh, afterOriginalExpiry, afterRefreshedExpiry := counterTTLRefreshProfile(driver)
		key := caseKey("counter:ttl-refresh")
		if _, err := cache.Increment(key, 1, ttl); err != nil {
			t.Fatalf("seed increment failed: %v", err)
		}
		time.Sleep(beforeRefresh)
		if _, err := cache.Increment(key, 1, ttl); err != nil {
			t.Fatalf("refresh increment failed: %v", err)
		}
		time.Sleep(afterOriginalExpiry)
		if _, ok, err := cache.Get(key); err != nil || !ok {
			t.Fatalf("expected counter to survive original ttl due to refresh, ok=%v err=%v", ok, err)
		}
		time.Sleep(afterRefreshedExpiry)
		if _, ok, err := cache.Get(key); err != nil || ok {
			t.Fatalf("expected counter to expire after refreshed ttl, ok=%v err=%v", ok, err)
		}
	})
}

func runDriverFactoryInvariantSuite(t *testing.T, fx storeFactory) {
	t.Helper()

	t.Run("rate_limit_scope_across_store_instances", func(t *testing.T) {
		storeA, cleanupA := fx.new(t)
		t.Cleanup(cleanupA)
		storeB, cleanupB := fx.new(t)
		t.Cleanup(cleanupB)

		driver := storeA.Driver()
		ca := NewCache(storeA)
		cb := NewCache(storeB)
		key := "factory_scope:rl"
		window := rateLimitWindowFor(driver)

		_, countA, err := ca.RateLimit(key, 10, window)
		if err != nil {
			t.Fatalf("rate limit on storeA failed: %v", err)
		}
		_, countB, err := cb.RateLimit(key, 10, window)
		if err != nil {
			t.Fatalf("rate limit on storeB failed: %v", err)
		}

		switch driver {
		case DriverNull:
			if countA != 0 || countB != 0 {
				t.Fatalf("null store should report zero counters, got %d/%d", countA, countB)
			}
		case DriverRedis, DriverMemcached, DriverDynamo, DriverSQL:
			if countA != 1 || countB != 2 {
				t.Fatalf("expected shared backend counters across instances, got countA=%d countB=%d", countA, countB)
			}
		default:
			// memory/file fixtures create isolated backing stores per factory call.
			if countA != 1 || countB != 1 {
				t.Fatalf("expected local/isolated backend counters not to share, got countA=%d countB=%d", countA, countB)
			}
		}
	})

	t.Run("prefix_isolation_delete_and_flush_shared_backends", func(t *testing.T) {
		storeA, cleanupA := fx.new(t, WithPrefix("itest_iso_a"))
		t.Cleanup(cleanupA)
		storeB, cleanupB := fx.new(t, WithPrefix("itest_iso_b"))
		t.Cleanup(cleanupB)
		driver := storeA.Driver()
		if driver != DriverRedis && driver != DriverMemcached && driver != DriverNATS && driver != DriverDynamo && driver != DriverSQL {
			t.Skip("prefix isolation is only meaningful for shared/prefixed backends")
		}

		ca := NewCache(storeA)
		cb := NewCache(storeB)
		key := "prefix:shared"
		if err := ca.Set(key, []byte("A"), time.Second); err != nil {
			t.Fatalf("set A failed: %v", err)
		}
		if err := cb.Set(key, []byte("B"), time.Second); err != nil {
			t.Fatalf("set B failed: %v", err)
		}
		if got, ok, err := ca.Get(key); err != nil || !ok || string(got) != "A" {
			t.Fatalf("expected prefix A value, ok=%v body=%q err=%v", ok, string(got), err)
		}
		if got, ok, err := cb.Get(key); err != nil || !ok || string(got) != "B" {
			t.Fatalf("expected prefix B value, ok=%v body=%q err=%v", ok, string(got), err)
		}

		if err := ca.Delete(key); err != nil {
			t.Fatalf("delete in prefix A failed: %v", err)
		}
		if _, ok, err := ca.Get(key); err != nil || ok {
			t.Fatalf("expected key deleted in prefix A only, ok=%v err=%v", ok, err)
		}
		if got, ok, err := cb.Get(key); err != nil || !ok || string(got) != "B" {
			t.Fatalf("expected prefix B key untouched by prefix A delete, ok=%v body=%q err=%v", ok, string(got), err)
		}

		if err := cb.Flush(); err != nil {
			t.Fatalf("flush prefix B failed: %v", err)
		}
		if _, ok, err := cb.Get(key); err != nil || ok {
			t.Fatalf("expected prefix B flush to clear its key only, ok=%v err=%v", ok, err)
		}
	})

	t.Run("shape_option_roundtrip_and_corruption_paths", func(t *testing.T) {
		storeRaw, cleanupRaw := fx.new(t, WithPrefix("itest_shape_raw"))
		t.Cleanup(cleanupRaw)
		storeCombo, cleanupCombo := fx.new(t,
			WithPrefix("itest_shape_combo"),
			WithCompression(CompressionGzip),
			WithEncryptionKey([]byte("0123456789abcdef0123456789abcdef")),
		)
		t.Cleanup(cleanupCombo)

		driver := storeRaw.Driver()
		if driver == DriverNull {
			t.Skip("null store does not persist shaped payloads")
		}

		comboCache := NewCache(storeCombo)

		if err := comboCache.Set("shape:combo", []byte("payload"), time.Second); err != nil {
			t.Fatalf("combo set failed: %v", err)
		}
		got, ok, err := comboCache.Get("shape:combo")
		if err != nil || !ok || string(got) != "payload" {
			t.Fatalf("combo round-trip failed: ok=%v body=%q err=%v", ok, string(got), err)
		}

		storeMax, cleanupMax := fx.new(t, WithPrefix("itest_shape_max"), WithMaxValueBytes(16))
		t.Cleanup(cleanupMax)
		maxCache := NewCache(storeMax)
		if err := maxCache.Set("shape:max:eq", []byte("1234567890abcdef"), time.Second); err != nil {
			t.Fatalf("expected exact max value to succeed, got %v", err)
		}
		if err := maxCache.Set("shape:max:gt", []byte("1234567890abcdefg"), time.Second); !errors.Is(err, ErrValueTooLarge) {
			t.Fatalf("expected ErrValueTooLarge for max+1, got %v", err)
		}

		storeCorruptBase, cleanupCorruptBase := fx.new(t, WithPrefix("itest_shape_corrupt"))
		t.Cleanup(cleanupCorruptBase)
		rawCompressedCache := NewCache(storeCorruptBase)
		compressedStore := newShapingStore(storeCorruptBase, CompressionGzip, 0)
		compressedCache := NewCache(compressedStore)
		if err := rawCompressedCache.Set("shape:corrupt:gzip", []byte("CMP1gnot-gzip"), time.Second); err != nil {
			t.Fatalf("seed corrupt compressed payload failed: %v", err)
		}
		if _, ok, err := compressedCache.Get("shape:corrupt:gzip"); !errors.Is(err, ErrCorruptCompression) || ok {
			t.Fatalf("expected corrupt compression error, ok=%v err=%v", ok, err)
		}

		storeEncCorruptBase, cleanupEncCorruptBase := fx.new(t, WithPrefix("itest_shape_enc_corrupt"))
		t.Cleanup(cleanupEncCorruptBase)
		rawEncryptedCache := NewCache(storeEncCorruptBase)
		encryptedStore, err := newEncryptingStore(storeEncCorruptBase, []byte("0123456789abcdef0123456789abcdef"))
		if err != nil {
			t.Fatalf("construct encrypting wrapper failed: %v", err)
		}
		encryptedCache := NewCache(encryptedStore)
		if err := rawEncryptedCache.Set("shape:corrupt:enc", []byte{'E', 'N', 'C', '1', 12, 1, 2, 3}, time.Second); err != nil {
			t.Fatalf("seed corrupt encrypted payload failed: %v", err)
		}
		if _, ok, err := encryptedCache.Get("shape:corrupt:enc"); !errors.Is(err, ErrDecryptFailed) || ok {
			t.Fatalf("expected decrypt failure, ok=%v err=%v", ok, err)
		}
	})
}

type getErrorInjectStore struct {
	inner  Store
	errKey string
	err    error
}

func (s *getErrorInjectStore) Driver() Driver { return s.inner.Driver() }

func (s *getErrorInjectStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if key == s.errKey {
		return nil, false, s.err
	}
	return s.inner.Get(ctx, key)
}

func (s *getErrorInjectStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.inner.Set(ctx, key, value, ttl)
}

func (s *getErrorInjectStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	return s.inner.Add(ctx, key, value, ttl)
}

func (s *getErrorInjectStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Increment(ctx, key, delta, ttl)
}

func (s *getErrorInjectStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Decrement(ctx, key, delta, ttl)
}

func (s *getErrorInjectStore) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}

func (s *getErrorInjectStore) DeleteMany(ctx context.Context, keys ...string) error {
	return s.inner.DeleteMany(ctx, keys...)
}

func (s *getErrorInjectStore) Flush(ctx context.Context) error { return s.inner.Flush(ctx) }

func lockTTLFor(driver Driver) time.Duration {
	if driver == DriverMemcached {
		return time.Second
	}
	return 300 * time.Millisecond
}

func lockTTLWaitForExpiry(driver Driver) time.Duration {
	if driver == DriverMemcached {
		return 1500 * time.Millisecond
	}
	return 400 * time.Millisecond
}

func rateLimitWindowFor(driver Driver) time.Duration {
	switch driver {
	case DriverRedis:
		return 1100 * time.Millisecond
	case DriverDynamo:
		return 2 * time.Second
	case DriverSQL:
		return time.Second
	default:
		return 120 * time.Millisecond
	}
}

func rateLimitResetWaitFor(_ Driver, window time.Duration) time.Duration {
	// Sleep > 2x to avoid boundary flake when test enters near the end of a bucket.
	return (2 * window) + 20*time.Millisecond
}

func alignRateLimitWindowStart(window time.Duration) {
	if window <= 0 {
		return
	}
	now := time.Now()
	mod := now.UnixNano() % window.Nanoseconds()
	remaining := time.Duration(window.Nanoseconds() - mod)
	// If we're near the end of a bucket, wait into the next bucket so the
	// monotonic sequence has nearly a full window to complete.
	if remaining < window/2 {
		time.Sleep(remaining + 10*time.Millisecond)
	}
}

func refreshAheadProfile(driver Driver) (ttl, refreshAhead, nearExpirySleep, asyncWait time.Duration) {
	if driver == DriverMemcached {
		return 2 * time.Second, 1500 * time.Millisecond, 700 * time.Millisecond, 3 * time.Second
	}
	return 300 * time.Millisecond, 250 * time.Millisecond, 80 * time.Millisecond, 2 * time.Second
}

func rememberStaleTTLProfile(driver Driver) (freshTTL, staleTTL, waitFreshExpire, waitStaleExpire time.Duration) {
	if driver == DriverMemcached {
		return time.Second, 4 * time.Second, 1500 * time.Millisecond, 3 * time.Second
	}
	return 80 * time.Millisecond, 240 * time.Millisecond, 120 * time.Millisecond, 180 * time.Millisecond
}

func batchDefaultTTLProfile(driver Driver) (defaultTTL, wait time.Duration) {
	if driver == DriverMemcached {
		return time.Second, 1500 * time.Millisecond
	}
	return 70 * time.Millisecond, 120 * time.Millisecond
}

func counterTTLRefreshProfile(driver Driver) (ttl, beforeRefresh, afterOriginalExpiry, afterRefreshedExpiry time.Duration) {
	if driver == DriverRedis {
		return time.Second, 700 * time.Millisecond, 700 * time.Millisecond, 700 * time.Millisecond
	}
	if driver == DriverMemcached {
		// Memcached expirations are second-granularity; use a larger ttl and waits
		// to avoid edge timing around second boundaries after touch-based refresh.
		return 2 * time.Second, 1200 * time.Millisecond, 1000 * time.Millisecond, 1500 * time.Millisecond
	}
	return 120 * time.Millisecond, 70 * time.Millisecond, 90 * time.Millisecond, 140 * time.Millisecond
}

func integrationContractCases() []contractCase {
	encryptionKey := []byte("0123456789abcdef0123456789abcdef")
	return []contractCase{
		{name: "baseline"},
		{
			name: "with_prefix",
			opts: []StoreOption{WithPrefix("itest_opt")},
		},
		{
			name: "with_compression",
			opts: []StoreOption{WithCompression(CompressionGzip)},
		},
		{
			name: "with_encryption",
			opts: []StoreOption{WithEncryptionKey(encryptionKey)},
		},
		{
			name: "with_prefix_compression_encryption",
			opts: []StoreOption{
				WithPrefix("itest_opt_combo"),
				WithCompression(CompressionGzip),
				WithEncryptionKey(encryptionKey),
			},
		},
		{
			name:                "with_max_value_bytes",
			opts:                []StoreOption{WithMaxValueBytes(16)},
			verifyMaxValueLimit: true,
		},
		{
			name:                   "with_default_ttl",
			opts:                   []StoreOption{WithDefaultTTL(60 * time.Millisecond)},
			verifyDefaultTTLExpiry: true,
		},
	}
}

func applyStoreOptions(cfg StoreConfig, opts ...StoreOption) StoreConfig {
	for _, opt := range opts {
		cfg = opt(cfg)
	}
	return cfg
}

func integrationFixtures(t *testing.T) []storeFactory {
	t.Helper()

	var fixtures []storeFactory

	if integrationDriverEnabled("null") {
		fixtures = append(fixtures, storeFactory{
			name: "null",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				store := NewNullStore(context.Background(), opts...)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("file") {
		fixtures = append(fixtures, storeFactory{
			name: "file",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				dir := t.TempDir()
				cfg := applyStoreOptions(StoreConfig{
					Driver:     DriverFile,
					DefaultTTL: 2 * time.Second,
					FileDir:    dir,
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("memory") {
		fixtures = append(fixtures, storeFactory{
			name: "memory",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				cfg := applyStoreOptions(StoreConfig{
					Driver:                DriverMemory,
					DefaultTTL:            2 * time.Second,
					MemoryCleanupInterval: time.Second,
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("redis") {
		addr := integrationAddr("redis")
		if addr == "" {
			t.Fatalf("redis integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "redis",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				client := redis.NewClient(&redis.Options{Addr: addr})
				cfg := applyStoreOptions(StoreConfig{
					Driver:      DriverRedis,
					DefaultTTL:  2 * time.Second,
					Prefix:      "itest",
					RedisClient: client,
				}, opts...)
				store := NewStore(context.Background(), cfg)
				cleanup := func() { _ = client.Close() }
				return store, cleanup
			},
		})
	}

	if integrationDriverEnabled("nats") {
		addr := integrationAddr("nats")
		if addr == "" {
			t.Fatalf("nats integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "nats",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				nc, err := nats.Connect("nats://" + addr)
				if err != nil {
					t.Fatalf("connect nats: %v", err)
				}
				js, err := nc.JetStream()
				if err != nil {
					_ = nc.Drain()
					nc.Close()
					t.Fatalf("jetstream nats: %v", err)
				}
				bucket := integrationNATSBucketName(t.Name())
				kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
					Bucket:  bucket,
					History: 1,
				})
				if err != nil {
					_ = nc.Drain()
					nc.Close()
					t.Fatalf("create nats kv bucket: %v", err)
				}
				cfg := applyStoreOptions(StoreConfig{
					Driver:       DriverNATS,
					DefaultTTL:   2 * time.Second,
					Prefix:       "itest",
					NATSKeyValue: kv,
				}, opts...)
				store := NewStore(context.Background(), cfg)
				cleanup := func() {
					_ = js.DeleteKeyValue(bucket)
					_ = nc.Drain()
					nc.Close()
				}
				return store, cleanup
			},
		})
	}

	if integrationDriverEnabled("memcached") {
		addr := integrationAddr("memcached")
		if addr == "" {
			t.Fatalf("memcached integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "memcached",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				cfg := applyStoreOptions(StoreConfig{
					Driver:             DriverMemcached,
					DefaultTTL:         2 * time.Second,
					Prefix:             "itest",
					MemcachedAddresses: []string{addr},
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("dynamodb") {
		endpoint := integrationAddr("dynamodb")
		if endpoint == "" {
			t.Fatalf("dynamodb integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "dynamodb",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				cfg := applyStoreOptions(StoreConfig{
					Driver:         DriverDynamo,
					DefaultTTL:     2 * time.Second,
					Prefix:         "itest",
					DynamoEndpoint: endpoint,
					DynamoRegion:   "us-east-1",
					DynamoTable:    "cache_entries",
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("sql_sqlite") {
		fixtures = append(fixtures, storeFactory{
			name: "sql_sqlite",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				cfg := applyStoreOptions(StoreConfig{
					Driver:        DriverSQL,
					DefaultTTL:    2 * time.Second,
					SQLDriverName: "sqlite",
					SQLDSN:        "file::memory:?cache=shared",
					SQLTable:      "cache_entries",
					Prefix:        "itest",
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("sql_postgres") {
		addr := integrationAddr("sql_postgres")
		if addr == "" {
			t.Fatalf("sql_postgres integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "sql_postgres",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				dsn := "postgres://user:pass@" + addr + "/app?sslmode=disable"
				cfg := applyStoreOptions(StoreConfig{
					Driver:        DriverSQL,
					DefaultTTL:    2 * time.Second,
					SQLDriverName: "pgx",
					SQLDSN:        dsn,
					SQLTable:      "cache_entries",
					Prefix:        "itest",
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	if integrationDriverEnabled("sql_mysql") {
		addr := integrationAddr("sql_mysql")
		if addr == "" {
			t.Fatalf("sql_mysql integration requested but no address available")
		}
		fixtures = append(fixtures, storeFactory{
			name: "sql_mysql",
			new: func(t *testing.T, opts ...StoreOption) (Store, func()) {
				dsn := "user:pass@tcp(" + addr + ")/app?parseTime=true"
				cfg := applyStoreOptions(StoreConfig{
					Driver:        DriverSQL,
					DefaultTTL:    2 * time.Second,
					SQLDriverName: "mysql",
					SQLDSN:        dsn,
					SQLTable:      "cache_entries",
					Prefix:        "itest",
				}, opts...)
				store := NewStore(context.Background(), cfg)
				return store, func() {}
			},
		})
	}

	return fixtures
}

func integrationNATSBucketName(name string) string {
	normalized := strings.ToUpper(name)
	var b strings.Builder
	for _, r := range normalized {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	base := b.String()
	if len(base) > 36 {
		base = base[len(base)-36:]
	}
	return fmt.Sprintf("CACHE_%s_%d", base, time.Now().UnixNano()%1_000_000)
}
