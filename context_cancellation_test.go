package cache

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

type blockingCtxStore struct {
	mu sync.Mutex

	getCalls       int
	setCalls       int
	addCalls       int
	incrementCalls int
	deleteCalls    int
}

func (s *blockingCtxStore) Driver() Driver { return DriverMemory }

func (s *blockingCtxStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	s.getCalls++
	s.mu.Unlock()
	<-ctx.Done()
	return nil, false, ctx.Err()
}

func (s *blockingCtxStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	s.setCalls++
	s.mu.Unlock()
	<-ctx.Done()
	return ctx.Err()
}

func (s *blockingCtxStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	s.addCalls++
	s.mu.Unlock()
	<-ctx.Done()
	return false, ctx.Err()
}

func (s *blockingCtxStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	s.mu.Lock()
	s.incrementCalls++
	s.mu.Unlock()
	<-ctx.Done()
	return 0, ctx.Err()
}

func (s *blockingCtxStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

func (s *blockingCtxStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	s.deleteCalls++
	s.mu.Unlock()
	<-ctx.Done()
	return ctx.Err()
}

func (s *blockingCtxStore) DeleteMany(ctx context.Context, keys ...string) error {
	return s.Delete(ctx, "")
}

func (s *blockingCtxStore) Flush(ctx context.Context) error {
	return s.Delete(ctx, "")
}

func (s *blockingCtxStore) snapshot() blockingCtxStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	return blockingCtxStore{
		getCalls:       s.getCalls,
		setCalls:       s.setCalls,
		addCalls:       s.addCalls,
		incrementCalls: s.incrementCalls,
		deleteCalls:    s.deleteCalls,
	}
}

func TestContextCancellation_GetSetLockCtxReturnPromptlyAndRespectContext(t *testing.T) {
	t.Run("getctx", func(t *testing.T) {
		store := &blockingCtxStore{}
		c := NewCache(store)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, ok, err := c.GetBytesCtx(ctx, "k")
		elapsed := time.Since(start)

		if ok {
			t.Fatalf("expected miss on canceled get")
		}
		if err == nil || err != context.DeadlineExceeded {
			t.Fatalf("expected context deadline exceeded, got %v", err)
		}
		if elapsed > 250*time.Millisecond {
			t.Fatalf("getctx returned too slowly after cancellation: %v", elapsed)
		}
		if got := store.snapshot(); got.getCalls != 1 || got.setCalls != 0 || got.addCalls != 0 {
			t.Fatalf("unexpected store calls: %+v", got)
		}
	})

	t.Run("setctx", func(t *testing.T) {
		store := &blockingCtxStore{}
		c := NewCache(store)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := c.SetBytesCtx(ctx, "k", []byte("v"), time.Minute)
		elapsed := time.Since(start)

		if err == nil || err != context.DeadlineExceeded {
			t.Fatalf("expected context deadline exceeded, got %v", err)
		}
		if elapsed > 250*time.Millisecond {
			t.Fatalf("setctx returned too slowly after cancellation: %v", elapsed)
		}
		if got := store.snapshot(); got.setCalls != 1 || got.getCalls != 0 || got.addCalls != 0 {
			t.Fatalf("unexpected store calls: %+v", got)
		}
	})

	t.Run("lockctx", func(t *testing.T) {
		store := &blockingCtxStore{}
		c := NewCache(store)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		start := time.Now()
		locked, err := c.LockCtx(ctx, "k", time.Minute, time.Millisecond)
		elapsed := time.Since(start)

		if locked {
			t.Fatalf("expected lock not acquired on cancellation")
		}
		if err == nil || err != context.DeadlineExceeded {
			t.Fatalf("expected context deadline exceeded, got %v", err)
		}
		if elapsed > 250*time.Millisecond {
			t.Fatalf("lockctx returned too slowly after cancellation: %v", elapsed)
		}
		before := store.snapshot().addCalls
		time.Sleep(10 * time.Millisecond)
		after := store.snapshot().addCalls
		if before != 1 || after != before {
			t.Fatalf("expected exactly one Add call and no post-return retries, before=%d after=%d", before, after)
		}
	})
}

