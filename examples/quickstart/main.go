package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/goforj/cache"
)

// Product is the catalog value cached in the example.
type Product struct {
	Name       string
	PriceCents int
}

// main caches a product lookup in memory without requiring a backend service.
func main() {
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	const key = "product:42"

	loadProduct := func() (Product, error) {
		return Product{Name: "Desk lamp", PriceCents: 4900}, nil
	}
	product, err := c.Remember(key, time.Minute, loadProduct)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s costs %d cents\n", product.Name, product.PriceCents)
	// Output: Desk lamp costs 4900 cents

	// Read, replace, and invalidate the same cached product directly.
	cached, found, err := c.Get[Product](key)
	if err != nil || !found {
		log.Fatalf("read cached product: found=%t, error=%v", found, err)
	}
	fmt.Printf("Cache hit: %s\n", cached.Name)
	// Output: Cache hit: Desk lamp

	cached.PriceCents = 4500
	if err := c.Set(key, cached, time.Minute); err != nil {
		log.Fatal(err)
	}
	updated, found, err := c.Get[Product](key)
	if err != nil || !found {
		log.Fatalf("read updated product: found=%t, error=%v", found, err)
	}
	fmt.Printf("Updated price: %d cents\n", updated.PriceCents)
	// Output: Updated price: 4500 cents

	if err := c.Delete(key); err != nil {
		log.Fatal(err)
	}
	_, found, err = c.Get[Product](key)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Cached after delete: %t\n", found)
	// Output: Cached after delete: false
}
