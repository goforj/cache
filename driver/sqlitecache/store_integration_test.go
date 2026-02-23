//go:build integration

package sqlitecache

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/cachetest"
)

func TestStoreContract_IntegrationSQLite(t *testing.T) {
	if !sqliteIntegrationEnabled() {
		t.Skip("sqlite integration test disabled by INTEGRATION_DRIVER")
	}

	store, err := New(Config{
		BaseConfig: cachecore.BaseConfig{DefaultTTL: 2 * time.Second, Prefix: "itest"},
		DSN:        "file::memory:?cache=shared",
		Table:      "cache_entries",
	})
	if err != nil {
		t.Fatalf("create sqlite store: %v", err)
	}

	cachetest.RunStoreContract(t, store, cachetest.Options{
		CaseName: t.Name(),
		TTL:      50 * time.Millisecond,
		TTLWait:  120 * time.Millisecond,
	})
}

func sqliteIntegrationEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("INTEGRATION_DRIVER")))
	if value == "" || value == "all" {
		return true
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch part {
		case "sql", "sqlcore", "sqlcache", "sql_sqlite", "sqlcache_sqlite", "sqlitecache":
			return true
		}
	}
	return false
}
