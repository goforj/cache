package main

import (
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/natscache"
	"time"
)

func main() {
	// New builds a NATS-backed cachecore.Store.
	// 
	// Defaults:
	// - DefaultTTL: 5*time.Minute when zero
	// - Prefix: "app" when empty
	// - BucketTTL: false (TTL enforced in value envelope metadata)
	// - KeyValue: required for real operations (nil allowed, operations return errors)

	// Example: inject NATS key-value bucket via explicit driver config
	var kv natscache.KeyValue // provided by your NATS setup
	store := natscache.New(natscache.Config{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL: 5 * time.Minute,
			Prefix:     "app",
		},
		KeyValue:  kv,
		BucketTTL: false,
	})
	fmt.Println(store.Driver()) // nats
}
