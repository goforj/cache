package cachecore

import "time"

// BaseConfig contains shared, backend-agnostic driver configuration.
type BaseConfig struct {
	DefaultTTL    time.Duration
	Prefix        string
	Compression   CompressionCodec
	MaxValueBytes int
	EncryptionKey []byte
}
