//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache/driver/dynamocache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Example: set dynamo region via explicit driver config.
	ctx := context.Background()
	store, err := dynamocache.New(ctx, dynamocache.Config{Region: "us-west-2"})
	if err != nil {
		panic(err)
	}
	fmt.Println(store.Driver()) // dynamodb
}
