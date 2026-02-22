package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type observerEvent struct {
	op     string
	key    string
	hit    bool
	err    error
	driver Driver
	dur    time.Duration
}

type observerRecorder struct {
	mu     sync.Mutex
	events []observerEvent
}

func (r *observerRecorder) OnCacheOp(_ context.Context, op, key string, hit bool, err error, dur time.Duration, driver Driver) {
	r.mu.Lock()
	r.events = append(r.events, observerEvent{
		op:     op,
		key:    key,
		hit:    hit,
		err:    err,
		driver: driver,
		dur:    dur,
	})
	r.mu.Unlock()
}

func (r *observerRecorder) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func (r *observerRecorder) eventsSince(n int) []observerEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n >= len(r.events) {
		return nil
	}
	out := make([]observerEvent, len(r.events)-n)
	copy(out, r.events[n:])
	return out
}

func TestObserverContract_HelperOpsEmitExpectedMetadata(t *testing.T) {
	ctx := context.Background()
	obs := &observerRecorder{}
	c := NewCache(newMemoryStore(0, 0)).WithObserver(obs)

	assertLast := func(t *testing.T, before int, wantOp, wantKey string, wantHit bool, wantErr error) observerEvent {
		t.Helper()
		events := obs.eventsSince(before)
		if len(events) == 0 {
			t.Fatalf("expected observer event %q, got none", wantOp)
		}
		got := events[len(events)-1]
		if got.op != wantOp {
			t.Fatalf("expected op=%q, got %q (segment=%+v)", wantOp, got.op, events)
		}
		if wantKey != "*" && got.key != wantKey {
			t.Fatalf("expected key=%q, got %q", wantKey, got.key)
		}
		if got.hit != wantHit {
			t.Fatalf("expected hit=%v, got %v", wantHit, got.hit)
		}
		if wantErr == nil && got.err != nil {
			t.Fatalf("expected nil err, got %v", got.err)
		}
		if wantErr != nil && !errors.Is(got.err, wantErr) {
			t.Fatalf("expected error %v, got %v", wantErr, got.err)
		}
		if got.driver != DriverMemory {
			t.Fatalf("expected driver=%q, got %q", DriverMemory, got.driver)
		}
		if got.dur < 0 {
			t.Fatalf("expected non-negative duration, got %v", got.dur)
		}
		return got
	}

	t.Run("get_hit_and_miss", func(t *testing.T) {
		before := obs.len()
		if _, ok, err := c.GetCtx(ctx, "missing:get"); err != nil || ok {
			t.Fatalf("expected miss: ok=%v err=%v", ok, err)
		}
		assertLast(t, before, "get", "missing:get", false, nil)

		if err := c.SetCtx(ctx, "present:get", []byte("v"), time.Minute); err != nil {
			t.Fatalf("set failed: %v", err)
		}
		before = obs.len()
		if _, ok, err := c.GetCtx(ctx, "present:get"); err != nil || !ok {
			t.Fatalf("expected hit: ok=%v err=%v", ok, err)
		}
		assertLast(t, before, "get", "present:get", true, nil)
	})

	t.Run("string_and_json_helpers", func(t *testing.T) {
		before := obs.len()
		if err := c.SetStringCtx(ctx, "s:key", "value", time.Minute); err != nil {
			t.Fatalf("set string failed: %v", err)
		}
		assertLast(t, before, "set_string", "s:key", false, nil)

		before = obs.len()
		if _, ok, err := c.GetStringCtx(ctx, "s:key"); err != nil || !ok {
			t.Fatalf("get string failed: ok=%v err=%v", ok, err)
		}
		assertLast(t, before, "get_string", "s:key", true, nil)

		type payload struct{ Name string `json:"name"` }
		before = obs.len()
		if err := SetJSONCtx(ctx, c, "j:key", payload{Name: "Ada"}, time.Minute); err != nil {
			t.Fatalf("set json failed: %v", err)
		}
		assertLast(t, before, "set_json", "j:key", false, nil)

		before = obs.len()
		if _, ok, err := GetJSONCtx[payload](ctx, c, "j:key"); err != nil || !ok {
			t.Fatalf("get json failed: ok=%v err=%v", ok, err)
		}
		assertLast(t, before, "get_json", "j:key", true, nil)
	})

	t.Run("add_and_counters", func(t *testing.T) {
		before := obs.len()
		created, err := c.AddCtx(ctx, "add:key", []byte("1"), time.Minute)
		if err != nil || !created {
			t.Fatalf("add create failed: created=%v err=%v", created, err)
		}
		assertLast(t, before, "add", "add:key", true, nil)

		before = obs.len()
		created, err = c.AddCtx(ctx, "add:key", []byte("2"), time.Minute)
		if err != nil || created {
			t.Fatalf("add duplicate failed: created=%v err=%v", created, err)
		}
		assertLast(t, before, "add", "add:key", false, nil)

		before = obs.len()
		if _, err := c.IncrementCtx(ctx, "ctr:key", 2, time.Minute); err != nil {
			t.Fatalf("increment failed: %v", err)
		}
		assertLast(t, before, "increment", "ctr:key", true, nil)

		before = obs.len()
		if _, err := c.DecrementCtx(ctx, "ctr:key", 1, time.Minute); err != nil {
			t.Fatalf("decrement failed: %v", err)
		}
		assertLast(t, before, "decrement", "ctr:key", true, nil)
	})

	t.Run("locking", func(t *testing.T) {
		before := obs.len()
		locked, err := c.TryLockCtx(ctx, "lock:key", time.Minute)
		if err != nil || !locked {
			t.Fatalf("try lock failed: locked=%v err=%v", locked, err)
		}
		assertLast(t, before, "try_lock", "lock:key", true, nil)

		before = obs.len()
		if err := c.UnlockCtx(ctx, "lock:key"); err != nil {
			t.Fatalf("unlock failed: %v", err)
		}
		assertLast(t, before, "unlock", "lock:key", true, nil)

		before = obs.len()
		locked, err = c.LockCtx(ctx, "lock:key2", time.Minute, time.Millisecond)
		if err != nil || !locked {
			t.Fatalf("lock failed: locked=%v err=%v", locked, err)
		}
		assertLast(t, before, "lock", "lock:key2", true, nil)
		_ = c.UnlockCtx(ctx, "lock:key2")
	})

	t.Run("pull_delete_delete_many_flush", func(t *testing.T) {
		if err := c.SetCtx(ctx, "pull:key", []byte("v"), time.Minute); err != nil {
			t.Fatalf("seed pull failed: %v", err)
		}
		before := obs.len()
		if _, ok, err := c.PullCtx(ctx, "pull:key"); err != nil || !ok {
			t.Fatalf("pull failed: ok=%v err=%v", ok, err)
		}
		assertLast(t, before, "pull", "pull:key", true, nil)

		if err := c.SetCtx(ctx, "del:key", []byte("v"), time.Minute); err != nil {
			t.Fatalf("seed delete failed: %v", err)
		}
		before = obs.len()
		if err := c.DeleteCtx(ctx, "del:key"); err != nil {
			t.Fatalf("delete failed: %v", err)
		}
		assertLast(t, before, "delete", "del:key", true, nil)

		if err := c.SetCtx(ctx, "dm:a", []byte("1"), time.Minute); err != nil {
			t.Fatalf("seed delete many a failed: %v", err)
		}
		if err := c.SetCtx(ctx, "dm:b", []byte("2"), time.Minute); err != nil {
			t.Fatalf("seed delete many b failed: %v", err)
		}
		before = obs.len()
		if err := c.DeleteManyCtx(ctx, "dm:a", "dm:b"); err != nil {
			t.Fatalf("delete many failed: %v", err)
		}
		events := obs.eventsSince(before)
		if len(events) != 2 {
			t.Fatalf("expected 2 delete_many events, got %d (%+v)", len(events), events)
		}
		for i, key := range []string{"dm:a", "dm:b"} {
			if events[i].op != "delete_many" || events[i].key != key || !events[i].hit || events[i].err != nil || events[i].driver != DriverMemory {
				t.Fatalf("unexpected delete_many event[%d]=%+v", i, events[i])
			}
		}

		before = obs.len()
		if err := c.FlushCtx(ctx); err != nil {
			t.Fatalf("flush failed: %v", err)
		}
		assertLast(t, before, "flush", "", true, nil)
	})

	t.Run("read_through_and_refresh_helpers", func(t *testing.T) {
		before := obs.len()
		if _, err := c.RememberCtx(ctx, "remember:key", time.Minute, func(context.Context) ([]byte, error) {
			return []byte("v"), nil
		}); err != nil {
			t.Fatalf("remember failed: %v", err)
		}
		assertLast(t, before, "remember", "remember:key", true, nil)

		before = obs.len()
		if _, err := c.RememberStringCtx(ctx, "remember:string", time.Minute, func(context.Context) (string, error) {
			return "v", nil
		}); err != nil {
			t.Fatalf("remember string failed: %v", err)
		}
		assertLast(t, before, "remember_string", "remember:string", true, nil)

		// RememberJSON emits get_json/set_json observer ops (no dedicated remember_json op).
		type payload struct{ Name string `json:"name"` }
		before = obs.len()
		if _, err := RememberJSONCtx[payload](ctx, c, "remember:json", time.Minute, func(context.Context) (payload, error) {
			return payload{Name: "Ada"}, nil
		}); err != nil {
			t.Fatalf("remember json failed: %v", err)
		}
		assertLast(t, before, "set_json", "remember:json", false, nil)

		before = obs.len()
		if _, _, err := RememberStaleCtx[string](ctx, c, "remember:stale", time.Minute, 2*time.Minute, func(context.Context) (string, error) {
			return "v", nil
		}); err != nil {
			t.Fatalf("remember stale failed: %v", err)
		}
		assertLast(t, before, "remember_stale", "remember:stale", true, nil)

		before = obs.len()
		if _, err := c.RefreshAheadCtx(ctx, "refresh:miss", time.Minute, 10*time.Second, func(context.Context) ([]byte, error) {
			return []byte("v"), nil
		}); err != nil {
			t.Fatalf("refresh ahead miss failed: %v", err)
		}
		assertLast(t, before, "refresh_ahead", "refresh:miss", true, nil)
	})

	t.Run("rate_limit_helpers_emit_increment_ops", func(t *testing.T) {
		before := obs.len()
		if _, _, err := c.RateLimitCtx(ctx, "rl:key", 5, time.Minute); err != nil {
			t.Fatalf("rate limit failed: %v", err)
		}
		last := assertLast(t, before, "increment", "*", true, nil)
		if len(last.key) == 0 || last.key[:6] != "rl:key" {
			t.Fatalf("expected rate limit increment key prefix, got %q", last.key)
		}

		before = obs.len()
		if _, _, _, _, err := c.RateLimitWithRemainingCtx(ctx, "rl:key2", 5, time.Minute); err != nil {
			t.Fatalf("rate limit with remaining failed: %v", err)
		}
		last = assertLast(t, before, "increment", "*", true, nil)
		if len(last.key) == 0 || last.key[:7] != "rl:key2" {
			t.Fatalf("expected rate limit increment key prefix, got %q", last.key)
		}
	})
}

