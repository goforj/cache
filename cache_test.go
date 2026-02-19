package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
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

	first, err := repo.RememberCtx(ctx, "k", time.Minute, fn)
	if err != nil {
		t.Fatalf("remember failed: %v", err)
	}
	second, err := repo.RememberCtx(ctx, "k", time.Minute, fn)
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
	value, err := RememberJSONCtx[testPayload](ctx, repo, "json", time.Minute, func(context.Context) (testPayload, error) {
		calls++
		return testPayload{Name: "cache"}, nil
	})
	if err != nil {
		t.Fatalf("remember json failed: %v", err)
	}
	if value.Name != "cache" {
		t.Fatalf("unexpected payload: %+v", value)
	}

	value, err = RememberJSONCtx[testPayload](ctx, repo, "json", time.Minute, func(context.Context) (testPayload, error) {
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
	body, ok, err := repo.Pull("pull")
	if err != nil {
		t.Fatalf("pull failed: %v", err)
	}
	if !ok || string(body) != "value" {
		t.Fatalf("unexpected pull result")
	}
	_, ok, err = repo.GetCtx(ctx, "pull")
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
	_, ok, err := repo.GetCtx(ctx, "a")
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
	_, ok, err = repo.GetCtx(ctx, "c")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected flush to clear key")
	}

	if _, err := repo.RememberCtx(ctx, "missing", time.Minute, nil); err == nil {
		t.Fatalf("expected remember nil callback error")
	}
	if _, err := repo.RememberStringCtx(ctx, "missing-string", time.Minute, nil); err == nil {
		t.Fatalf("expected remember string nil callback error")
	}
	_, err = RememberJSONCtx[testPayload](ctx, repo, "missing-json", time.Minute, nil)
	if err == nil {
		t.Fatalf("expected remember json nil callback error")
	}

	expected := errors.New("boom")
	_, err = repo.RememberCtx(ctx, "broken", time.Minute, func(context.Context) ([]byte, error) {
		return nil, expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
}

type spyStore struct {
	driver   Driver
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

func (s *spyStore) Driver() Driver { return s.driver }

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
	store := &spyStore{driver: DriverMemory}
	c := NewCache(store)
	if c.Store() != store {
		t.Fatalf("expected Store to return underlying store")
	}
	if c.Driver() != DriverMemory {
		t.Fatalf("expected driver to propagate")
	}
}

func TestCacheRememberStringUsesResolvedTTL(t *testing.T) {
	store := &spyStore{driver: DriverMemory}
	c := NewCacheWithTTL(store, 2*time.Second)
	ctx := context.Background()

	calls := 0
	val, err := c.RememberStringCtx(ctx, "k", 0, func(context.Context) (string, error) {
		calls++
		return "hello", nil
	})
	if err != nil || val != "hello" {
		t.Fatalf("remember string failed: %v %q", err, val)
	}
	val, err = c.RememberStringCtx(ctx, "k", time.Second, func(context.Context) (string, error) {
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
	store := &spyStore{driver: DriverMemory}
	c := NewCacheWithTTL(store, time.Minute)
	ctx := context.Background()

	if err := c.SetCtx(ctx, "k", []byte("v"), 3*time.Second); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if len(store.ttls) != 1 || store.ttls[0] != 3*time.Second {
		t.Fatalf("expected ttl=3s, got %v", store.ttls)
	}
}

func TestNewCacheWithTTLDefaultsWhenNonPositive(t *testing.T) {
	store := &spyStore{driver: DriverMemory}
	c := NewCacheWithTTL(store, -1)
	ctx := context.Background()

	_, _ = c.RememberStringCtx(ctx, "k", 0, func(context.Context) (string, error) {
		return "v", nil
	})
	if len(store.ttls) != 1 || store.ttls[0] != defaultCacheTTL {
		t.Fatalf("expected default cache ttl, got %v", store.ttls)
	}
}

func TestCacheGetStringError(t *testing.T) {
	expected := errors.New("boom")
	store := &spyStore{driver: DriverMemory, getErr: expected}
	c := NewCache(store)
	ctx := context.Background()
	_, _, err := c.GetStringCtx(ctx, "k")
	if !errors.Is(err, expected) {
		t.Fatalf("expected propagated error")
	}
}

func TestCacheGetStringSuccess(t *testing.T) {
	store := &spyStore{driver: DriverMemory, getOK: true, getBody: []byte("ok")}
	c := NewCache(store)
	ctx := context.Background()
	val, ok, err := c.GetStringCtx(ctx, "k")
	if err != nil || !ok || val != "ok" {
		t.Fatalf("unexpected result: val=%q ok=%v err=%v", val, ok, err)
	}
}

func TestCacheSetJSONMarshalError(t *testing.T) {
	store := &spyStore{driver: DriverMemory}
	c := NewCache(store)
	ctx := context.Background()
	ch := make(chan int)
	if err := SetJSONCtx(ctx, c, "bad", ch, time.Second); err == nil {
		t.Fatalf("expected marshal error")
	}
}

func TestCachePullDeleteError(t *testing.T) {
	store := &spyStore{driver: DriverMemory, getOK: true, getBody: []byte("x"), delErr: expectedErr}
	c := NewCache(store)
	_, _, err := c.Pull("k")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected delete error, got %v", err)
	}
}

func TestCacheRememberSetError(t *testing.T) {
	store := &spyStore{driver: DriverMemory, setErr: expectedErr}
	c := NewCache(store)
	ctx := context.Background()
	_, err := c.RememberCtx(ctx, "k", time.Second, func(context.Context) ([]byte, error) {
		return []byte("x"), nil
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected set error")
	}
}

func TestCacheRememberGetError(t *testing.T) {
	store := &spyStore{driver: DriverMemory, getErr: expectedErr}
	c := NewCache(store)
	ctx := context.Background()
	if _, err := c.RememberCtx(ctx, "k", time.Second, func(context.Context) ([]byte, error) { return []byte("x"), nil }); !errors.Is(err, expectedErr) {
		t.Fatalf("expected get error")
	}
}

func TestCacheRememberJSONCallbackError(t *testing.T) {
	store := &spyStore{driver: DriverMemory}
	c := NewCache(store)
	ctx := context.Background()
	expected := errors.New("cb")
	_, err := RememberJSONCtx[int](ctx, c, "k", time.Second, func(context.Context) (int, error) {
		return 0, expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error")
	}
}

func TestCacheRememberJSONSetError(t *testing.T) {
	store := &spyStore{driver: DriverMemory, setErr: expectedErr}
	c := NewCache(store)
	ctx := context.Background()
	_, err := RememberJSONCtx[int](ctx, c, "k", time.Second, func(context.Context) (int, error) { return 5, nil })
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected set error")
	}
}

func TestCacheRememberStringCallbackError(t *testing.T) {
	store := &spyStore{driver: DriverMemory}
	c := NewCache(store)
	ctx := context.Background()
	expected := errors.New("cb")
	if _, err := c.RememberStringCtx(ctx, "k", time.Second, func(context.Context) (string, error) {
		return "", expected
	}); !errors.Is(err, expected) {
		t.Fatalf("expected callback error")
	}
}

func TestCacheRememberStringUsesCachedValueWithoutCallback(t *testing.T) {
	store := &spyStore{driver: DriverMemory, getOK: true, getBody: []byte("cached")}
	c := NewCache(store)
	ctx := context.Background()
	calls := 0
	val, err := c.RememberStringCtx(ctx, "k", time.Second, func(context.Context) (string, error) {
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
	store := &spyStore{driver: DriverMemory, getOK: true, getBody: body}
	c := NewCache(store)
	ctx := context.Background()
	calls := 0
	result, err := RememberJSONCtx[struct {
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
	store := &spyStore{driver: DriverMemory}
	c := NewCache(store)
	ctx := context.Background()
	if _, err := RememberJSONCtx[int](ctx, c, "k", time.Second, nil); err == nil {
		t.Fatalf("expected nil callback error")
	}
}

func TestCacheRememberJSONGetError(t *testing.T) {
	store := &spyStore{driver: DriverMemory, getOK: true, getBody: []byte("not-json")}
	c := NewCache(store)
	ctx := context.Background()
	if _, err := RememberJSONCtx[int](ctx, c, "k", time.Second, func(context.Context) (int, error) { return 1, nil }); err == nil {
		t.Fatalf("expected get decode error")
	}
}

func TestCacheGetStringMissing(t *testing.T) {
	store := &spyStore{driver: DriverMemory, getOK: false}
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
	store := &spyStore{driver: DriverMemory, getOK: true, getBody: []byte("not-json")}
	c := NewCache(store)
	ctx := context.Background()
	_, ok, err := GetJSONCtx[struct{}](ctx, c, "bad")
	if err == nil || ok {
		t.Fatalf("expected decode error")
	}
}

func TestCachePullMissing(t *testing.T) {
	store := &spyStore{driver: DriverMemory, getOK: false}
	c := NewCache(store)
	_, ok, err := c.Pull("none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false when missing")
	}
}

func TestCacheRememberPropagatesCallbackError(t *testing.T) {
	store := &spyStore{driver: DriverMemory}
	c := NewCache(store)
	ctx := context.Background()
	expected := errors.New("boom")
	_, err := c.RememberCtx(ctx, "k", time.Second, func(context.Context) ([]byte, error) {
		return nil, expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
}
