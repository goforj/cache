//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache/driver/dynamocache"
)

func main() {
	// Example: inject dynamo client via explicit driver config.
	ctx := context.Background()
	var client dynamocache.DynamoAPI // assume already configured
	store, err := dynamocache.New(ctx, dynamocache.Config{Client: client})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // dynamodb
}
