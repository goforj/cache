package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

type testPayload struct {
	Name string `json:"name"`
}

func TestCacheRememberCachesValue(t *testing.T) {
	repo := NewCache(newMemoryStore(0, 0))
	ctx := context.Background()

	calls := 0
	fn := func(context.Context) ([]byte, error) {
		calls++
		return []byte("alpha"), nil
	}

	first, err := repo.RememberBytesCtx(ctx, "k", time.Minute, fn)
	if err != nil {
		t.Fatalf("remember failed: %v", err)
	}
	second, err := repo.RememberBytesCtx(ctx, "k", time.Minute, fn)
	if err != nil {
		t.Fatalf("remember failed: %v", err)
	}

	if string(first) != "alpha" || string(second) != "alpha" {
		t.Fatalf("unexpected remember value")
	}
	if calls != 1 {
		t.Fatalf("expected callback once, got %d", calls)
	}
}

func TestRememberValueTyped(t *testing.T) {
	type profile struct{ Name string }
	c := NewCache(newMemoryStore(0, 0))

	val, err := Remember[profile](c, "p", time.Minute, func() (profile, error) {
		return profile{Name: "Ada"}, nil
	})
	if err != nil || val.Name != "Ada" {
		t.Fatalf("unexpected remember value: %+v err=%v", val, err)
	}

	// ensure cached
	val, err = Remember[profile](c, "p", time.Minute, func() (profile, error) {
		return profile{Name: "Other"}, nil
	})
	if err != nil || val.Name != "Ada" {
		t.Fatalf("expected cached value, got %+v err=%v", val, err)
	}
}

func TestCacheRememberJSON(t *testing.T) {
	repo := NewCache(newMemoryStore(0, 0))
	ctx := context.Background()

	calls := 0
	value, err := RememberCtx[testPayload](ctx, repo, "json", time.Minute, func(context.Context) (testPayload, error) {
		calls++
		return testPayload{Name: "cache"}, nil
	})
	if err != nil {
		t.Fatalf("remember json failed: %v", err)
	}
	if value.Name != "cache" {
		t.Fatalf("unexpected payload: %+v", value)
	}

	value, err = RememberCtx[testPayload](ctx, repo, "json", time.Minute, func(context.Context) (testPayload, error) {
		calls++
		return testPayload{Name: "again"}, nil
	})
	if err != nil {
		t.Fatalf("remember json failed: %v", err)
	}
	if value.Name != "cache" {
		t.Fatalf("unexpected cached payload: %+v", value)
	}
	if calls != 1 {
		t.Fatalf("expected callback once, got %d", calls)
	}
}

func TestCacheGetSetJSON(t *testing.T) {
	repo := NewCache(newMemoryStore(0, 0))
	ctx := context.Background()

	if err := SetJSONCtx(ctx, repo, "u", testPayload{Name: "alex"}, time.Minute); err != nil {
		t.Fatalf("set json failed: %v", err)
	}
	got, ok, err := GetJSONCtx[testPayload](ctx, repo, "u")
	if err != nil {
		t.Fatalf("get json failed: %v", err)
	}
	if !ok || got.Name != "alex" {
		t.Fatalf("unexpected json result: ok=%v value=%+v", ok, got)
	}
}

func TestCacheAddIncrementDecrementAndPull(t *testing.T) {
	repo := NewCache(newMemoryStore(0, 0))
	ctx := context.Background()

	created, err := repo.AddCtx(ctx, "add", []byte("x"), time.Minute)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if !created {
		t.Fatalf("expected first add to create key")
	}
	created, err = repo.AddCtx(ctx, "add", []byte("y"), time.Minute)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if created {
		t.Fatalf("expected second add to be ignored")
	}

	value, err := repo.IncrementCtx(ctx, "counter", 3, time.Minute)
	if err != nil {
		t.Fatalf("increment failed: %v", err)
	}
	if value != 3 {
		t.Fatalf("expected 3, got %d", value)
	}
	value, err = repo.DecrementCtx(ctx, "counter", 1, time.Minute)
	if err != nil {
		t.Fatalf("decrement failed: %v", err)
	}
	if value != 2 {
		t.Fatalf("expected 2, got %d", value)
	}

	if err := repo.SetStringCtx(ctx, "pull", "value", time.Minute); err != nil {
		t.Fatalf("set string failed: %v", err)
	}
	body, ok, err := repo.PullBytes("pull")
	if err != nil {
		t.Fatalf("pull failed: %v", err)
	}
	if !ok || string(body) != "value" {
		t.Fatalf("unexpected pull result")
	}
	_, ok, err = repo.GetBytesCtx(ctx, "pull")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected pull key to be deleted")
	}
}

