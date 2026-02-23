//go:build integration

package cache_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
)

func TestIntegrationBackendFaultRecovery_AllDrivers(t *testing.T) {
	fixtures := integrationFixtures(t)
	ran := false
	for _, fx := range fixtures {
		fx := fx
		rt := integrationRuntimeFor(fx.name)
		if rt == nil || rt.container == nil {
			continue // only container-backed drivers
		}
		ran = true

		t.Run(fx.name, func(t *testing.T) {
			testBackendFaultRecoveryForDriver(t, fx)
		})
	}
	if !ran {
		t.Skip("no container-backed drivers selected for integration/root fault recovery suite")
	}
}

func testBackendFaultRecoveryForDriver(t *testing.T, fx storeFactory) {
	t.Helper()
	store, cleanup := fx.new(t)
	t.Cleanup(cleanup)
	cache := NewCache(store)
	driver := store.Driver()

	preKey := "fault:preflight"
	if err := cache.SetBytes(preKey, []byte("ok"), time.Second); err != nil {
		t.Fatalf("preflight set failed before outage: %v", err)
	}
	if got, ok, err := cache.GetBytes(preKey); err != nil || !ok || string(got) != "ok" {
		t.Fatalf("preflight get failed before outage: ok=%v body=%q err=%v", ok, string(got), err)
	}

	lockKey := "fault:lock"
	if locked, err := cache.TryLock(lockKey, lockTTLFor(driver)); err != nil || !locked {
		t.Fatalf("preflight try lock failed before outage: locked=%v err=%v", locked, err)
	}

	if err := restartIntegrationBackend(t, fx.name); err != nil {
		t.Fatalf("restart backend %s failed: %v", fx.name, err)
	}

	// During outage semantics are checked inside restartIntegrationBackend (operations against the
	// backend should fail while stopped). After restart, verify recovery with a fresh store instance.
	waitForDriverRecovery(t, fx)

	postStore, postCleanup := fx.new(t)
	t.Cleanup(postCleanup)
	postCache := NewCache(postStore)

	t.Run("post_recovery_set_get_roundtrip", func(t *testing.T) {
		key := "fault:recovery:roundtrip"
		if err := postCache.SetBytes(key, []byte("alive"), time.Second); err != nil {
			t.Fatalf("set after recovery failed: %v", err)
		}
		got, ok, err := postCache.GetBytes(key)
		if err != nil || !ok || string(got) != "alive" {
			t.Fatalf("get after recovery failed: ok=%v body=%q err=%v", ok, string(got), err)
		}
	})

	t.Run("post_recovery_lock_not_stuck", func(t *testing.T) {
		key := "fault:recovery:lock"
		ttl := lockTTLFor(driver)
		locked, err := postCache.TryLock(key, ttl)
		if err != nil || !locked {
			t.Fatalf("try lock after recovery failed: locked=%v err=%v", locked, err)
		}
		time.Sleep(lockTTLWaitForExpiry(driver))
		locked, err = postCache.TryLock(key, ttl)
		if err != nil || !locked {
			t.Fatalf("lock appears stuck after recovery/ttl expiry: locked=%v err=%v", locked, err)
		}
		_ = postCache.Unlock(key)
	})

	t.Run("post_recovery_refresh_ahead_and_stale_paths", func(t *testing.T) {
		refreshKey := "fault:recovery:refresh"
		var refreshCalls atomic.Int64
		body, err := postCache.RefreshAheadBytes(refreshKey, time.Second, 200*time.Millisecond, func() ([]byte, error) {
			refreshCalls.Add(1)
			return []byte("seed"), nil
		})
		if err != nil || string(body) != "seed" {
			t.Fatalf("refresh ahead miss after recovery failed: body=%q err=%v", string(body), err)
		}
		body, err = postCache.RefreshAheadBytes(refreshKey, time.Second, 200*time.Millisecond, func() ([]byte, error) {
			refreshCalls.Add(1)
			return []byte("new"), nil
		})
		if err != nil || string(body) != "seed" {
			t.Fatalf("refresh ahead hit after recovery failed: body=%q err=%v", string(body), err)
		}
		time.Sleep(80 * time.Millisecond)
		if calls := refreshCalls.Load(); calls < 1 || calls > 2 {
			t.Fatalf("unexpected refresh callback count after recovery: %d", calls)
		}

		staleKey := "fault:recovery:stale"
		val, usedStale, err := postCache.RememberStaleBytes(staleKey, time.Second, 2*time.Second, func() ([]byte, error) {
			return []byte("stable"), nil
		})
		if err != nil || usedStale || string(val) != "stable" {
			t.Fatalf("remember stale seed after recovery failed: usedStale=%v val=%q err=%v", usedStale, string(val), err)
		}
		if err := postCache.Delete(staleKey); err != nil {
			t.Fatalf("delete fresh key after recovery failed: %v", err)
		}
		expected := errors.New("upstream down")
		val, usedStale, err = postCache.RememberStaleBytes(staleKey, time.Second, 2*time.Second, func() ([]byte, error) {
			return nil, expected
		})
		if err != nil || !usedStale || string(val) != "stable" {
			t.Fatalf("remember stale fallback after recovery failed: usedStale=%v val=%q err=%v", usedStale, string(val), err)
		}
	})
}

