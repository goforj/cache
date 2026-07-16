package cache

import (
	"os"
	"path/filepath"
	"time"

	"github.com/goforj/cache/cachecore"
)

const (
	defaultCachePrefix           = "app"
	defaultCacheTTL              = 5 * time.Minute
	defaultMemoryCleanupInterval = 10 * time.Minute
)

// defaultFileDir isolates the convenience file store under the operating system temporary directory.
func defaultFileDir() string {
	return filepath.Join(os.TempDir(), "cache-file")
}

// StoreConfig controls shared/root store construction settings.
type StoreConfig struct {
	cachecore.BaseConfig

	// MemoryCleanupInterval controls in-process cache eviction.
	MemoryCleanupInterval time.Duration

	// FileDir controls where file driver stores cache entries.
	FileDir string
}

// withDefaults returns a normalized copy so caller-owned configuration is never mutated.
func (c StoreConfig) withDefaults() StoreConfig {
	if c.DefaultTTL <= 0 {
		c.DefaultTTL = defaultCacheTTL
	}
	if c.MemoryCleanupInterval <= 0 {
		c.MemoryCleanupInterval = defaultMemoryCleanupInterval
	}
	if c.Prefix == "" {
		c.Prefix = defaultCachePrefix
	}
	if c.FileDir == "" {
		c.FileDir = defaultFileDir()
	}
	if c.Compression == "" {
		c.Compression = CompressionNone
	}
	return c
}
