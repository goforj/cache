package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
)

func main() {
	// NewFileStoreWithConfig builds a filesystem-backed store using explicit root config.

	// Example: file helper with root config
	ctx := context.Background()
	store := cache.NewFileStoreWithConfig(ctx, cache.StoreConfig{
		BaseConfig: cachecore.BaseConfig{
			EncryptionKey: []byte("01234567890123456789012345678901"),
			MaxValueBytes: 4096,
			Compression:   cache.CompressionGzip,
		},
		FileDir: "/tmp/my-cache",
	})
	fmt.Println(store.Driver()) // file
}
