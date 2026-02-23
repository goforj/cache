package cachecore

// CompressionCodec represents a value compression algorithm.
type CompressionCodec string

const (
	CompressionNone   CompressionCodec = "none"
	CompressionGzip   CompressionCodec = "gzip"
	CompressionSnappy CompressionCodec = "snappy"
)
