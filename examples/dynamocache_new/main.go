//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/dynamocache"
	"time"
)

func main() {
	// New builds a DynamoDB-backed cachecore.Store.
	// 
	// Defaults:
	// - Region: "us-east-1" when empty
	// - Table: "cache_entries" when empty
	// - DefaultTTL: 5*time.Minute when zero
	// - Prefix: "app" when empty
	// - Client: auto-created when nil (uses Region and optional Endpoint)
	// - Endpoint: empty by default (normal AWS endpoint resolution)

	// Example: custom dynamo table via explicit driver config
	ctx := context.Background()
	store, err := dynamocache.New(ctx, dynamocache.Config{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL: 5 * time.Minute,
			Prefix:     "app",
		},
		Region: "us-east-1",
		Table:  "cache_entries",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // dynamo
}