func TestCacheDeleteManyFlushAndErrors(t *testing.T) {
	repo := NewCache(newMemoryStore(0, 0))
	ctx := context.Background()

	if err := repo.SetStringCtx(ctx, "a", "1", time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := repo.SetStringCtx(ctx, "b", "2", time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := repo.DeleteManyCtx(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	_, ok, err := repo.GetBytesCtx(ctx, "a")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected deleted key")
	}

	if err := repo.SetStringCtx(ctx, "c", "3", time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := repo.FlushCtx(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	_, ok, err = repo.GetBytesCtx(ctx, "c")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected flush to clear key")
	}

	if _, err := repo.RememberBytesCtx(ctx, "missing", time.Minute, nil); err == nil {
		t.Fatalf("expected remember nil callback error")
	}
	if _, err := RememberCtx[string](ctx, repo, "missing-string", time.Minute, nil); err == nil {
		t.Fatalf("expected remember string nil callback error")
	}
	_, err = RememberCtx[testPayload](ctx, repo, "missing-json", time.Minute, nil)
	if err == nil {
		t.Fatalf("expected remember json nil callback error")
	}

	expected := errors.New("boom")
	_, err = repo.RememberBytesCtx(ctx, "broken", time.Minute, func(context.Context) ([]byte, error) {
		return nil, expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
}

type spyStore struct {
	driver   cachecore.Driver
	readyErr error
	getBody  []byte
	getOK    bool
	getErr   error
	setErr   error
	addErr   error
	addOK    bool
	incVal   int64
	incErr   error
	delErr   error
	delMany  error
	flushErr error
	ttls     []time.Duration
	getCalls int
}

var expectedErr = errors.New("expected")

func (s *spyStore) Driver() cachecore.Driver { return s.driver }
func (s *spyStore) Ready(context.Context) error {
	return s.readyErr
}

func (s *spyStore) Get(context.Context, string) ([]byte, bool, error) {
	s.getCalls++
	return cloneBytes(s.getBody), s.getOK, s.getErr
}

func (s *spyStore) Set(_ context.Context, _ string, value []byte, ttl time.Duration) error {
	s.getBody = cloneBytes(value)
	s.getOK = true
	s.ttls = append(s.ttls, ttl)
	return s.setErr
}

func (s *spyStore) Add(_ context.Context, _ string, _ []byte, ttl time.Duration) (bool, error) {
	s.ttls = append(s.ttls, ttl)
	return s.addOK, s.addErr
}

func (s *spyStore) Increment(_ context.Context, _ string, delta int64, ttl time.Duration) (int64, error) {
	s.incVal += delta
	s.ttls = append(s.ttls, ttl)
	return s.incVal, s.incErr
}

func (s *spyStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

func (s *spyStore) Delete(context.Context, string) error { return s.delErr }

func (s *spyStore) DeleteMany(context.Context, ...string) error { return s.delMany }

func (s *spyStore) Flush(context.Context) error { return s.flushErr }

func TestCacheStoreAndDriver(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory}
	c := NewCache(store)
	if c.Store() != store {
		t.Fatalf("expected Store to return underlying store")
	}
	if c.Driver() != cachecore.DriverMemory {
		t.Fatalf("expected driver to propagate")
	}
}

func TestCacheReady(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory}
	c := NewCache(store)
	if err := c.Ready(); err != nil {
		t.Fatalf("expected ready nil, got %v", err)
	}

	expected := errors.New("not ready")
	store.readyErr = expected
	if err := c.ReadyCtx(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("expected ready error %v, got %v", expected, err)
	}
}

func TestCacheRememberStringUsesResolvedTTL(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory}
	c := NewCacheWithTTL(store, 2*time.Second)
	ctx := context.Background()

	calls := 0
	val, err := RememberCtx[string](ctx, c, "k", 0, func(context.Context) (string, error) {
		calls++
		return "hello", nil
	})
	if err != nil || val != "hello" {
		t.Fatalf("remember string failed: %v %q", err, val)
	}
	val, err = RememberCtx[string](ctx, c, "k", time.Second, func(context.Context) (string, error) {
		calls++
		return "new", nil
	})
	if err != nil || val != "hello" {
		t.Fatalf("expected cached value, got %q err=%v", val, err)
	}
	if calls != 1 {
		t.Fatalf("expected callback once, got %d", calls)
	}
	if len(store.ttls) < 1 || store.ttls[0] != 2*time.Second {
		t.Fatalf("expected default ttl recorded, got %v", store.ttls)
	}
}

func TestCacheSetUsesProvidedTTL(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory}
	c := NewCacheWithTTL(store, time.Minute)
	ctx := context.Background()

	if err := c.SetBytesCtx(ctx, "k", []byte("v"), 3*time.Second); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if len(store.ttls) != 1 || store.ttls[0] != 3*time.Second {
		t.Fatalf("expected ttl=3s, got %v", store.ttls)
	}
}

func TestNewCacheWithTTLDefaultsWhenNonPositive(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory}
	c := NewCacheWithTTL(store, -1)
	ctx := context.Background()

	_, _ = RememberCtx[string](ctx, c, "k", 0, func(context.Context) (string, error) {
		return "v", nil
	})
	if len(store.ttls) != 1 || store.ttls[0] != defaultCacheTTL {
		t.Fatalf("expected default cache ttl, got %v", store.ttls)
	}
}

func TestCacheWriteOpsResolveDefaultTTLWhenNonPositive(t *testing.T) {
	defaultTTL := 2 * time.Second
	tests := []struct {
		name string
		run  func(*Cache, time.Duration) error
	}{
		{
			name: "set",
			run: func(c *Cache, ttl time.Duration) error {
				return c.SetBytes("ttl:set", []byte("v"), ttl)
			},
		},
		{
			name: "add",
			run: func(c *Cache, ttl time.Duration) error {
				_, err := c.Add("ttl:add", []byte("v"), ttl)
				return err
			},
		},
		{
			name: "increment",
			run: func(c *Cache, ttl time.Duration) error {
				_, err := c.Increment("ttl:inc", 1, ttl)
				return err
			},
		},
		{
			name: "decrement",
			run: func(c *Cache, ttl time.Duration) error {
				_, err := c.Decrement("ttl:dec", 1, ttl)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ttl := range []time.Duration{0, -1 * time.Second} {
				store := &spyStore{driver: cachecore.DriverMemory, addOK: true}
				c := NewCacheWithTTL(store, defaultTTL)
				if err := tt.run(c, ttl); err != nil {
					t.Fatalf("op failed for ttl=%v: %v", ttl, err)
				}
				if len(store.ttls) != 1 || store.ttls[0] != defaultTTL {
					t.Fatalf("expected default ttl %v for ttl=%v, got %v", defaultTTL, ttl, store.ttls)
				}
			}
		})
	}
}

func TestCacheGetStringError(t *testing.T) {
	expected := errors.New("boom")
	store := &spyStore{driver: cachecore.DriverMemory, getErr: expected}
	c := NewCache(store)
	ctx := context.Background()
	_, _, err := c.GetStringCtx(ctx, "k")
	if !errors.Is(err, expected) {
		t.Fatalf("expected propagated error")
	}
}

func TestCacheGetStringSuccess(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory, getOK: true, getBody: []byte("ok")}
	c := NewCache(store)
	ctx := context.Background()
	val, ok, err := c.GetStringCtx(ctx, "k")
	if err != nil || !ok || val != "ok" {
		t.Fatalf("unexpected result: val=%q ok=%v err=%v", val, ok, err)
	}
}

func TestCacheSetJSONMarshalError(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory}
	c := NewCache(store)
	ctx := context.Background()
	ch := make(chan int)
	if err := SetJSONCtx(ctx, c, "bad", ch, time.Second); err == nil {
		t.Fatalf("expected marshal error")
	}
}

