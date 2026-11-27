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
	c := cache.NewCache(cache.NewMemoryStore(context.Background()))
	catalog := map[string]Product{
		"42": {Name: "Desk lamp", PriceCents: 4900},
	}
	const id = "42"

	product, err := c.Remember("product:"+id, time.Minute, func() (Product, error) {
		product, found := catalog[id]
		if !found {
			return Product{}, fmt.Errorf("product %s not found", id)
		}
		return product, nil
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s costs %d cents\n", product.Name, product.PriceCents)
	// Output: Desk lamp costs 4900 cents
}
