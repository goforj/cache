package cachetest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

// Options configures shared store contract checks.
type Options struct {
	// CaseName is used to namespace keys. Defaults to t.Name().
	CaseName string
	// NullSemantics enables relaxed expectations for the null store.
	NullSemantics bool
	// SkipCloneCheck disables the "get returns a cloned value" assertion.
	SkipCloneCheck bool
	// TTL controls the expiry duration used in TTL tests.
	TTL time.Duration
	// TTLWait is how long the harness waits for expiry to occur.
	TTLWait time.Duration
	// SkipFlush disables the flush assertion for drivers where it is expensive or unavailable.
	SkipFlush bool
}

// Store is the minimal contract required by RunStoreContract.
type Store = cachecore.Store

// RunStoreContract runs a backend-agnostic store contract suite.
func RunStoreContract(t *testing.T, store Store, opts Options) {
	t.Helper()

	caseName := opts.CaseName
	if caseName == "" {
		caseName = t.Name()
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 50 * time.Millisecond
	}
	wait := opts.TTLWait
	if wait <= 0 {
		wait = 120 * time.Millisecond
	}

	ctx := context.Background()
	key := func(s string) string {
		return sanitize(caseName) + ":" + s
	}

	// Set/Get round-trip.
	if err := store.Set(ctx, key("alpha"), []byte("value"), time.Second); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, key("alpha"))
	if err != nil {
		t.Fatalf("get failed: ok=%v err=%v", ok, err)
	}
	if opts.NullSemantics {
		if ok {
			t.Fatalf("expected miss for null semantics")
		}
	} else {
		if !ok || string(body) != "value" {
			t.Fatalf("unexpected get result: ok=%v body=%q err=%v", ok, string(body), err)
		}
		if !opts.SkipCloneCheck {
			body[0] = 'X'
			body2, ok2, err2 := store.Get(ctx, key("alpha"))
			if err2 != nil || !ok2 || string(body2) != "value" {
				t.Fatalf("expected stored value unchanged, got ok=%v body=%q err=%v", ok2, string(body2), err2)
			}
		}
	}

	// TTL expiry.
	if err := store.Set(ctx, key("ttl"), []byte("v"), ttl); err != nil {
		t.Fatalf("set ttl failed: %v", err)
	}
	if err := waitForMiss(ctx, store, key("ttl"), wait); err != nil {
		t.Fatalf("expected ttl expiry: %v", err)
	}

	// Add only when missing.
	created, err := store.Add(ctx, key("once"), []byte("first"), time.Second)
	if err != nil {
		t.Fatalf("add first failed: created=%v err=%v", created, err)
	}
	created, err = store.Add(ctx, key("once"), []byte("second"), time.Second)
	if err != nil {
		t.Fatalf("add duplicate failed: %v", err)
	}
	if opts.NullSemantics {
		if !created {
			t.Fatalf("expected null-like add duplicate to report created=true")
		}
	} else if created {
		t.Fatalf("expected duplicate add to return created=false")
	}

	// Counters.
	n, err := store.Increment(ctx, key("counter"), 3, time.Second)
	if err != nil {
		t.Fatalf("increment failed: %v", err)
	}
	if opts.NullSemantics {
		if n != 0 {
			t.Fatalf("expected null-like increment to return 0, got %d", n)
		}
	} else if n != 3 {
		t.Fatalf("expected increment=3, got %d", n)
	}
	n, err = store.Decrement(ctx, key("counter"), 1, time.Second)
	if err != nil {
		t.Fatalf("decrement failed: %v", err)
	}
	if opts.NullSemantics {
		if n != 0 {
			t.Fatalf("expected null-like decrement to return 0, got %d", n)
		}
	} else if n != 2 {
		t.Fatalf("expected decrement=2, got %d", n)
	}

	// Delete and DeleteMany.
	if err := store.Set(ctx, key("a"), []byte("1"), time.Second); err != nil {
		t.Fatalf("set a failed: %v", err)
	}
	if err := store.Set(ctx, key("b"), []byte("2"), time.Second); err != nil {
		t.Fatalf("set b failed: %v", err)
	}
	if err := store.Delete(ctx, key("a")); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := store.DeleteMany(ctx, key("b")); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, key("a")); err != nil || ok {
		t.Fatalf("expected key a deleted; ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.Get(ctx, key("b")); err != nil || ok {
		t.Fatalf("expected key b deleted; ok=%v err=%v", ok, err)
	}

	// Flush.
	if !opts.SkipFlush {
		if err := store.Set(ctx, key("flush"), []byte("x"), time.Second); err != nil {
			t.Fatalf("set flush failed: %v", err)
		}
		if err := store.Flush(ctx); err != nil {
			t.Fatalf("flush failed: %v", err)
		}
		if _, ok, err := store.Get(ctx, key("flush")); err != nil || ok {
			t.Fatalf("expected flush to clear key; ok=%v err=%v", ok, err)
		}
	}

}

func waitForMiss(ctx context.Context, store Store, key string, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		_, ok, err := store.Get(ctx, key)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, ok, err := store.Get(ctx, key)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("key %q still present after %s", key, wait)
	}
	return nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}