func restartIntegrationBackend(t *testing.T, name string) error {
	t.Helper()
	rt := integrationRuntimeFor(name)
	if rt == nil || rt.container == nil {
		return fmt.Errorf("no container runtime for %s", name)
	}

	// Validate "during outage" on a fresh store while the backend is stopped.
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStop()
	stopTimeout := 2 * time.Second
	if err := rt.container.Stop(stopCtx, &stopTimeout); err != nil {
		return fmt.Errorf("stop container: %w", err)
	}

	// Probe outage behavior using a short timeout and a fresh store instance for this driver.
	if err := assertBackendOutageErrors(t, name); err != nil {
		_ = rt.container.Start(context.Background())
		return err
	}

	startCtx, cancelStart := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelStart()
	if err := rt.container.Start(startCtx); err != nil {
		return fmt.Errorf("start container: %w", err)
	}
	return nil
}

func assertBackendOutageErrors(t *testing.T, driverName string) error {
	t.Helper()

	// Find the matching fixture so we exercise the real driver path (client setup, wrappers, etc).
	var fx *storeFactory
	for _, cand := range integrationFixtures(t) {
		if cand.name == driverName {
			c := cand
			fx = &c
			break
		}
	}
	if fx == nil {
		return fmt.Errorf("fixture %s not found", driverName)
	}

	store, cleanup := fx.new(t)
	defer cleanup()
	cache := NewCache(store)

	timeout := 300 * time.Millisecond
	if store.Driver() == cachecore.DriverDynamo || store.Driver() == cachecore.DriverSQL {
		timeout = 900 * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	start := time.Now()
	_, ok, err := cache.GetBytesCtx(ctx, "fault:outage:get")
	elapsed := time.Since(start)
	if elapsed > 2*time.Second {
		return fmt.Errorf("GetCtx during outage returned too slowly: %v", elapsed)
	}
	if err == nil {
		return fmt.Errorf("expected GetCtx outage error, got ok=%v err=nil", ok)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
	defer cancel2()
	start = time.Now()
	err = cache.SetBytesCtx(ctx2, "fault:outage:set", []byte("x"), time.Second)
	elapsed = time.Since(start)
	if elapsed > 2*time.Second {
		return fmt.Errorf("SetCtx during outage returned too slowly: %v", elapsed)
	}
	if err == nil {
		return fmt.Errorf("expected SetCtx outage error, got nil")
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), timeout)
	defer cancel3()
	start = time.Now()
	locked, err := cache.LockCtx(ctx3, "fault:outage:lock", time.Second, 25*time.Millisecond)
	elapsed = time.Since(start)
	if elapsed > 2*time.Second {
		return fmt.Errorf("LockCtx during outage returned too slowly: %v", elapsed)
	}
	if err == nil || locked {
		return fmt.Errorf("expected LockCtx outage error, got locked=%v err=%v", locked, err)
	}

	return nil
}

func waitForDriverRecovery(t *testing.T, fx storeFactory) {
	t.Helper()

	deadline := time.Now().Add(25 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		store, cleanup := fx.new(t)
		cache := NewCache(store)
		key := fmt.Sprintf("fault:recovery:probe:%d", time.Now().UnixNano())
		err := cache.SetBytes(key, []byte("ok"), time.Second)
		if err == nil {
			if got, ok, getErr := cache.GetBytes(key); getErr == nil && ok && string(got) == "ok" {
				cleanup()
				return
			} else {
				lastErr = fmt.Errorf("get probe failed: ok=%v err=%v", ok, getErr)
			}
		} else {
			lastErr = err
		}
		cleanup()
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("backend did not recover in time for driver %s: lastErr=%v", fx.name, lastErr)
}

func integrationRuntimeFor(name string) *integrationRuntime {
	return integrationRuntimes[strings.ToLower(name)]
}
