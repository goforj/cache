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
	c := cache.NewCache(cache.NewMemoryStore(context.Background()))
	const key = "stock:desk-lamp"

	if err := c.Set(key, 12, time.Minute); err != nil {
		log.Fatal(err)
	}

	stock, found, err := c.Get[int](key)
	if err != nil || !found {
		log.Fatalf("read stock: found=%t, error=%v", found, err)
	}
	fmt.Printf("Cached stock: %d\n", stock)
	// Output: Cached stock: 12

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
