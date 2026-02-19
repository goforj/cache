//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithEncryptionKey enables at-rest encryption using the provided AES key (16/24/32 bytes).

	// Example: encrypt values
	ctx := context.Background()
	key := []byte("01234567890123456789012345678901")
	store := cache.NewStoreWith(ctx, cache.DriverFile, cache.WithEncryptionKey(key))
	fmt.Println(store.Driver()) // file
}
