//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithDynamoRegion sets the DynamoDB region for requests.

	// Example: set dynamo region
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoRegion("us-west-2"))
	fmt.Println(store.Driver()) // dynamodb
}
