package sqlcore

import (
	"errors"
	"strings"
	"testing"

	"github.com/goforj/cache/cachecore"
)

// TestNewRejectsInvalidShapingBeforeDatabaseSetup verifies configuration errors take precedence over connection setup.
func TestNewRejectsInvalidShapingBeforeDatabaseSetup(t *testing.T) {
	store, err := New(Config{BaseConfig: cachecore.BaseConfig{EncryptionKey: []byte("short")}})
	if !errors.Is(err, cachecore.ErrEncryptionKey) || store != nil {
		t.Fatalf("New = (%v, %v), want nil and ErrEncryptionKey", store, err)
	}
}

// TestSQLStoreHelpersAndBranches verifies statement generation and duplicate classification across supported dialects.
func TestSQLStoreHelpersAndBranches(t *testing.T) {
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
}

// TestSQLCacheKeyPrefix verifies configured namespaces are applied exactly once.
func TestSQLCacheKeyPrefix(t *testing.T) {
	store := &sqlStore{prefix: "p"}
	if got := store.cacheKey("k"); got != "p:k" {
		t.Fatalf("unexpected cache key %s", got)
	}
}

// TestSQLCacheKeyNoPrefix verifies unprefixed SQL stores preserve logical keys.
func TestSQLCacheKeyNoPrefix(t *testing.T) {
	store := &sqlStore{prefix: ""}
	if got := store.cacheKey("k"); got != "k" {
		t.Fatalf("expected raw key, got %s", got)
	}
}

// TestSQLTableNameValidation verifies interpolated table identifiers reject unsafe syntax.
func TestSQLTableNameValidation(t *testing.T) {
	if err := validateSQLTableName("cache_entries; DROP TABLE users"); err == nil {
		t.Fatalf("expected invalid table name error")
	}
	if err := validateSQLTableName("public.cache_entries"); err != nil {
		t.Fatalf("expected dotted table name to be allowed: %v", err)
	}
}
