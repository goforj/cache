//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache/driver/dynamocache"
)

func main() {
	// Example: custom dynamo table via explicit driver config.
	ctx := context.Background()
	store, err := dynamocache.New(ctx, dynamocache.Config{Table: "cache_entries"})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // dynamodb
}
