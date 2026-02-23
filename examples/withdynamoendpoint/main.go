//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache/driver/dynamocache"
)

func main() {
	// Example: Dynamo local endpoint via explicit driver config.
	ctx := context.Background()
	store, err := dynamocache.New(ctx, dynamocache.Config{Endpoint: "http://localhost:8000"})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // dynamodb
}
