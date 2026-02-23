//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
)

func main() {
	// Example: encrypt values via explicit StoreConfig.
	ctx := context.Background()
	key := []byte("01234567890123456789012345678901")
	store := cache.NewFileStoreWithConfig(ctx, cache.StoreConfig{
		BaseConfig: cachecore.BaseConfig{EncryptionKey: key},
	})
	fmt.Println(store.Driver()) // file
}
