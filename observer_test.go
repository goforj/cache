package cache

import (
	"context"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

type observerSpy struct {
	ops []string
}

func (o *observerSpy) OnCacheOp(_ context.Context, op string, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver) {
	_ = key
	_ = hit
	_ = err
	_ = dur
	_ = driver
	o.ops = append(o.ops, op)
}

func TestWithObserverHooks(t *testing.T) {
	obs := &observerSpy{}
	c := NewCache(newMemoryStore(0, 0)).WithObserver(obs)

	if err := c.SetString("k", "v", time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if _, ok, err := c.GetString("k"); err != nil || !ok {
		t.Fatalf("get failed: %v ok=%v", err, ok)
	}
	if err := c.Delete("k"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if len(obs.ops) < 3 {
		t.Fatalf("expected observer to see ops, got %v", obs.ops)
	}
}
