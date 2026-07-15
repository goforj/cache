package rediscache

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/cachetest"
)

// TestNewNilClientErrors verifies construction fails fast when no Redis client is supplied.
func TestNewNilClientErrors(t *testing.T) {
	store := New(Config{})
	if err := store.Ready(context.Background()); err == nil {
		t.Fatalf("expected ready error when redis client is nil")
	}
	if _, _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatalf("expected get error when redis client is nil")
	}
	if err := store.Set(context.Background(), "k", []byte("v"), 0); err == nil {
		t.Fatalf("expected set error when redis client is nil")
	}
	if err := store.Delete(context.Background(), "k"); err == nil {
		t.Fatalf("expected delete error when redis client is nil")
	}
	if _, err := store.Add(context.Background(), "k", []byte("v"), 0); err == nil {
		t.Fatalf("expected add error when redis client is nil")
	}
	if _, err := store.Increment(context.Background(), "k", 1, 0); err == nil {
		t.Fatalf("expected increment error when redis client is nil")
	}
	if _, err := store.Decrement(context.Background(), "k", 1, 0); err == nil {
		t.Fatalf("expected decrement error when redis client is nil")
	}
	if err := store.DeleteMany(context.Background(), "a", "b"); err == nil {
		t.Fatalf("expected delete many error when redis client is nil")
	}
	if err := store.Flush(context.Background()); err == nil {
		t.Fatalf("expected flush error when redis client is nil")
	}
}

// TestNewAppliesShapingConfiguration verifies the driver honors shared BaseConfig fields.
func TestNewAppliesShapingConfiguration(t *testing.T) {
	client := newStubClient()
	store := New(Config{
		Client: client,
		BaseConfig: cachecore.BaseConfig{
			Compression:   cachecore.CompressionGzip,
			EncryptionKey: []byte("0123456789abcdef0123456789abcdef"),
		},
	})
	ctx := context.Background()
	if err := store.Set(ctx, "secret", []byte("payload"), time.Minute); err != nil {
		t.Fatalf("set shaped value: %v", err)
	}
	if raw := client.store["app:secret"]; !strings.HasPrefix(raw, "CMP1") {
		t.Fatalf("persisted value is not shaped: %x", raw)
	}
	got, ok, err := store.Get(ctx, "secret")
	if err != nil || !ok || string(got) != "payload" {
		t.Fatalf("shaped round trip: got=%q ok=%v err=%v", got, ok, err)
	}
}

// TestNewShapingConfigFailureFailsClosed verifies Store-only construction preserves the config error.
func TestNewShapingConfigFailureFailsClosed(t *testing.T) {
	store := New(Config{BaseConfig: cachecore.BaseConfig{EncryptionKey: []byte("short")}})
	if err := store.Ready(context.Background()); !errors.Is(err, cachecore.ErrEncryptionKey) {
		t.Fatalf("Ready error = %v, want ErrEncryptionKey", err)
	}
}

// TestNewWithAddrCreatesClient verifies address-based construction owns a usable Redis client.
func TestNewWithAddrCreatesClient(t *testing.T) {
	s := New(Config{Addr: "127.0.0.1:6379"})
	impl, ok := s.(*store)
	if !ok {
		t.Fatalf("expected *store, got %T", s)
	}
	if impl.client == nil {
		t.Fatalf("expected client to be auto-created when Addr is set")
	}
}

