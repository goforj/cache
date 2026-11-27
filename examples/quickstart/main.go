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
	catalog := map[int]Product{
		42: {Name: "Desk lamp", PriceCents: 4900},
	}

	rememberProduct := func(id int) (Product, error) {
		key := fmt.Sprintf("product:%d", id)
		return c.Remember(key, time.Minute, func() (Product, error) {
			product, found := catalog[id]
			if !found {
				return Product{}, fmt.Errorf("product %d not found", id)
			}
			return product, nil
		})
	}

	product, err := rememberProduct(42)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s costs %d cents\n", product.Name, product.PriceCents)
	// Output: Desk lamp costs 4900 cents

	_, err = rememberProduct(99)
	fmt.Println(err)
	// Output: product 99 not found
}
