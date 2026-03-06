package cache

import (
	"context"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

type spyObserver struct {
	ops []string
}

func (s *spyObserver) OnCacheOp(ctx context.Context, op string, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver) {
	_ = ctx
	_ = key
	_ = hit
	_ = err
	_ = dur
	_ = driver
	s.ops = append(s.ops, op)
}

func TestObserverRecordsAllOps(t *testing.T) {
	ctx := context.Background()
	obs := &spyObserver{}
	c := NewCache(newMemoryStore(0, 0)).WithObserver(obs)

	_, _ = c.RememberBytesContext(ctx, "r1", time.Second, func(context.Context) ([]byte, error) { return []byte("v"), nil })
	_, _ = RememberContext[string](ctx, c, "r2", time.Second, func(context.Context) (string, error) { return "v", nil })
	_, _ = RememberContext[string](ctx, c, "r3", time.Second, func(context.Context) (string, error) { return "v", nil })
	_, _, _ = c.GetBytesContext(ctx, "missing")
	_ = c.DeleteContext(ctx, "missing")
	_ = c.DeleteManyContext(ctx, "missing")
	_ = c.FlushContext(ctx)

	if len(obs.ops) < 6 {
		t.Fatalf("expected observer to record multiple ops, got %v", obs.ops)
	}
}

func TestObserverNilIsSafe(t *testing.T) {
	ctx := context.Background()
	c := NewCache(newMemoryStore(0, 0)) // no observer
	_, _ = c.RememberBytesContext(ctx, "k", time.Second, func(context.Context) ([]byte, error) { return []byte("v"), nil })
	// ensure no panic when observer nil
}