// TestOperationsWithStubClient verifies Redis commands implement the complete store operation contract.
func TestOperationsWithStubClient(t *testing.T) {
	ctx := context.Background()
	client := newStubClient()
	store := New(Config{Client: client, BaseConfig: cachecore.BaseConfig{Prefix: "pfx"}})
	if err := store.Ready(ctx); err != nil {
		t.Fatalf("ready failed: %v", err)
	}

	if err := store.Set(ctx, "alpha", []byte("one"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "alpha")
	if err != nil || !ok || string(body) != "one" {
		t.Fatalf("unexpected get result: ok=%v err=%v body=%s", ok, err, string(body))
	}

	created, err := store.Add(ctx, "alpha", []byte("two"), 0)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if created {
		t.Fatalf("expected add false when key exists")
	}

	val, err := store.Increment(ctx, "counter", 2, time.Second)
	if err != nil || val != 2 {
		t.Fatalf("increment failed: val=%d err=%v", val, err)
	}
	val, err = store.Decrement(ctx, "counter", 1, time.Second)
	if err != nil || val != 1 {
		t.Fatalf("decrement failed: val=%d err=%v", val, err)
	}
	if ttl, ok := client.ttl["pfx:counter"]; !ok || ttl.Before(time.Now().Add(900*time.Millisecond)) {
		t.Fatalf("expected expire to be set, got %v", ttl)
	}

	if val, err := store.Increment(ctx, "defaultttl", 1, 0); err != nil || val != 1 {
		t.Fatalf("increment with default ttl failed: val=%d err=%v", val, err)
	}
	if ttl, ok := client.ttl["pfx:defaultttl"]; !ok || ttl.Before(time.Now().Add(defaultTTL-time.Second)) {
		t.Fatalf("expected default ttl to be applied, got %v", ttl)
	}

	if err := store.Delete(ctx, "alpha"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := store.DeleteMany(ctx); err != nil {
		t.Fatalf("delete many empty failed: %v", err)
	}
	if err := store.Set(ctx, "a", []byte("1"), 0); err != nil {
		t.Fatalf("set a failed: %v", err)
	}
	if err := store.Set(ctx, "b", []byte("2"), 0); err != nil {
		t.Fatalf("set b failed: %v", err)
	}
	if err := store.DeleteMany(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}

	if err := store.Set(ctx, "flushme", []byte("x"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "flushme"); err != nil || ok {
		t.Fatalf("expected flushed key to be gone")
	}
}

// TestReadyErrorPropagation verifies ping failures remain visible to readiness callers.
func TestReadyErrorPropagation(t *testing.T) {
	ctx := context.Background()
	client := newStubClient()
	client.pingErr = errors.New("ping")
	store := New(Config{Client: client, BaseConfig: cachecore.BaseConfig{Prefix: "pfx"}})
	if err := store.Ready(ctx); err == nil {
		t.Fatalf("expected ping error")
	}
}

// TestGetMissing verifies Redis nil responses normalize to cache misses.
func TestGetMissing(t *testing.T) {
	ctx := context.Background()
	client := newStubClient()
	store := New(Config{Client: client, BaseConfig: cachecore.BaseConfig{Prefix: "pfx"}})
	_, ok, err := store.Get(ctx, "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing key")
	}
}

// TestExpireError verifies counter TTL refresh failures are not ignored.
func TestExpireError(t *testing.T) {
	ctx := context.Background()
	client := newStubClient()
	client.expireErr = errors.New("fail expire")
	store := New(Config{Client: client, BaseConfig: cachecore.BaseConfig{Prefix: "pfx"}})
	if _, err := store.Increment(ctx, "k", 1, time.Second); err == nil {
		t.Fatalf("expected expire error")
	}
}

// TestErrorPropagation verifies command failures survive each Redis store boundary.
func TestErrorPropagation(t *testing.T) {
	ctx := context.Background()
	client := newStubClient()
	client.getErr = errors.New("get")
	store := New(Config{Client: client, BaseConfig: cachecore.BaseConfig{Prefix: "pfx"}})
	if _, _, err := store.Get(ctx, "k"); err == nil {
		t.Fatalf("expected get error")
	}

	client = newStubClient()
	client.setNXErr = errors.New("setnx")
	store = New(Config{Client: client, BaseConfig: cachecore.BaseConfig{Prefix: "pfx"}})
	if _, err := store.Add(ctx, "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected add error")
	}

	client = newStubClient()
	client.incrErr = errors.New("incr")
	store = New(Config{Client: client, BaseConfig: cachecore.BaseConfig{Prefix: "pfx"}})
	if _, err := store.Increment(ctx, "k", 1, time.Second); err == nil {
		t.Fatalf("expected incr error")
	}

	client = newStubClient()
	client.scanErr = errors.New("scan")
	store = New(Config{Client: client, BaseConfig: cachecore.BaseConfig{Prefix: "pfx"}})
	if err := store.Flush(ctx); err == nil {
		t.Fatalf("expected flush scan error")
	}

	client = newStubClient()
	client.delErr = errors.New("del")
	client.store["pfx:a"] = "1"
	store = New(Config{Client: client, BaseConfig: cachecore.BaseConfig{Prefix: "pfx"}})
	if err := store.Flush(ctx); err == nil {
		t.Fatalf("expected flush delete error")
	}
}

// TestStoreContractWithStubClient verifies the stubbed Redis backend satisfies the shared store suite.
func TestStoreContractWithStubClient(t *testing.T) {
	store := New(Config{Client: newStubClient(), BaseConfig: cachecore.BaseConfig{Prefix: "contract"}})
	cachetest.RunStoreContract(t, store, cachetest.Options{
		CaseName:       t.Name(),
		SkipCloneCheck: true, // stub client returns mutable bytes from its string-backed response
		TTL:            50 * time.Millisecond,
		TTLWait:        120 * time.Millisecond,
	})
}