func TestObserverContract_ErrorPropagation(t *testing.T) {
	ctx := context.Background()
	obs := &observerRecorder{}
	c := NewCache(newMemoryStore(0, 0)).WithObserver(obs)

	lastEvent := func(t *testing.T, before int) observerEvent {
		t.Helper()
		events := obs.eventsSince(before)
		if len(events) == 0 {
			t.Fatalf("expected observer event, got none")
		}
		return events[len(events)-1]
	}

	t.Run("get_json_decode_error", func(t *testing.T) {
		if err := c.SetCtx(ctx, "bad:json", []byte("{"), time.Minute); err != nil {
			t.Fatalf("seed bad json failed: %v", err)
		}
		before := obs.len()
		_, ok, err := GetJSONCtx[map[string]any](ctx, c, "bad:json")
		if err == nil || ok {
			t.Fatalf("expected decode error and miss semantics: ok=%v err=%v", ok, err)
		}
		got := lastEvent(t, before)
		if got.op != "get_json" || got.hit {
			t.Fatalf("unexpected observer event: %+v", got)
		}
		if got.err == nil {
			t.Fatalf("expected observer error for decode failure")
		}
		if got.driver != DriverMemory {
			t.Fatalf("expected driver memory, got %q", got.driver)
		}
	})

	t.Run("remember_nil_callback", func(t *testing.T) {
		before := obs.len()
		_, err := c.RememberCtx(ctx, "remember:nil", time.Minute, nil)
		if err == nil {
			t.Fatalf("expected error")
		}
		got := lastEvent(t, before)
		if got.op != "remember" || got.hit || got.err == nil {
			t.Fatalf("unexpected observer event: %+v", got)
		}
	})

	t.Run("refresh_ahead_nil_callback", func(t *testing.T) {
		before := obs.len()
		_, err := c.RefreshAheadCtx(ctx, "refresh:nil", time.Minute, 10*time.Second, nil)
		if err == nil {
			t.Fatalf("expected error")
		}
		got := lastEvent(t, before)
		if got.op != "refresh_ahead" || got.hit || got.err == nil {
			t.Fatalf("unexpected observer event: %+v", got)
		}
	})

	t.Run("lock_context_timeout", func(t *testing.T) {
		locked, err := c.TryLockCtx(ctx, "lock:timeout", time.Minute)
		if err != nil || !locked {
			t.Fatalf("seed lock failed: locked=%v err=%v", locked, err)
		}
		t.Cleanup(func() { _ = c.UnlockCtx(context.Background(), "lock:timeout") })

		lockCtx, cancel := context.WithTimeout(ctx, 15*time.Millisecond)
		defer cancel()

		before := obs.len()
		locked, err = c.LockCtx(lockCtx, "lock:timeout", time.Minute, time.Millisecond)
		if err == nil || locked {
			t.Fatalf("expected timeout error: locked=%v err=%v", locked, err)
		}
		got := lastEvent(t, before)
		if got.op != "lock" || got.hit || !errors.Is(got.err, context.DeadlineExceeded) {
			t.Fatalf("unexpected observer event: %+v", got)
		}
	})
}