func TestCachePullDeleteError(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory, getOK: true, getBody: []byte("x"), delErr: expectedErr}
	c := NewCache(store)
	_, _, err := c.PullBytes("k")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected delete error, got %v", err)
	}
}

func TestCacheRememberSetError(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory, setErr: expectedErr}
	c := NewCache(store)
	ctx := context.Background()
	_, err := c.RememberBytesCtx(ctx, "k", time.Second, func(context.Context) ([]byte, error) {
		return []byte("x"), nil
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected set error")
	}
}

func TestCacheRememberGetError(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory, getErr: expectedErr}
	c := NewCache(store)
	ctx := context.Background()
	if _, err := c.RememberBytesCtx(ctx, "k", time.Second, func(context.Context) ([]byte, error) { return []byte("x"), nil }); !errors.Is(err, expectedErr) {
		t.Fatalf("expected get error")
	}
}

func TestCacheRememberJSONCallbackError(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory}
	c := NewCache(store)
	ctx := context.Background()
	expected := errors.New("cb")
	_, err := RememberCtx[int](ctx, c, "k", time.Second, func(context.Context) (int, error) {
		return 0, expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error")
	}
}

func TestCacheRememberJSONSetError(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory, setErr: expectedErr}
	c := NewCache(store)
	ctx := context.Background()
	_, err := RememberCtx[int](ctx, c, "k", time.Second, func(context.Context) (int, error) { return 5, nil })
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected set error")
	}
}

func TestCacheRememberStringCallbackError(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory}
	c := NewCache(store)
	ctx := context.Background()
	expected := errors.New("cb")
	if _, err := RememberCtx[string](ctx, c, "k", time.Second, func(context.Context) (string, error) {
		return "", expected
	}); !errors.Is(err, expected) {
		t.Fatalf("expected callback error")
	}
}

func TestCacheRememberStringUsesCachedValueWithoutCallback(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory, getOK: true, getBody: []byte(`"cached"`)}
	c := NewCache(store)
	ctx := context.Background()
	calls := 0
	val, err := RememberCtx[string](ctx, c, "k", time.Second, func(context.Context) (string, error) {
		calls++
		return "fresh", nil
	})
	if err != nil || val != "cached" || calls != 0 || store.getCalls == 0 {
		t.Fatalf("expected cached value without callback, val=%q calls=%d gets=%d err=%v", val, calls, store.getCalls, err)
	}
}

func TestCacheRememberJSONReturnsCachedValue(t *testing.T) {
	payload := struct {
		V string `json:"v"`
	}{V: "cached"}
	body, _ := json.Marshal(payload)
	store := &spyStore{driver: cachecore.DriverMemory, getOK: true, getBody: body}
	c := NewCache(store)
	ctx := context.Background()
	calls := 0
	result, err := RememberCtx[struct {
		V string `json:"v"`
	}](ctx, c, "k", time.Second, func(context.Context) (struct {
		V string `json:"v"`
	}, error) {
		calls++
		return payload, nil
	})
	if err != nil || result.V != "cached" || calls != 0 {
		t.Fatalf("expected cached json value, result=%+v calls=%d err=%v", result, calls, err)
	}
}

func TestCacheRememberJSONNilCallback(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory}
	c := NewCache(store)
	ctx := context.Background()
	if _, err := RememberCtx[int](ctx, c, "k", time.Second, nil); err == nil {
		t.Fatalf("expected nil callback error")
	}
}

func TestCacheRememberJSONGetError(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory, getOK: true, getBody: []byte("not-json")}
	c := NewCache(store)
	ctx := context.Background()
	if _, err := RememberCtx[int](ctx, c, "k", time.Second, func(context.Context) (int, error) { return 1, nil }); err == nil {
		t.Fatalf("expected get decode error")
	}
}

func TestCacheGetStringMissing(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory, getOK: false}
	c := NewCache(store)
	ctx := context.Background()
	_, ok, err := c.GetStringCtx(ctx, "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing string")
	}
}

func TestCacheGetJSONDecodeError(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory, getOK: true, getBody: []byte("not-json")}
	c := NewCache(store)
	ctx := context.Background()
	_, ok, err := GetJSONCtx[struct{}](ctx, c, "bad")
	if err == nil || ok {
		t.Fatalf("expected decode error")
	}
}

func TestCachePullMissing(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory, getOK: false}
	c := NewCache(store)
	_, ok, err := c.PullBytes("none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false when missing")
	}
}

