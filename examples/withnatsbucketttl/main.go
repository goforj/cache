//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/driver/natscache"
)

func main() {
	// Example: enable NATS bucket-level TTL mode via explicit driver config.
	var kv natscache.KeyValue // provided by your NATS setup
	store := natscache.New(natscache.Config{KeyValue: kv, BucketTTL: true})
	fmt.Println(store.Driver()) // nats
}
