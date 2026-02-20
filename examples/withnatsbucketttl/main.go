//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithNATSBucketTTL toggles bucket-level TTL mode for DriverNATS.
	// When enabled, values are stored as raw bytes and per-operation ttl values are ignored.

	// Example: enable NATS bucket-level TTL mode
	ctx := context.Background()
	var kv cache.NATSKeyValue // provided by your NATS setup
	store := cache.NewStoreWith(ctx, cache.DriverNATS,
		cache.WithNATSKeyValue(kv),
		cache.WithNATSBucketTTL(true),
	)
	fmt.Println(store.Driver()) // nats
}
