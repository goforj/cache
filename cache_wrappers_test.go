package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

// TestGenericTypedWrappers verifies typed cache helpers encode and decode values consistently.
func TestGenericTypedWrappers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := NewCache(NewMemoryStore(ctx))

	type payload struct {
		Name string `json:"name"`
	}

	if err := Set(c, "typed:set", payload{Name: "Ada"}, time.Second); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if err := Set(c.WithContext(ctx), "typed:setctx", payload{Name: "Grace"}, time.Second); err != nil {
		t.Fatalf("Set with bound context failed: %v", err)
	}

	got, ok, err := Get[payload](c, "typed:set")
	if err != nil || !ok || got.Name != "Ada" {
		t.Fatalf("Get failed: ok=%v got=%+v err=%v", ok, got, err)
	}
	got, ok, err = Get[payload](c.WithContext(ctx), "typed:setctx")
	if err != nil || !ok || got.Name != "Grace" {
		t.Fatalf("Get with bound context failed: ok=%v got=%+v err=%v", ok, got, err)
	}

	if err := Set(c, "typed:pull", payload{Name: "Linus"}, time.Second); err != nil {
		t.Fatalf("seed pull failed: %v", err)
	}
	pulled, ok, err := Pull[payload](c, "typed:pull")
	if err != nil || !ok || pulled.Name != "Linus" {
		t.Fatalf("Pull failed: ok=%v got=%+v err=%v", ok, pulled, err)
	}
	pulled, ok, err = Pull[payload](c.WithContext(ctx), "typed:pull")
	if err != nil || ok {
		t.Fatalf("Pull with bound context miss expected after pull: ok=%v got=%+v err=%v", ok, pulled, err)
	}
}

// TestGenericRefreshAheadAndRememberStaleWrappers verifies typed stale-value helpers preserve callback values.
func TestGenericRefreshAheadAndRememberStaleWrappers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := NewCache(NewMemoryStore(ctx))

	type payload struct {
		Name string `json:"name"`
	}

	v, err := RefreshAhead[payload](c, "ra:typed", time.Second, 200*time.Millisecond, func() (payload, error) {
		return payload{Name: "Ada"}, nil
	})
	if err != nil || v.Name != "Ada" {
		t.Fatalf("RefreshAhead failed: v=%+v err=%v", v, err)
	}
	v, err = RefreshAhead[payload](c.WithContext(ctx), "ra:typed", time.Second, 200*time.Millisecond, func() (payload, error) {
		return payload{Name: "Grace"}, nil
	})
	if err != nil || v.Name != "Ada" {
		t.Fatalf("RefreshAhead with bound context cached path failed: v=%+v err=%v", v, err)
	}

	rs, usedStale, err := RememberStale[payload](c, "rs:typed", time.Second, 2*time.Second, func() (payload, error) {
		return payload{Name: "Linus"}, nil
	})
	if err != nil || usedStale || rs.Name != "Linus" {
		t.Fatalf("RememberStale failed: usedStale=%v v=%+v err=%v", usedStale, rs, err)
	}
	rs, usedStale, err = RememberStale[payload](c.WithContext(ctx), "rs:typed", time.Second, 2*time.Second, func() (payload, error) {
		return payload{Name: "Other"}, nil
	})
	if err != nil || usedStale || rs.Name != "Linus" {
		t.Fatalf("RememberStale with bound context cached path failed: usedStale=%v v=%+v err=%v", usedStale, rs, err)
	}
}

