package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

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
	if err := SetCtx(ctx, c, "typed:setctx", payload{Name: "Grace"}, time.Second); err != nil {
		t.Fatalf("SetCtx failed: %v", err)
	}

	got, ok, err := Get[payload](c, "typed:set")
	if err != nil || !ok || got.Name != "Ada" {
		t.Fatalf("Get failed: ok=%v got=%+v err=%v", ok, got, err)
	}
	got, ok, err = GetCtx[payload](ctx, c, "typed:setctx")
	if err != nil || !ok || got.Name != "Grace" {
		t.Fatalf("GetCtx failed: ok=%v got=%+v err=%v", ok, got, err)
	}

	if err := Set(c, "typed:pull", payload{Name: "Linus"}, time.Second); err != nil {
		t.Fatalf("seed pull failed: %v", err)
	}
	pulled, ok, err := Pull[payload](c, "typed:pull")
	if err != nil || !ok || pulled.Name != "Linus" {
		t.Fatalf("Pull failed: ok=%v got=%+v err=%v", ok, pulled, err)
	}
	pulled, ok, err = PullCtx[payload](ctx, c, "typed:pull")
	if err != nil || ok {
		t.Fatalf("PullCtx miss expected after pull: ok=%v got=%+v err=%v", ok, pulled, err)
	}
}

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
	v, err = RefreshAheadCtx[payload](ctx, c, "ra:typed", time.Second, 200*time.Millisecond, func(context.Context) (payload, error) {
		return payload{Name: "Grace"}, nil
	})
	if err != nil || v.Name != "Ada" {
		t.Fatalf("RefreshAheadCtx cached path failed: v=%+v err=%v", v, err)
	}

	rs, usedStale, err := RememberStale[payload](c, "rs:typed", time.Second, 2*time.Second, func() (payload, error) {
		return payload{Name: "Linus"}, nil
	})
	if err != nil || usedStale || rs.Name != "Linus" {
		t.Fatalf("RememberStale failed: usedStale=%v v=%+v err=%v", usedStale, rs, err)
	}
	rs, usedStale, err = RememberStaleCtx[payload](ctx, c, "rs:typed", time.Second, 2*time.Second, func(context.Context) (payload, error) {
		return payload{Name: "Other"}, nil
	})
	if err != nil || usedStale || rs.Name != "Linus" {
		t.Fatalf("RememberStaleCtx cached path failed: usedStale=%v v=%+v err=%v", usedStale, rs, err)
	}
}

func TestGenericWrapperErrorBranches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := NewCache(NewMemoryStore(ctx))

	type payload struct {
		Name string `json:"name"`
	}

	// Decode error path for GetCtx/PullCtx.
	if err := c.SetBytes("bad:json", []byte("not-json"), time.Second); err != nil {
		t.Fatalf("seed bad json failed: %v", err)
	}
	if _, ok, err := GetCtx[payload](ctx, c, "bad:json"); err == nil || ok {
		t.Fatalf("expected GetCtx decode error, ok=%v err=%v", ok, err)
	}
	if _, ok, err := PullCtx[payload](ctx, c, "bad:json"); err == nil || ok {
		t.Fatalf("expected PullCtx decode error, ok=%v err=%v", ok, err)
	}

	// Encode error path for SetCtx using unsupported JSON type.
	if err := SetCtx[func()](ctx, c, "bad:set", func() {}, time.Second); err == nil {
		t.Fatalf("expected SetCtx encode error for func value")
	}

	// Nil callback guards for typed wrappers.
	if _, err := RefreshAhead[payload](c, "ra:nil", time.Second, 200*time.Millisecond, nil); err == nil {
		t.Fatalf("expected RefreshAhead nil callback error")
	}
	if _, err := RefreshAheadCtx[payload](ctx, c, "ra:nilctx", time.Second, 200*time.Millisecond, nil); err == nil {
		t.Fatalf("expected RefreshAheadCtx nil callback error")
	}
	if _, _, err := RememberStale[payload](c, "rs:nil", time.Second, 2*time.Second, nil); err == nil {
		t.Fatalf("expected RememberStale nil callback error")
	}
	if _, _, err := RememberStaleCtx[payload](ctx, c, "rs:nilctx", time.Second, 2*time.Second, nil); err == nil {
		t.Fatalf("expected RememberStaleCtx nil callback error")
	}
}

func TestObserverFuncAndErrorStoreDriver(t *testing.T) {
	t.Parallel()

	// Nil ObserverFunc should be a no-op.
	var nilObs ObserverFunc
	nilObs.OnCacheOp(context.Background(), "get", "k", false, nil, 0, cachecore.DriverMemory)

	called := false
	ObserverFunc(func(ctx context.Context, op, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver) {
		called = true
		if op != "set" || key != "k" || driver != cachecore.DriverMemory {
			t.Fatalf("unexpected observer payload")
		}
	}).OnCacheOp(context.Background(), "set", "k", true, nil, time.Millisecond, cachecore.DriverMemory)
	if !called {
		t.Fatalf("observer func was not called")
	}

	e := &errorStore{driver: cachecore.DriverRedis, err: errors.New("boom")}
	if got := e.Driver(); got != cachecore.DriverRedis {
		t.Fatalf("expected driver=%q got=%q", cachecore.DriverRedis, got)
	}
}

func TestEncryptingStoreFlushDelegates(t *testing.T) {
	t.Parallel()
	base := &spyStore{driver: cachecore.DriverMemory}
	s, err := newEncryptingStore(base, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("newEncryptingStore failed: %v", err)
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
}

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
