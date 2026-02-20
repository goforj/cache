//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithNATSKeyValue sets the NATS JetStream KeyValue bucket; required when using DriverNATS.

	// Example: inject NATS key-value bucket
	ctx := context.Background()
	var kv cache.NATSKeyValue // provided by your NATS setup
	store := cache.NewStoreWith(ctx, cache.DriverNATS, cache.WithNATSKeyValue(kv))
	fmt.Println(store.Driver()) // nats
}