func TestContextCancellation_RefreshAheadAndRememberHelpersDoNotInvokeCallbacks(t *testing.T) {
	type testCase struct {
		name      string
		run       func(t *testing.T, c *Cache, ctx context.Context)
		wantCalls blockingCtxStore
	}

	cases := []testCase{
		{
			name: "refresh_ahead_ctx",
			run: func(t *testing.T, c *Cache, ctx context.Context) {
				t.Helper()
				called := false
				start := time.Now()
				_, err := c.RefreshAheadBytesCtx(ctx, "ra", time.Minute, 10*time.Second, func(context.Context) ([]byte, error) {
					called = true
					return []byte("v"), nil
				})
				elapsed := time.Since(start)
				if err != context.DeadlineExceeded {
					t.Fatalf("expected deadline exceeded, got %v", err)
				}
				if called {
					t.Fatalf("refresh callback should not run when get is canceled")
				}
				if elapsed > 250*time.Millisecond {
					t.Fatalf("refresh_ahead returned too slowly after cancellation: %v", elapsed)
				}
			},
			wantCalls: blockingCtxStore{getCalls: 1},
		},
		{
			name: "remember_ctx",
			run: func(t *testing.T, c *Cache, ctx context.Context) {
				t.Helper()
				called := false
				start := time.Now()
				_, err := c.RememberBytesCtx(ctx, "r", time.Minute, func(context.Context) ([]byte, error) {
					called = true
					return []byte("v"), nil
				})
				elapsed := time.Since(start)
				if err != context.DeadlineExceeded {
					t.Fatalf("expected deadline exceeded, got %v", err)
				}
				if called {
					t.Fatalf("remember callback should not run when get is canceled")
				}
				if elapsed > 250*time.Millisecond {
					t.Fatalf("remember returned too slowly after cancellation: %v", elapsed)
				}
			},
			wantCalls: blockingCtxStore{getCalls: 1},
		},
		{
			name: "remember_string_ctx",
			run: func(t *testing.T, c *Cache, ctx context.Context) {
				t.Helper()
				called := false
				_, err := RememberCtx[string](ctx, c, "rs", time.Minute, func(context.Context) (string, error) {
					called = true
					return "v", nil
				})
				if err != context.DeadlineExceeded {
					t.Fatalf("expected deadline exceeded, got %v", err)
				}
				if called {
					t.Fatalf("remember string callback should not run when get is canceled")
				}
			},
			wantCalls: blockingCtxStore{getCalls: 1},
		},
		{
			name: "remember_json_ctx",
			run: func(t *testing.T, c *Cache, ctx context.Context) {
				t.Helper()
				called := false
				type payload struct{ Name string }
				_, err := RememberCtx[payload](ctx, c, "rj", time.Minute, func(context.Context) (payload, error) {
					called = true
					return payload{Name: "Ada"}, nil
				})
				if err != context.DeadlineExceeded {
					t.Fatalf("expected deadline exceeded, got %v", err)
				}
				if called {
					t.Fatalf("remember json callback should not run when get is canceled")
				}
			},
			wantCalls: blockingCtxStore{getCalls: 1},
		},
		{
			name: "remember_stale_ctx",
			run: func(t *testing.T, c *Cache, ctx context.Context) {
				t.Helper()
				called := false
				_, _, err := RememberStaleCtx[map[string]string](ctx, c, "rst", time.Minute, 2*time.Minute, func(context.Context) (map[string]string, error) {
					called = true
					return map[string]string{"name": "Ada"}, nil
				})
				if err != context.DeadlineExceeded {
					t.Fatalf("expected deadline exceeded, got %v", err)
				}
				if called {
					t.Fatalf("remember stale callback should not run when get is canceled")
				}
			},
			wantCalls: blockingCtxStore{getCalls: 1},
		},
		{
			name: "get_json_ctx",
			run: func(t *testing.T, c *Cache, ctx context.Context) {
				t.Helper()
				type payload struct{ Name string }
				_, ok, err := GetJSONCtx[payload](ctx, c, "gj")
				if ok {
					t.Fatalf("expected miss on canceled get json")
				}
				if err != context.DeadlineExceeded {
					t.Fatalf("expected deadline exceeded, got %v", err)
				}
			},
			wantCalls: blockingCtxStore{getCalls: 1},
		},
		{
			name: "set_json_ctx",
			run: func(t *testing.T, c *Cache, ctx context.Context) {
				t.Helper()
				type payload struct{ Name string }
				err := SetJSONCtx(ctx, c, "sj", payload{Name: "Ada"}, time.Minute)
				if err != context.DeadlineExceeded {
					t.Fatalf("expected deadline exceeded, got %v", err)
				}
			},
			wantCalls: blockingCtxStore{setCalls: 1},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := &blockingCtxStore{}
			c := NewCache(store)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()

			tc.run(t, c, ctx)

			got := store.snapshot()
			if got.getCalls != tc.wantCalls.getCalls || got.setCalls != tc.wantCalls.setCalls || got.addCalls != tc.wantCalls.addCalls || got.incrementCalls != tc.wantCalls.incrementCalls {
				t.Fatalf("unexpected store call counts: got=%+v want=%+v", got, tc.wantCalls)
			}
		})
	}
}

func TestContextCancellation_RememberJSONCtxDoesNotDecodeOrSetAfterCanceledGet(t *testing.T) {
	store := &blockingCtxStore{}
	c := NewCache(store)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	type payload struct{ Name string }
	called := false
	_, err := RememberCtx[payload](ctx, c, "json-cancel", time.Minute, func(context.Context) (payload, error) {
		called = true
		return payload{Name: "Ada"}, nil
	})
	if err != context.DeadlineExceeded {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if called {
		t.Fatalf("callback should not run after canceled get")
	}
	got := store.snapshot()
	if got.getCalls != 1 || got.setCalls != 0 {
		t.Fatalf("expected only one get call, got %+v", got)
	}

	// Sanity guard: encoding path would be available if invoked, proving this test is
	// about cancellation short-circuiting rather than unsupported JSON types.
	if _, err := json.Marshal(payload{Name: "Ada"}); err != nil {
		t.Fatalf("unexpected json marshal error: %v", err)
	}
}
