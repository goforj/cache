//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithDynamoTable sets the table used by the DynamoDB driver.

	// Example: custom dynamo table
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoTable("cache_entries"))
	fmt.Println(store.Driver()) // dynamodb
}
