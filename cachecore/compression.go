package cachecore

// CompressionCodec represents a value compression algorithm.
type CompressionCodec string

const (
	// CompressionNone leaves values uncompressed.
	CompressionNone CompressionCodec = "none"
	// CompressionGzip encodes values with gzip.
	CompressionGzip CompressionCodec = "gzip"
	// CompressionSnappy is reserved for Snappy support and is currently unsupported.
	CompressionSnappy CompressionCodec = "snappy"
)
