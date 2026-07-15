package cache

import "github.com/goforj/cache/cachecore"

// CompressionCodec represents a value compression algorithm.
type CompressionCodec = cachecore.CompressionCodec

const (
	// CompressionNone leaves values uncompressed.
	CompressionNone = cachecore.CompressionNone
	// CompressionGzip encodes values with gzip.
	CompressionGzip = cachecore.CompressionGzip
	// CompressionSnappy is reserved for Snappy support and is currently unsupported.
	CompressionSnappy = cachecore.CompressionSnappy
)

var (
	// ErrValueTooLarge reports that a logical or encoded cache value exceeds MaxValueBytes.
	ErrValueTooLarge = cachecore.ErrValueTooLarge
	// ErrInvalidMaxValueBytes reports a negative MaxValueBytes configuration.
	ErrInvalidMaxValueBytes = cachecore.ErrInvalidMaxValueBytes
	// ErrUnsupportedCodec reports a compression codec that this release cannot encode or decode.
	ErrUnsupportedCodec = cachecore.ErrUnsupportedCodec
	// ErrCorruptCompression reports a malformed compressed value envelope.
	ErrCorruptCompression = cachecore.ErrCorruptCompression
	// ErrEncryptionKey reports an AES key whose length is not 16, 24, or 32 bytes.
	ErrEncryptionKey = cachecore.ErrEncryptionKey
	// ErrDecryptFailed reports an invalid or wrong-key encrypted value envelope.
	ErrDecryptFailed = cachecore.ErrDecryptFailed
	// ErrEncryptValueTooBig is retained for compatibility with callers that classify shaping failures.
	ErrEncryptValueTooBig = cachecore.ErrEncryptValueTooBig
)
