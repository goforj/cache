//go:build integration

package cache

import "github.com/goforj/cache/cachecore"

// IntegrationWrapShapingStore exposes the shaping wrapper to integration-module tests
// without making it part of the normal root package API surface.
func IntegrationWrapShapingStore(inner cachecore.Store, codec CompressionCodec, max int) cachecore.Store {
	store, _ := cachecore.WrapStore(inner, cachecore.BaseConfig{Compression: codec, MaxValueBytes: max})
	return store
}

// IntegrationWrapEncryptingStore exposes the encryption wrapper to integration-module tests
// without making it part of the normal root package API surface.
func IntegrationWrapEncryptingStore(inner cachecore.Store, key []byte) (cachecore.Store, error) {
	return cachecore.WrapStore(inner, cachecore.BaseConfig{EncryptionKey: key})
}