func TestCacheRememberPropagatesCallbackError(t *testing.T) {
	store := &spyStore{driver: cachecore.DriverMemory}
	c := NewCache(store)
	ctx := context.Background()
	expected := errors.New("boom")
	_, err := c.RememberBytesCtx(ctx, "k", time.Second, func(context.Context) ([]byte, error) {
		return nil, expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
}

func TestCacheConvenienceWrappers(t *testing.T) {
	store := NewMemoryStore(context.Background())
	c := NewCache(store)

	if err := c.SetBytes("k", []byte("v"), time.Second); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if body, ok, err := c.GetBytes("k"); err != nil || !ok || string(body) != "v" {
		t.Fatalf("get failed: ok=%v err=%v body=%q", ok, err, string(body))
	}

	if err := SetJSON(c, "json", testPayload{Name: "nina"}, time.Second); err != nil {
		t.Fatalf("set json failed: %v", err)
	}
	gotJSON, ok, err := GetJSON[testPayload](c, "json")
	if err != nil || !ok || gotJSON.Name != "nina" {
		t.Fatalf("get json failed: ok=%v err=%v value=%+v", ok, err, gotJSON)
	}

	created, err := c.Add("once", []byte("1"), time.Second)
	if err != nil || !created {
		t.Fatalf("add first failed: created=%v err=%v", created, err)
	}
	created, err = c.Add("once", []byte("2"), time.Second)
	if err != nil || created {
		t.Fatalf("add second failed: created=%v err=%v", created, err)
	}

	val, err := c.Increment("counter", 3, time.Second)
	if err != nil || val != 3 {
		t.Fatalf("increment failed: val=%d err=%v", val, err)
	}
	val, err = c.Decrement("counter", 1, time.Second)
	if err != nil || val != 2 {
		t.Fatalf("decrement failed: val=%d err=%v", val, err)
	}

	if err := c.SetString("a", "1", time.Second); err != nil {
		t.Fatalf("set string a failed: %v", err)
	}
	if err := c.SetString("b", "2", time.Second); err != nil {
		t.Fatalf("set string b failed: %v", err)
	}
	if err := c.DeleteMany("a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if _, ok, err := c.GetBytes("a"); err != nil || ok {
		t.Fatalf("expected a deleted: ok=%v err=%v", ok, err)
	}

	if err := c.SetString("flush", "x", time.Second); err != nil {
		t.Fatalf("set flush failed: %v", err)
	}
	if err := c.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if _, ok, err := c.GetBytes("flush"); err != nil || ok {
		t.Fatalf("expected flush key removed: ok=%v err=%v", ok, err)
	}

	callsBytes := 0
	body, err := c.RememberBytes("rb", time.Second, func() ([]byte, error) {
		callsBytes++
		return []byte("remembered"), nil
	})
	if err != nil || string(body) != "remembered" {
		t.Fatalf("remember bytes first failed: body=%q err=%v", string(body), err)
	}
	_, err = c.RememberBytes("rb", time.Second, func() ([]byte, error) {
		callsBytes++
		return []byte("other"), nil
	})
	if err != nil || callsBytes != 1 {
		t.Fatalf("remember bytes cache failed: calls=%d err=%v", callsBytes, err)
	}

	callsString := 0
	s, err := Remember[string](c, "rs", time.Second, func() (string, error) {
		callsString++
		return "hello", nil
	})
	if err != nil || s != "hello" {
		t.Fatalf("remember string first failed: val=%q err=%v", s, err)
	}
	_, err = Remember[string](c, "rs", time.Second, func() (string, error) {
		callsString++
		return "other", nil
	})
	if err != nil || callsString != 1 {
		t.Fatalf("remember string cache failed: calls=%d err=%v", callsString, err)
	}

	callsJSON := 0
	type profile struct {
		Name string `json:"name"`
	}
	p, err := Remember[profile](c, "rj", time.Second, func() (profile, error) {
		callsJSON++
		return profile{Name: "ada"}, nil
	})
	if err != nil || p.Name != "ada" {
		t.Fatalf("remember json first failed: val=%+v err=%v", p, err)
	}
	_, err = Remember[profile](c, "rj", time.Second, func() (profile, error) {
		callsJSON++
		return profile{Name: "other"}, nil
	})
	if err != nil || callsJSON != 1 {
		t.Fatalf("remember json cache failed: calls=%d err=%v", callsJSON, err)
	}
}

func TestRememberJSONWrapperErrors(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	if _, err := Remember[testPayload](c, "k", time.Second, nil); err == nil {
		t.Fatalf("expected nil callback error")
	}

	expected := errors.New("wrapper boom")
	_, err := Remember[testPayload](c, "k2", time.Second, func() (testPayload, error) {
		return testPayload{}, expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
}

func TestRememberConvenienceErrorPaths(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))

	if _, err := c.RememberBytes("rb-nil", time.Second, nil); err == nil {
		t.Fatalf("expected nil callback error for remember bytes")
	}
	if _, err := Remember[string](c, "rs-nil", time.Second, nil); err == nil {
		t.Fatalf("expected nil callback error for remember string")
	}

	expected := errors.New("callback failed")
	if _, err := c.RememberBytes("rb-err", time.Second, func() ([]byte, error) {
		return nil, expected
	}); !errors.Is(err, expected) {
		t.Fatalf("expected remember bytes callback error, got %v", err)
	}
	if _, err := Remember[string](c, "rs-err", time.Second, func() (string, error) {
		return "", expected
	}); !errors.Is(err, expected) {
		t.Fatalf("expected remember string callback error, got %v", err)
	}
}

func TestRememberValueWithCodecBranches(t *testing.T) {
	ctx := context.Background()

	decodeErrCodec := ValueCodec[int]{
		Encode: func(v int) ([]byte, error) { return []byte("1"), nil },
		Decode: func(_ []byte) (int, error) { return 0, errors.New("decode boom") },
	}
	store := &spyStore{driver: cachecore.DriverMemory, getOK: true, getBody: []byte("cached")}
	cache := NewCache(store)
	if _, err := rememberValueWithCodecCtx[int](ctx, cache, "k", time.Second, func() (int, error) { return 1, nil }, decodeErrCodec); err == nil {
		t.Fatalf("expected decode error")
	}

	getErrStore := &spyStore{driver: cachecore.DriverMemory, getErr: expectedErr}
	if _, err := rememberValueWithCodecCtx[int](ctx, NewCache(getErrStore), "k", time.Second, func() (int, error) { return 1, nil }, defaultValueCodec[int]()); !errors.Is(err, expectedErr) {
		t.Fatalf("expected get error, got %v", err)
	}

	missStore := &spyStore{driver: cachecore.DriverMemory, getOK: false}
	if _, err := rememberValueWithCodecCtx[int](ctx, NewCache(missStore), "k", time.Second, nil, defaultValueCodec[int]()); err == nil {
		t.Fatalf("expected nil callback error")
	}

	fnErrStore := &spyStore{driver: cachecore.DriverMemory, getOK: false}
	fnErr := errors.New("fn boom")
	if _, err := rememberValueWithCodecCtx[int](ctx, NewCache(fnErrStore), "k", time.Second, func() (int, error) { return 0, fnErr }, defaultValueCodec[int]()); !errors.Is(err, fnErr) {
		t.Fatalf("expected fn error, got %v", err)
	}

	encodeErr := errors.New("encode boom")
	encodeErrCodec := ValueCodec[int]{
		Encode: func(v int) ([]byte, error) { return nil, encodeErr },
		Decode: func(b []byte) (int, error) { return 0, nil },
	}
	encodeErrStore := &spyStore{driver: cachecore.DriverMemory, getOK: false}
	if _, err := rememberValueWithCodecCtx[int](ctx, NewCache(encodeErrStore), "k", time.Second, func() (int, error) { return 5, nil }, encodeErrCodec); !errors.Is(err, encodeErr) {
		t.Fatalf("expected encode error, got %v", err)
	}
}

func TestRememberValueDecodesLegacyJSONPayload(t *testing.T) {
	type profile struct {
		Name string `json:"name"`
	}

	c := NewCache(NewMemoryStore(context.Background()))
	legacyPayload := []byte(`{"name":"Ada","deprecated_field":"x"}`)
	if err := c.SetBytes("legacy:profile", legacyPayload, time.Minute); err != nil {
		t.Fatalf("seed legacy payload failed: %v", err)
	}

	calls := 0
	out, err := Remember[profile](c, "legacy:profile", time.Minute, func() (profile, error) {
		calls++
		return profile{Name: "new"}, nil
	})
	if err != nil {
		t.Fatalf("remember value failed: %v", err)
	}
	if out.Name != "Ada" {
		t.Fatalf("expected legacy cached decode, got %+v", out)
	}
	if calls != 0 {
		t.Fatalf("expected callback not to run on cached legacy payload, got %d", calls)
	}
}

func TestCacheRateLimitAllowsThenDenies(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	key := "rl:test:user:1"

	for i := 1; i <= 3; i++ {
		res, err := c.RateLimit(key, 3, time.Minute)
		if err != nil {
			t.Fatalf("rate limit call %d failed: %v", i, err)
		}
		if !res.Allowed || res.Count != int64(i) {
			t.Fatalf("expected allowed count=%d, got allowed=%v count=%d", i, res.Allowed, res.Count)
		}
	}

	res, err := c.RateLimit(key, 3, time.Minute)
	if err != nil {
		t.Fatalf("rate limit deny call failed: %v", err)
	}
	if res.Allowed || res.Count != 4 {
		t.Fatalf("expected denied at count=4, got allowed=%v count=%d", res.Allowed, res.Count)
	}
}

func TestCacheRateLimitValidatesInput(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))

	if _, err := c.RateLimit("rl:k", 0, time.Second); err == nil {
		t.Fatalf("expected error for non-positive limit")
	}
	if _, err := c.RateLimit("rl:k", 1, 0); err == nil {
		t.Fatalf("expected error for non-positive window")
	}
}

