package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/goforj/cache"
)

// main loads and caches a price in cents.
func main() {
	// Create an in-memory cache store.
	c := cache.NewCache(cache.NewMemoryStore(context.Background()))

	// Load the price on a cache miss and keep it for one minute.
	price, err := c.Remember("product:42:price", time.Minute, func() (int, error) {
		return strconv.Atoi("4900")
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Desk lamp costs %d cents\n", price)
	// Output: Desk lamp costs 4900 cents
}
