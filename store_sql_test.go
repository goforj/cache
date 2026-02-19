package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func newSQLiteStore(t *testing.T) Store {
	t.Helper()
	dsn := "file::memory:?cache=shared"
	store, err := newSQLStore(StoreConfig{
		SQLDriverName: "sqlite",
		SQLDSN:        dsn,
		SQLTable:      "cache_entries",
		DefaultTTL:    time.Second,
		Prefix:        "p",
	})
	if err != nil {
		t.Fatalf("sqlite store create failed: %v", err)
	}
	return store
}

func TestSQLStoreBasics(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()

	// postgres branch coverage for upsert SQL
	pg := &sqlStore{driverName: "postgres", table: "t"}
	if !strings.Contains(pg.upsertSQL(), "ON CONFLICT") {
		t.Fatalf("expected postgres upsert")
	}
	if !isDuplicateErr(errors.New("duplicate key value violates"), "pgx") {
		t.Fatalf("expected duplicate detection pg")
	}
	if !isDuplicateErr(errors.New("Duplicate entry"), "mysql") {
		t.Fatalf("expected duplicate detection mysql")
	}
	if isDuplicateErr(errors.New("other"), "sqlite") {
		t.Fatalf("unexpected duplicate detection")
	}
	mysql := &sqlStore{driverName: "mysql", table: "t"}
	if !strings.Contains(mysql.upsertSQL(), "ON DUPLICATE") {
		t.Fatalf("expected mysql upsert")
	}
	sqlite := &sqlStore{driverName: "sqlite", table: "t"}
	if !strings.Contains(sqlite.upsertSQL(), "ON CONFLICT") {
		t.Fatalf("expected sqlite upsert")
	}

	if err := store.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "k")
	if err != nil || !ok || string(body) != "v" {
		t.Fatalf("get failed: ok=%v err=%v val=%s", ok, err, string(body))
	}

	created, err := store.Add(ctx, "k", []byte("v2"), time.Minute)
	if err != nil || created {
		t.Fatalf("add duplicate unexpected: created=%v err=%v", created, err)
	}

	val, err := store.Increment(ctx, "n", 2, time.Minute)
	if err != nil || val != 2 {
		t.Fatalf("increment failed: %v val=%d", err, val)
	}
	val, err = store.Decrement(ctx, "n", 1, time.Minute)
	if err != nil || val != 1 {
		t.Fatalf("decrement failed: %v val=%d", err, val)
	}

	if err := store.Delete(ctx, "k"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := store.DeleteMany(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
}

func TestSQLStoreTTLExpiry(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()
	if err := store.Set(ctx, "ttl", []byte("x"), 50*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	_, ok, err := store.Get(ctx, "ttl")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected ttl expiry")
	}
}

func TestSQLStoreIncrementNonNumeric(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()
	if err := store.Set(ctx, "num", []byte("NaN"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if _, err := store.Increment(ctx, "num", 1, time.Minute); err == nil {
		t.Fatalf("expected numeric parse error")
	}
}

func TestSQLStoreDeleteManyEmpty(t *testing.T) {
	store := newSQLiteStore(t)
	if err := store.DeleteMany(context.Background()); err != nil {
		t.Fatalf("delete many empty should be nil: %v", err)
	}
}

func TestSQLStoreCacheKeyNoPrefix(t *testing.T) {
	ss := &sqlStore{prefix: ""}
	if got := ss.cacheKey("k"); got != "k" {
		t.Fatalf("expected raw key, got %s", got)
	}
}

func TestSQLStoreDefaultTableAndTTLFallback(t *testing.T) {
	store, err := newSQLStore(StoreConfig{
		SQLDriverName: "sqlite",
		SQLDSN:        "file::memory:?cache=shared",
		DefaultTTL:    0,
	})
	if err != nil {
		t.Fatalf("store create failed: %v", err)
	}
	ss := store.(*sqlStore)
	if ss.table != "cache_entries" {
		t.Fatalf("expected default table, got %s", ss.table)
	}
	if ss.defaultTTL != defaultCacheTTL {
		t.Fatalf("expected default ttl fallback")
	}
}

func TestSQLStoreErrorsWhenDBClosed(t *testing.T) {
	store := newSQLiteStore(t)
	ss := store.(*sqlStore)
	_ = ss.db.Close()

	ctx := context.Background()
	if _, _, err := store.Get(ctx, "k"); err == nil {
		t.Fatalf("expected get error on closed db")
	}
	if err := store.Set(ctx, "k", []byte("v"), time.Minute); err == nil {
		t.Fatalf("expected set error on closed db")
	}
	if _, err := store.Add(ctx, "k", []byte("v"), time.Minute); err == nil {
		t.Fatalf("expected add error on closed db")
	}
	if _, err := store.Increment(ctx, "k", 1, time.Minute); err == nil {
		t.Fatalf("expected increment error on closed db")
	}
}

func TestSQLStoreIncrementResetsExpired(t *testing.T) {
	store := newSQLiteStore(t)
	ss := store.(*sqlStore)
	ctx := context.Background()

	expired := time.Now().Add(-time.Hour).UnixMilli()
	_, err := ss.db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (k, v, ea) VALUES (?, ?, ?)", ss.table), ss.cacheKey("old"), []byte("5"), expired)
	if err != nil {
		t.Fatalf("insert expired: %v", err)
	}

	val, err := store.Increment(ctx, "old", 1, time.Minute)
	if err != nil {
		t.Fatalf("increment failed: %v", err)
	}
	if val != 1 {
		t.Fatalf("expected reset increment to 1, got %d", val)
	}
}

func TestSQLStoreAddZeroTTL(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()
	created, err := store.Add(ctx, "ttl0", []byte("v"), 0)
	if err != nil || !created {
		t.Fatalf("expected add success with default ttl, err=%v created=%v", err, created)
	}
}

func TestSQLStoreIncrementForUpdateBranch(t *testing.T) {
	store := newSQLiteStore(t)
	ss := store.(*sqlStore)
	ss.driverName = "postgres"
	if _, err := ss.Increment(context.Background(), "k", 1, time.Minute); err == nil {
		t.Fatalf("expected error due to unsupported for update")
	}
}