func TestCacheRateLimitPropagatesIncrementError(t *testing.T) {
	c := NewCache(&spyStore{driver: cachecore.DriverMemory, incErr: expectedErr})
	if _, err := c.RateLimit("rl:k", 1, time.Second); !errors.Is(err, expectedErr) {
		t.Fatalf("expected increment error, got %v", err)
	}
}

func TestCacheRateLimitWindowResets(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	key := "rl:test:reset"
	window := 120 * time.Millisecond

	res, err := c.RateLimit(key, 1, window)
	if err != nil || !res.Allowed || res.Count != 1 {
		t.Fatalf("expected first call allowed, got res=%+v err=%v", res, err)
	}
	res, err = c.RateLimit(key, 1, window)
	if err != nil || res.Allowed || res.Count != 2 {
		t.Fatalf("expected second call denied, got res=%+v err=%v", res, err)
	}

	time.Sleep(250 * time.Millisecond)

	res, err = c.RateLimit(key, 1, window)
	if err != nil || !res.Allowed || res.Count != 1 {
		t.Fatalf("expected window reset, got res=%+v err=%v", res, err)
	}
}

func TestCacheRateLimitMetadata(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	key := "rl:remaining:user:1"

	res, err := c.RateLimit(key, 3, time.Minute)
	if err != nil || !res.Allowed || res.Count != 1 || res.Remaining != 2 || !res.ResetAt.After(time.Now()) {
		t.Fatalf("first call mismatch res=%+v err=%v", res, err)
	}

	res, err = c.RateLimit(key, 3, time.Minute)
	if err != nil || !res.Allowed || res.Count != 2 || res.Remaining != 1 {
		t.Fatalf("second call mismatch res=%+v err=%v", res, err)
	}

	res, err = c.RateLimit(key, 3, time.Minute)
	if err != nil || !res.Allowed || res.Count != 3 || res.Remaining != 0 {
		t.Fatalf("third call mismatch res=%+v err=%v", res, err)
	}

	res, err = c.RateLimit(key, 3, time.Minute)
	if err != nil || res.Allowed || res.Count != 4 || res.Remaining != 0 {
		t.Fatalf("fourth call mismatch res=%+v err=%v", res, err)
	}
}

func TestCacheRateLimitValidationAndError(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	if _, err := c.RateLimit("rl:bad", 0, time.Second); err == nil {
		t.Fatalf("expected limit validation error")
	}
	if _, err := c.RateLimit("rl:bad", 1, 0); err == nil {
		t.Fatalf("expected window validation error")
	}

	cErr := NewCache(&spyStore{driver: cachecore.DriverMemory, incErr: expectedErr})
	if _, err := cErr.RateLimit("rl:err", 1, time.Second); !errors.Is(err, expectedErr) {
		t.Fatalf("expected increment error, got %v", err)
	}
}

func TestCacheBatchSetAndBatchGet(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	err := c.BatchSetBytes(map[string][]byte{
		"a": []byte("1"),
		"b": []byte("2"),
	}, time.Minute)
	if err != nil {
		t.Fatalf("batch set failed: %v", err)
	}

	values, err := c.BatchGetBytes("a", "b", "missing")
	if err != nil {
		t.Fatalf("batch get failed: %v", err)
	}
	if got := string(values["a"]); got != "1" {
		t.Fatalf("unexpected a value: %q", got)
	}
	if got := string(values["b"]); got != "2" {
		t.Fatalf("unexpected b value: %q", got)
	}
	if _, ok := values["missing"]; ok {
		t.Fatalf("missing key should be omitted")
	}
}

func TestCacheBatchGetPropagatesError(t *testing.T) {
	c := NewCache(&spyStore{driver: cachecore.DriverMemory, getErr: expectedErr})
	if _, err := c.BatchGetBytes("a", "b"); !errors.Is(err, expectedErr) {
		t.Fatalf("expected get error, got %v", err)
	}
}

func TestCacheBatchSetPropagatesError(t *testing.T) {
	c := NewCache(&spyStore{driver: cachecore.DriverMemory, setErr: expectedErr})
	if err := c.BatchSetBytes(map[string][]byte{"a": []byte("1")}, time.Second); !errors.Is(err, expectedErr) {
		t.Fatalf("expected set error, got %v", err)
	}
}

