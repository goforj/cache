//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/driver/natscache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Example: inject NATS key-value bucket via explicit driver config.
	var kv natscache.KeyValue // provided by your NATS setup
	store := natscache.New(natscache.Config{KeyValue: kv})
	fmt.Println(store.Driver()) // nats
}