// TestGenericWrapperErrorBranches verifies typed wrappers propagate store and codec failures.
func TestGenericWrapperErrorBranches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := NewCache(NewMemoryStore(ctx))

	type payload struct {
		Name string `json:"name"`
	}

	// Decode error path for bound-context Get/Pull.
	if err := c.SetBytes("bad:json", []byte("not-json"), time.Second); err != nil {
		t.Fatalf("seed bad json failed: %v", err)
	}
	if _, ok, err := Get[payload](c.WithContext(ctx), "bad:json"); err == nil || ok {
		t.Fatalf("expected bound-context Get decode error, ok=%v err=%v", ok, err)
	}
	if _, ok, err := Pull[payload](c.WithContext(ctx), "bad:json"); err == nil || ok {
		t.Fatalf("expected bound-context Pull decode error, ok=%v err=%v", ok, err)
	}

	// Encode error path for Set using unsupported JSON type.
	if err := Set[func()](c.WithContext(ctx), "bad:set", func() {}, time.Second); err == nil {
		t.Fatalf("expected bound-context Set encode error for func value")
	}

	// Nil callback guards for typed wrappers.
	if _, err := RefreshAhead[payload](c, "ra:nil", time.Second, 200*time.Millisecond, nil); err == nil {
		t.Fatalf("expected RefreshAhead nil callback error")
	}
	if _, err := RefreshAhead[payload](c.WithContext(ctx), "ra:nilctx", time.Second, 200*time.Millisecond, nil); err == nil {
		t.Fatalf("expected RefreshAhead with bound context nil callback error")
	}
	if _, _, err := RememberStale[payload](c, "rs:nil", time.Second, 2*time.Second, nil); err == nil {
		t.Fatalf("expected RememberStale nil callback error")
	}
	if _, _, err := RememberStale[payload](c.WithContext(ctx), "rs:nilctx", time.Second, 2*time.Second, nil); err == nil {
		t.Fatalf("expected RememberStale with bound context nil callback error")
	}
}

// TestObserverFuncAndErrorStoreDriver verifies function observers and failing stores expose their configured behavior.
func TestObserverFuncAndErrorStoreDriver(t *testing.T) {
	t.Parallel()

	// Nil ObserverFunc should be a no-op.
	var nilObs ObserverFunc
	nilObs.OnCacheOp(context.Background(), CacheOpEvent{Operation: "get", Key: "k", Driver: cachecore.DriverMemory})

	called := false
	ObserverFunc(func(ctx context.Context, event CacheOpEvent) {
		called = true
		if event.Operation != "set" || event.Key != "k" || event.Driver != cachecore.DriverMemory {
			t.Fatalf("unexpected observer payload")
		}
	}).OnCacheOp(context.Background(), CacheOpEvent{Operation: "set", Key: "k", Hit: true, Duration: time.Millisecond, Driver: cachecore.DriverMemory})
	if !called {
		t.Fatalf("observer func was not called")
	}

	e := &errorStore{driver: cachecore.DriverRedis, err: errors.New("boom")}
	if got := e.Driver(); got != cachecore.DriverRedis {
		t.Fatalf("expected driver=%q got=%q", cachecore.DriverRedis, got)
	}
}

// TestEncryptingStoreFlushDelegates verifies encryption leaves whole-store flushing to the wrapped store.
func TestEncryptingStoreFlushDelegates(t *testing.T) {
	t.Parallel()
	base := &spyStore{driver: cachecore.DriverMemory}
	s, err := cachecore.WrapStore(base, cachecore.BaseConfig{EncryptionKey: []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatalf("WrapStore failed: %v", err)
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
}

// TestNewStoreForDriverBranches verifies driver selection succeeds for built-ins and rejects unknown names.
func TestNewStoreForDriverBranches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for _, tc := range []struct {
		name   string
		driver cachecore.Driver
		want   cachecore.Driver
	}{
		{"memory", cachecore.DriverMemory, cachecore.DriverMemory},
		{"file", cachecore.DriverFile, cachecore.DriverFile},
		{"null", cachecore.DriverNull, cachecore.DriverNull},
		{"redis_removed", cachecore.DriverRedis, cachecore.DriverRedis},
		{"nats_removed", cachecore.DriverNATS, cachecore.DriverNATS},
		{"memcached_removed", cachecore.DriverMemcached, cachecore.DriverMemcached},
		{"dynamo_removed", cachecore.DriverDynamo, cachecore.DriverDynamo},
		{"sql_removed", cachecore.DriverSQL, cachecore.DriverSQL},
		{"unknown", cachecore.Driver("wat"), cachecore.Driver("wat")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := StoreConfig{}
			if tc.driver == cachecore.DriverFile {
				cfg.FileDir = t.TempDir()
			}
			store := newStoreForDriver(ctx, tc.driver, cfg)
			if got := store.Driver(); got != tc.want {
				t.Fatalf("expected driver=%q got=%q", tc.want, got)
			}
		})
	}

	// Invalid encryption config returns an errorStore branch from newStoreForDriver.
	store := newStoreForDriver(ctx, cachecore.DriverMemory, StoreConfig{
		BaseConfig: cachecore.BaseConfig{EncryptionKey: []byte("short")},
	})
	if _, ok, err := store.Get(ctx, "k"); err == nil || ok {
		t.Fatalf("expected errorStore get failure for invalid encryption config, ok=%v err=%v", ok, err)
	}
}