func TestCacheRefreshAheadMissComputesSynchronously(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	calls := 0
	body, err := c.RefreshAheadBytes("ra:miss", time.Second, 200*time.Millisecond, func() ([]byte, error) {
		calls++
		return []byte("fresh"), nil
	})
	if err != nil || string(body) != "fresh" || calls != 1 {
		t.Fatalf("unexpected miss result: body=%q calls=%d err=%v", string(body), calls, err)
	}
}

func TestCacheRefreshAheadHitTriggersAsyncRefresh(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	key := "ra:hit"

	calls := 0
	_, err := c.RefreshAheadBytes(key, 300*time.Millisecond, 250*time.Millisecond, func() ([]byte, error) {
		calls++
		return []byte("v1"), nil
	})
	if err != nil {
		t.Fatalf("seed refresh ahead failed: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	done := make(chan struct{}, 1)
	body, err := c.RefreshAheadBytes(key, 300*time.Millisecond, 250*time.Millisecond, func() ([]byte, error) {
		calls++
		done <- struct{}{}
		return []byte("v2"), nil
	})
	if err != nil || string(body) != "v1" {
		t.Fatalf("expected immediate cached value, body=%q err=%v", string(body), err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected async refresh callback to run")
	}

	body2, ok, err := c.GetBytes(key)
	if err != nil || !ok || string(body2) != "v2" {
		t.Fatalf("expected refreshed value, ok=%v body=%q err=%v", ok, string(body2), err)
	}
	if calls < 2 {
		t.Fatalf("expected refresh callback to run twice total, calls=%d", calls)
	}
}

func TestCacheRefreshAheadConcurrentHitTriggersSingleAsyncRefresh(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	key := "ra:contend"
	ttl := 2 * time.Second
	refreshAhead := 200 * time.Millisecond

	var calls atomic.Int64
	refreshStarted := make(chan struct{}, 1)
	releaseRefresh := make(chan struct{})
	cb := func() ([]byte, error) {
		n := calls.Add(1)
		if n == 1 {
			return []byte("v1"), nil
		}
		select {
		case refreshStarted <- struct{}{}:
		default:
		}
		<-releaseRefresh
		return []byte("v2"), nil
	}

	_, err := c.RefreshAheadBytes(key, ttl, refreshAhead, cb)
	if err != nil {
		t.Fatalf("seed refresh ahead failed: %v", err)
	}
	time.Sleep(1850 * time.Millisecond)

	const workers = 24
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	values := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			body, err := c.RefreshAheadBytes(key, ttl, refreshAhead, cb)
			if err != nil {
				errs <- err
				return
			}
			values <- string(body)
		}()
	}
	close(start)
	wg.Wait()

	select {
	case <-refreshStarted:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected async refresh callback to start under contention")
	}
	// While the async refresh callback is blocked, additional callbacks should
	// not start because refresh lock is held.
	time.Sleep(80 * time.Millisecond)
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected exactly one in-flight async refresh while lock is held, got %d callbacks", got)
	}
	close(releaseRefresh)

	for len(errs) > 0 {
		t.Fatalf("unexpected refresh ahead contention error: %v", <-errs)
	}
	for i := 0; i < workers; i++ {
		if got := <-values; got != "v1" {
			t.Fatalf("expected immediate cached value during contention, got %q", got)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		body, ok, err := c.GetBytes(key)
		if err == nil && ok && string(body) == "v2" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected refreshed value v2, ok=%v body=%q err=%v", ok, string(body), err)
		}
		time.Sleep(10 * time.Millisecond)
	}

}

func TestCacheRefreshAheadValidationAndErrors(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	if _, err := c.RefreshAheadBytes("ra:bad", 0, time.Second, func() ([]byte, error) { return []byte("x"), nil }); err == nil {
		t.Fatalf("expected ttl validation error")
	}
	if _, err := c.RefreshAheadBytes("ra:bad", time.Second, 0, func() ([]byte, error) { return []byte("x"), nil }); err == nil {
		t.Fatalf("expected refreshAhead validation error")
	}
	if _, err := c.RefreshAheadBytes("ra:bad", time.Second, time.Second, nil); err == nil {
		t.Fatalf("expected nil callback error")
	}

	expected := errors.New("upstream")
	if _, err := c.RefreshAheadBytes("ra:err", time.Second, time.Second, func() ([]byte, error) {
		return nil, expected
	}); !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
}

