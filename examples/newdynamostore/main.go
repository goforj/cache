//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// NewDynamoStore is a convenience for a DynamoDB-backed store.

	// Example: dynamo helper (stub)
	ctx := context.Background()
	store := cache.NewDynamoStore(ctx, cache.StoreConfig{DynamoEndpoint: "http://localhost:8000"})
	fmt.Println(store.Driver()) // dynamodb
}
