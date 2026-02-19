package cache

import (
	"context"
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

	first, err := repo.Remember(ctx, "k", time.Minute, fn)
	if err != nil {
		t.Fatalf("remember failed: %v", err)
	}
	second, err := repo.Remember(ctx, "k", time.Minute, fn)
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

func TestCacheRememberJSON(t *testing.T) {
	repo := NewCache(newMemoryStore(0, 0))
	ctx := context.Background()

	calls := 0
	value, err := RememberJSON[testPayload](ctx, repo, "json", time.Minute, func(context.Context) (testPayload, error) {
		calls++
		return testPayload{Name: "cache"}, nil
	})
	if err != nil {
		t.Fatalf("remember json failed: %v", err)
	}
	if value.Name != "cache" {
		t.Fatalf("unexpected payload: %+v", value)
	}

	value, err = RememberJSON[testPayload](ctx, repo, "json", time.Minute, func(context.Context) (testPayload, error) {
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

	if err := SetJSON(ctx, repo, "u", testPayload{Name: "alex"}, time.Minute); err != nil {
		t.Fatalf("set json failed: %v", err)
	}
	got, ok, err := GetJSON[testPayload](ctx, repo, "u")
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

	created, err := repo.Add(ctx, "add", []byte("x"), time.Minute)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if !created {
		t.Fatalf("expected first add to create key")
	}
	created, err = repo.Add(ctx, "add", []byte("y"), time.Minute)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if created {
		t.Fatalf("expected second add to be ignored")
	}

	value, err := repo.Increment(ctx, "counter", 3, time.Minute)
	if err != nil {
		t.Fatalf("increment failed: %v", err)
	}
	if value != 3 {
		t.Fatalf("expected 3, got %d", value)
	}
	value, err = repo.Decrement(ctx, "counter", 1, time.Minute)
	if err != nil {
		t.Fatalf("decrement failed: %v", err)
	}
	if value != 2 {
		t.Fatalf("expected 2, got %d", value)
	}

	if err := repo.SetString(ctx, "pull", "value", time.Minute); err != nil {
		t.Fatalf("set string failed: %v", err)
	}
	body, ok, err := repo.Pull(ctx, "pull")
	if err != nil {
		t.Fatalf("pull failed: %v", err)
	}
	if !ok || string(body) != "value" {
		t.Fatalf("unexpected pull result")
	}
	_, ok, err = repo.Get(ctx, "pull")
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

	if err := repo.SetString(ctx, "a", "1", time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := repo.SetString(ctx, "b", "2", time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := repo.DeleteMany(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	_, ok, err := repo.Get(ctx, "a")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected deleted key")
	}

	if err := repo.SetString(ctx, "c", "3", time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := repo.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	_, ok, err = repo.Get(ctx, "c")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected flush to clear key")
	}

	if _, err := repo.Remember(ctx, "missing", time.Minute, nil); err == nil {
		t.Fatalf("expected remember nil callback error")
	}
	if _, err := repo.RememberString(ctx, "missing-string", time.Minute, nil); err == nil {
		t.Fatalf("expected remember string nil callback error")
	}
	_, err = RememberJSON[testPayload](ctx, repo, "missing-json", time.Minute, nil)
	if err == nil {
		t.Fatalf("expected remember json nil callback error")
	}

	expected := errors.New("boom")
	_, err = repo.Remember(ctx, "broken", time.Minute, func(context.Context) ([]byte, error) {
		return nil, expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
}