func TestCacheRefreshAheadHitSkipsAsyncRefreshWithoutValidMetadata(t *testing.T) {
	tests := []struct {
		name     string
		seedMeta func(t *testing.T, c *Cache, key string)
	}{
		{
			name: "missing metadata key",
			seedMeta: func(t *testing.T, c *Cache, key string) {
				t.Helper()
			},
		},
		{
			name: "malformed metadata key",
			seedMeta: func(t *testing.T, c *Cache, key string) {
				t.Helper()
				if err := c.SetBytes(key+refreshMetaSuffix, []byte("not-an-int"), time.Second); err != nil {
					t.Fatalf("seed malformed metadata failed: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCache(NewMemoryStore(context.Background()))
			key := "ra:no-meta:" + tt.name
			if err := c.SetBytes(key, []byte("cached"), time.Second); err != nil {
				t.Fatalf("seed value failed: %v", err)
			}
			tt.seedMeta(t, c, key)

			var calls atomic.Int64
			body, err := c.RefreshAheadBytes(key, time.Second, 500*time.Millisecond, func() ([]byte, error) {
				calls.Add(1)
				return []byte("refreshed"), nil
			})
			if err != nil {
				t.Fatalf("refresh ahead failed: %v", err)
			}
			if got := string(body); got != "cached" {
				t.Fatalf("expected cached value on hit, got %q", got)
			}

			time.Sleep(100 * time.Millisecond)
			if got := calls.Load(); got != 0 {
				t.Fatalf("expected no async refresh callback without valid metadata, got %d", got)
			}
		})
	}
}

func TestRefreshAheadTyped(t *testing.T) {
	type summary struct {
		Text string `json:"text"`
	}
	c := NewCache(NewMemoryStore(context.Background()))

	val, err := RefreshAhead[summary](c, "ra:typed", time.Second, 300*time.Millisecond, func() (summary, error) {
		return summary{Text: "hello"}, nil
	})
	if err != nil || val.Text != "hello" {
		t.Fatalf("typed refresh ahead failed: val=%+v err=%v", val, err)
	}
}

func TestRefreshAheadValueWithCodecErrors(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	ctx := context.Background()

	if _, err := RefreshAheadValueWithCodec[int](ctx, c, "ra:codec:nil", time.Second, 200*time.Millisecond, nil, defaultValueCodec[int]()); err == nil {
		t.Fatalf("expected nil callback error")
	}

	encodeErr := errors.New("encode boom")
	codec := ValueCodec[int]{
		Encode: func(v int) ([]byte, error) { return nil, encodeErr },
		Decode: func(b []byte) (int, error) { return 0, nil },
	}
	if _, err := RefreshAheadValueWithCodec[int](ctx, c, "ra:codec:encode", time.Second, 200*time.Millisecond, func() (int, error) { return 1, nil }, codec); !errors.Is(err, encodeErr) {
		t.Fatalf("expected encode error, got %v", err)
	}
}

func TestCacheTryLockAndUnlock(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	key := "lock:job:1"

	locked, err := c.TryLock(key, time.Second)
	if err != nil || !locked {
		t.Fatalf("expected first try lock success, locked=%v err=%v", locked, err)
	}
	locked, err = c.TryLock(key, time.Second)
	if err != nil || locked {
		t.Fatalf("expected second try lock miss, locked=%v err=%v", locked, err)
	}
	if err := c.Unlock(key); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}
	locked, err = c.TryLock(key, time.Second)
	if err != nil || !locked {
		t.Fatalf("expected try lock success after unlock, locked=%v err=%v", locked, err)
	}
}

func TestCacheTryLockValidationAndError(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	if _, err := c.TryLock("lock:bad", 0); err == nil {
		t.Fatalf("expected ttl validation error")
	}

	cErr := NewCache(&spyStore{driver: cachecore.DriverMemory, addErr: expectedErr})
	if _, err := cErr.TryLock("lock:err", time.Second); !errors.Is(err, expectedErr) {
		t.Fatalf("expected add error, got %v", err)
	}
}

func TestCacheLockWaitsAndTimesOut(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	key := "lock:wait"

	locked, err := c.TryLock(key, 500*time.Millisecond)
	if err != nil || !locked {
		t.Fatalf("seed lock failed: locked=%v err=%v", locked, err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = c.Unlock(key)
	}()

	locked, err = c.Lock(key, time.Second, time.Second)
	if err != nil || !locked {
		t.Fatalf("expected lock to eventually acquire, locked=%v err=%v", locked, err)
	}

	timeoutKey := "lock:timeout"
	locked, err = c.TryLock(timeoutKey, time.Second)
	if err != nil || !locked {
		t.Fatalf("seed timeout lock failed: locked=%v err=%v", locked, err)
	}
	locked, err = c.Lock(timeoutKey, time.Second, 80*time.Millisecond)
	if err == nil || locked {
		t.Fatalf("expected timeout lock failure, locked=%v err=%v", locked, err)
	}
}

func TestCacheTryLockConcurrentContentionSingleWinner(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	key := "lock:contend:single-winner"

	const workers = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	var winners atomic.Int64
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			locked, err := c.TryLock(key, time.Second)
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

	for len(errs) > 0 {
		t.Fatalf("unexpected try lock error: %v", <-errs)
	}
	if got := winners.Load(); got != 1 {
		t.Fatalf("expected exactly one lock winner under contention, got %d", got)
	}
}

func TestCacheLockConcurrentTimeoutContention(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	key := "lock:contend:timeout"

	locked, err := c.TryLock(key, time.Second)
	if err != nil || !locked {
		t.Fatalf("seed lock failed: locked=%v err=%v", locked, err)
	}

	const workers = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			locked, err := c.Lock(key, time.Second, 60*time.Millisecond)
			if err == nil || locked {
				errs <- errors.New("expected lock timeout under contention")
				return
			}
		}()
	}
	close(start)
	wg.Wait()

	for len(errs) > 0 {
		t.Fatalf("unexpected lock contention result: %v", <-errs)
	}
}

func TestCacheRememberCtxConcurrentMissContention(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	ctx := context.Background()

	var calls atomic.Int64
	const workers = 24
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	values := make(chan string, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			body, err := c.RememberBytesCtx(ctx, "remember:contend", time.Minute, func(context.Context) ([]byte, error) {
				calls.Add(1)
				time.Sleep(20 * time.Millisecond)
				return []byte("value"), nil
			})
			if err != nil {
				errs <- err
				return
			}
			values <- string(body)
		}()
	}

	close(start)
	wg.Wait()

	for len(errs) > 0 {
		t.Fatalf("unexpected remember contention error: %v", <-errs)
	}
	for i := 0; i < workers; i++ {
		if got := <-values; got != "value" {
			t.Fatalf("unexpected remember value: %q", got)
		}
	}
	if calls.Load() < 1 {
		t.Fatalf("expected callback to run at least once")
	}

	body, ok, err := c.GetBytes("remember:contend")
	if err != nil || !ok || string(body) != "value" {
		t.Fatalf("expected cached value after contention, ok=%v body=%q err=%v", ok, string(body), err)
	}
}

func TestCacheRememberStaleFreshHit(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	if err := c.SetBytes("stale:fresh", []byte("cached"), time.Minute); err != nil {
		t.Fatalf("seed set failed: %v", err)
	}

	calls := 0
	val, stale, err := c.RememberStaleBytes("stale:fresh", time.Minute, 2*time.Minute, func() ([]byte, error) {
		calls++
		return []byte("new"), nil
	})
	if err != nil || stale || string(val) != "cached" || calls != 0 {
		t.Fatalf("expected fresh hit, stale=%v val=%q calls=%d err=%v", stale, string(val), calls, err)
	}
}

