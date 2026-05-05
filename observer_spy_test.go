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

	_, _ = c.WithContext(ctx).rememberBytes(ctx, "r1", time.Second, func(context.Context) ([]byte, error) { return []byte("v"), nil })
	_, _ = Remember[string](c.WithContext(ctx), "r2", time.Second, func() (string, error) { return "v", nil })
	_, _ = Remember[string](c.WithContext(ctx), "r3", time.Second, func() (string, error) { return "v", nil })
	_, _, _ = c.WithContext(ctx).GetBytes("missing")
	_ = c.WithContext(ctx).Delete("missing")
	_ = c.WithContext(ctx).DeleteMany("missing")
	_ = c.WithContext(ctx).Flush()

	if len(obs.ops) < 6 {
		t.Fatalf("expected observer to record multiple ops, got %v", obs.ops)
	}
}

func TestObserverNilIsSafe(t *testing.T) {
	ctx := context.Background()
	c := NewCache(newMemoryStore(0, 0)) // no observer
	_, _ = c.WithContext(ctx).rememberBytes(ctx, "k", time.Second, func(context.Context) ([]byte, error) { return []byte("v"), nil })
	// ensure no panic when observer nil
}
