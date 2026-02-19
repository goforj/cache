//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithDynamoEndpoint sets the DynamoDB endpoint (useful for local testing).

	// Example: dynamo local endpoint
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoEndpoint("http://localhost:8000"))
	fmt.Println(store.Driver()) // dynamodb
}
