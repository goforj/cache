package sqlcore

import (
	"testing"

	"github.com/goforj/cache/cachecore"
)

func TestSQLDriverErrorsWhenMissingDSN(t *testing.T) {
	store, err := New(Config{DriverName: "pgfake"})
	if err == nil {
		t.Fatalf("expected error")
	}
	_ = store
}

func TestSQLDriverName(t *testing.T) {
	store, err := New(Config{DriverName: "pgfake", DSN: "irrelevant", Table: "t"})
	if err != nil {
		t.Fatalf("create sql store: %v", err)
	}
	if store.Driver() != cachecore.DriverSQL {
		t.Fatalf("expected driver sql")
	}
}

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

func TestSQLEnsureSchemaError(t *testing.T) {
	if _, err := New(Config{
		DriverName: "pgfail",
		DSN:        "irrelevant",
		Table:      "tbl",
	}); err == nil {
		t.Fatalf("expected schema error")
	}
}

func TestSQLPingError(t *testing.T) {
	if _, err := New(Config{
		DriverName: "pingfail",
		DSN:        "irrelevant",
	}); err == nil {
		t.Fatalf("expected ping error")
	}
}
