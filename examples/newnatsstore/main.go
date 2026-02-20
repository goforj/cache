//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// NewNATSStore is a convenience for a NATS JetStream KeyValue-backed store.

	// Example: NATS helper
	ctx := context.Background()
	var kv cache.NATSKeyValue // provided by your NATS setup
	store := cache.NewNATSStore(ctx, kv, cache.WithPrefix("app"))
	fmt.Println(store.Driver()) // nats
}
