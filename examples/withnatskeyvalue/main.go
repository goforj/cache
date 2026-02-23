//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/driver/natscache"
)

func main() {
	// Example: inject NATS key-value bucket via explicit driver config.
	var kv natscache.KeyValue // provided by your NATS setup
	store := natscache.New(natscache.Config{KeyValue: kv})
	fmt.Println(store.Driver()) // nats
}
