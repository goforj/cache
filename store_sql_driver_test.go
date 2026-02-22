package cache

import (
	"context"
	"testing"
)

func TestSQLDriverErrorsWhenMissingDSN(t *testing.T) {
	store, err := newSQLStore(StoreConfig{SQLDriverName: "sqlite"})
	if err == nil {
		t.Fatalf("expected error")
	}
	_ = store
}

func TestSQLCacheKeyPrefix(t *testing.T) {
	store := &sqlStore{prefix: "p"}
	if got := store.cacheKey("k"); got != "p:k" {
		t.Fatalf("unexpected cache key %s", got)
	}
}

func TestSQLDriverName(t *testing.T) {
	store := NewSQLStore(context.Background(), "sqlite", "file::memory:?cache=shared", "t")
	if store.Driver() != DriverSQL {
		t.Fatalf("expected driver sql")
	}
}

func TestSQLEnsureSchemaPostgresAndMySQL(t *testing.T) {
	if _, err := newSQLStore(StoreConfig{
		SQLDriverName: "pgfake",
		SQLDSN:        "irrelevant",
		SQLTable:      "tbl",
	}); err != nil {
		t.Fatalf("pg schema should succeed: %v", err)
	}
	if _, err := newSQLStore(StoreConfig{
		SQLDriverName: "mysqlfake",
		SQLDSN:        "irrelevant",
		SQLTable:      "tbl",
	}); err != nil {
		t.Fatalf("mysql schema should succeed: %v", err)
	}
	if _, err := newSQLStore(StoreConfig{
		SQLDriverName: "postgres",
		SQLDSN:        "irrelevant",
		SQLTable:      "tbl",
	}); err != nil {
		t.Fatalf("postgres schema should succeed: %v", err)
	}
}

func TestSQLEnsureSchemaError(t *testing.T) {
	if _, err := newSQLStore(StoreConfig{
		SQLDriverName: "pgfail",
		SQLDSN:        "irrelevant",
		SQLTable:      "tbl",
	}); err == nil {
		t.Fatalf("expected schema error")
	}
}

func TestSQLPingError(t *testing.T) {
	if _, err := newSQLStore(StoreConfig{
		SQLDriverName: "pingfail",
		SQLDSN:        "irrelevant",
	}); err == nil {
		t.Fatalf("expected ping error")
	}
}

func TestSQLTableNameValidation(t *testing.T) {
	if err := validateSQLTableName("cache_entries; DROP TABLE users"); err == nil {
		t.Fatalf("expected invalid table name error")
	}

	if err := validateSQLTableName("public.cache_entries"); err != nil {
		t.Fatalf("expected dotted table name to be allowed: %v", err)
	}
}
