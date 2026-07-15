package sqlcore

import (
	"context"
	"testing"

	"github.com/goforj/cache/cachecore"
)

// TestSQLDriverErrorsWhenMissingDSN verifies construction rejects an unusable SQL configuration.
func TestSQLDriverErrorsWhenMissingDSN(t *testing.T) {
	store, err := New(Config{DriverName: "pgfake"})
	if err == nil {
		t.Fatalf("expected error")
	}
	_ = store
}

// TestSQLDriverName verifies SQL-backed stores report the shared SQL driver identity.
func TestSQLDriverName(t *testing.T) {
	store, err := New(Config{DriverName: "pgfake", DSN: "irrelevant", Table: "t"})
	if err != nil {
		t.Fatalf("create sql store: %v", err)
	}
	if store.Driver() != cachecore.DriverSQL {
		t.Fatalf("expected driver sql")
	}
	if err := store.Ready(context.Background()); err != nil {
		t.Fatalf("expected ready nil, got %v", err)
	}
}

// TestSQLEnsureSchemaPostgresAndMySQL verifies schema creation uses dialect-appropriate definitions.
func TestSQLEnsureSchemaPostgresAndMySQL(t *testing.T) {
	if _, err := New(Config{
		DriverName: "pgfake",
		DSN:        "irrelevant",
		Table:      "tbl",
	}); err != nil {
		t.Fatalf("pg schema should succeed: %v", err)
	}
	if _, err := New(Config{
		DriverName: "mysqlfake",
		DSN:        "irrelevant",
		Table:      "tbl",
	}); err != nil {
		t.Fatalf("mysql schema should succeed: %v", err)
	}
	if _, err := New(Config{
		DriverName: "postgres",
		DSN:        "irrelevant",
		Table:      "tbl",
	}); err != nil {
		t.Fatalf("postgres schema should succeed: %v", err)
	}
}

// TestSQLEnsureSchemaError verifies initialization preserves DDL failures.
func TestSQLEnsureSchemaError(t *testing.T) {
	if _, err := New(Config{
		DriverName: "pgfail",
		DSN:        "irrelevant",
		Table:      "tbl",
	}); err == nil {
		t.Fatalf("expected schema error")
	}
}

// TestSQLPingError verifies readiness preserves database connectivity failures.
func TestSQLPingError(t *testing.T) {
	if _, err := New(Config{
		DriverName: "pingfail",
		DSN:        "irrelevant",
	}); err == nil {
		t.Fatalf("expected ping error")
	}
}
