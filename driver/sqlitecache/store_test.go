package sqlitecache

import (
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/cachetest"
)

func TestSQLiteStoreContract(t *testing.T) {
	store, err := New(Config{
		BaseConfig: cachecore.BaseConfig{DefaultTTL: time.Second, Prefix: "contract"},
		DSN:        "file::memory:?cache=shared",
		Table:      "cache_entries",
	})
	if err != nil {
		t.Fatalf("sqlite store create failed: %v", err)
	}

	cachetest.RunStoreContract(t, store, cachetest.Options{
		CaseName: t.Name(),
		TTL:      50 * time.Millisecond,
		TTLWait:  120 * time.Millisecond,
	})
}