func TestCacheRememberStaleFallsBackOnCallbackError(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	key := "stale:fallback"

	val, stale, err := c.RememberStaleBytes(key, time.Minute, 5*time.Minute, func() ([]byte, error) {
		return []byte("seed"), nil
	})
	if err != nil || stale || string(val) != "seed" {
		t.Fatalf("seed remember stale failed: stale=%v val=%q err=%v", stale, string(val), err)
	}
	if err := c.Delete(key); err != nil {
		t.Fatalf("delete fresh key failed: %v", err)
	}

	expected := errors.New("upstream down")
	val, stale, err = c.RememberStaleBytes(key, time.Minute, 5*time.Minute, func() ([]byte, error) {
		return nil, expected
	})
	if err != nil || !stale || string(val) != "seed" {
		t.Fatalf("expected stale fallback, stale=%v val=%q err=%v", stale, string(val), err)
	}
}

func TestCacheRememberStaleNoFallbackReturnsError(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	expected := errors.New("compute failed")
	_, stale, err := c.RememberStaleBytes("stale:none", time.Minute, time.Minute, func() ([]byte, error) {
		return nil, expected
	})
	if stale || !errors.Is(err, expected) {
		t.Fatalf("expected compute error without stale, stale=%v err=%v", stale, err)
	}
}

func TestCacheRememberStaleNilCallback(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	if _, _, err := c.RememberStaleBytes("stale:nil", time.Minute, time.Minute, nil); err == nil {
		t.Fatalf("expected nil callback error")
	}
}

func TestCacheRememberStaleNonPositiveStaleTTLBehavior(t *testing.T) {
	t.Run("stale ttl falls back to primary ttl when primary ttl is positive", func(t *testing.T) {
		c := NewCache(NewMemoryStore(context.Background()))
		key := "stale:ttl-fallback"

		val, stale, err := c.RememberStaleBytes(key, time.Minute, 0, func() ([]byte, error) {
			return []byte("seed"), nil
		})
		if err != nil || stale || string(val) != "seed" {
			t.Fatalf("seed remember stale failed: stale=%v val=%q err=%v", stale, string(val), err)
		}
		if err := c.Delete(key); err != nil {
			t.Fatalf("delete fresh key failed: %v", err)
		}

		val, stale, err = c.RememberStaleBytes(key, time.Minute, 0, func() ([]byte, error) {
			return nil, errors.New("upstream down")
		})
		if err != nil || !stale || string(val) != "seed" {
			t.Fatalf("expected stale fallback via ttl-derived stale ttl, stale=%v val=%q err=%v", stale, string(val), err)
		}
	})

	t.Run("no stale key written when both ttl and stale ttl are non-positive", func(t *testing.T) {
		c := NewCacheWithTTL(NewMemoryStore(context.Background()), 200*time.Millisecond)
		key := "stale:no-stale-write"

		val, stale, err := c.RememberStaleBytes(key, 0, 0, func() ([]byte, error) {
			return []byte("seed"), nil
		})
		if err != nil || stale || string(val) != "seed" {
			t.Fatalf("seed remember stale failed: stale=%v val=%q err=%v", stale, string(val), err)
		}
		if _, ok, err := c.GetBytes(key); err != nil || !ok {
			t.Fatalf("expected fresh key to be written using default ttl, ok=%v err=%v", ok, err)
		}
		if _, ok, err := c.GetBytes(key + staleSuffix); err != nil {
			t.Fatalf("unexpected stale key read error: %v", err)
		} else if ok {
			t.Fatalf("expected no stale key when both ttl inputs are non-positive")
		}
		if err := c.Delete(key); err != nil {
			t.Fatalf("delete fresh key failed: %v", err)
		}

		expected := errors.New("upstream down")
		_, usedStale, err := c.RememberStaleBytes(key, 0, 0, func() ([]byte, error) {
			return nil, expected
		})
		if usedStale {
			t.Fatalf("expected no stale fallback when stale key was never written")
		}
		if !errors.Is(err, expected) {
			t.Fatalf("expected original callback error, got %v", err)
		}
	})
}

func TestRememberStaleTypedFreshAndFallback(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	type profile struct {
		Name string `json:"name"`
	}
	key := "stale:typed"

	val, stale, err := RememberStale[profile](c, key, time.Minute, 5*time.Minute, func() (profile, error) {
		return profile{Name: "Ada"}, nil
	})
	if err != nil || stale || val.Name != "Ada" {
		t.Fatalf("typed seed failed: val=%+v stale=%v err=%v", val, stale, err)
	}

	if err := c.Delete(key); err != nil {
		t.Fatalf("delete fresh key failed: %v", err)
	}

	val, stale, err = RememberStale[profile](c, key, time.Minute, 5*time.Minute, func() (profile, error) {
		return profile{}, errors.New("upstream failed")
	})
	if err != nil || !stale || val.Name != "Ada" {
		t.Fatalf("typed stale fallback failed: val=%+v stale=%v err=%v", val, stale, err)
	}
}

func TestRememberStaleValueWithCodecErrors(t *testing.T) {
	c := NewCache(NewMemoryStore(context.Background()))
	ctx := context.Background()

	if _, _, err := rememberStaleValueWithCodecCtx[int](ctx, c, "stale:codec:nil", time.Minute, time.Minute, nil, defaultValueCodec[int]()); err == nil {
		t.Fatalf("expected nil callback error")
	}

	encodeErr := errors.New("encode boom")
	codec := ValueCodec[int]{
		Encode: func(v int) ([]byte, error) { return nil, encodeErr },
		Decode: func(b []byte) (int, error) { return 0, nil },
	}
	if _, _, err := rememberStaleValueWithCodecCtx[int](ctx, c, "stale:codec:encode", time.Minute, time.Minute, func() (int, error) { return 1, nil }, codec); !errors.Is(err, encodeErr) {
		t.Fatalf("expected encode error, got %v", err)
	}

	decodeErr := errors.New("decode boom")
	decodeCodec := ValueCodec[int]{
		Encode: func(v int) ([]byte, error) { return []byte("1"), nil },
		Decode: func(b []byte) (int, error) { return 0, decodeErr },
	}
	if _, _, err := rememberStaleValueWithCodecCtx[int](ctx, c, "stale:codec:decode", time.Minute, time.Minute, func() (int, error) { return 1, nil }, decodeCodec); !errors.Is(err, decodeErr) {
		t.Fatalf("expected decode error, got %v", err)
	}
}
