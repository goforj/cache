//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithDynamoClient injects a pre-built DynamoDB client.

	// Example: inject dynamo client
	ctx := context.Background()
	var client cache.DynamoAPI // assume already configured
	store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoClient(client))
	fmt.Println(store.Driver()) // dynamodb
}
