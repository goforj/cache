//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/rediscache"
)

func main() {
	// Example: prefix keys via explicit Redis config.
	store := rediscache.New(rediscache.Config{
		BaseConfig: cachecore.BaseConfig{Prefix: "svc"},
	})
	fmt.Println(store.Driver()) // redis
}
