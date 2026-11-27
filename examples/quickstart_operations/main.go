package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/goforj/cache"
)

// main shows direct cache operations for a piece of product inventory.
func main() {
	// Create an in-memory cache store.
	c := cache.NewCache(cache.NewMemoryStore(context.Background()))
	const key = "stock:desk-lamp"

	// Cache the stock count for one minute.
	if err := c.Set(key, 12, time.Minute); err != nil {
		log.Fatal(err)
	}

	// Read the count as an int. A cache miss sets found to false.
	stock, found, err := c.Get[int](key)
	if err != nil || !found {
		log.Fatalf("read stock: found=%t, error=%v", found, err)
	}
	fmt.Printf("Cached stock: %d\n", stock)
	// Output: Cached stock: 12

	// Delete the entry and verify the next read misses.
	if err := c.Delete(key); err != nil {
		log.Fatal(err)
	}
	_, found, err = c.Get[int](key)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Cached after delete: %t\n", found)
	// Output: Cached after delete: false
}
