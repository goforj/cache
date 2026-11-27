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

	loadProduct := func() (Product, error) {
		return Product{Name: "Desk lamp", PriceCents: 4900}, nil
	}
	product, err := c.Remember("product:42", time.Minute, loadProduct)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s costs %d cents\n", product.Name, product.PriceCents)
	// Output: Desk lamp costs 4900 cents
}
