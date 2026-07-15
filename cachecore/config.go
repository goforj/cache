package cachecore

import "time"

// BaseConfig contains shared, backend-agnostic driver configuration.
type BaseConfig struct {
	// DefaultTTL is used when an operation does not provide a positive TTL.
	DefaultTTL time.Duration
	// Prefix namespaces logical keys within a shared backend.
	Prefix string
	// Compression selects value compression when the constructing driver supports shaping.
	Compression CompressionCodec
	// MaxValueBytes limits value size; zero disables the limit and negative values are invalid.
	MaxValueBytes int
	// EncryptionKey enables AES-GCM value encryption when the constructing driver supports shaping.
	EncryptionKey []byte
}
