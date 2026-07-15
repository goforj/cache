package cachecore

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"time"
)

var (
	// ErrValueTooLarge reports that a logical or encoded cache value exceeds MaxValueBytes.
	ErrValueTooLarge = errors.New("cache: value exceeds max size")
	// ErrInvalidMaxValueBytes reports a negative MaxValueBytes configuration.
	ErrInvalidMaxValueBytes = errors.New("cache: max value bytes must not be negative")
	// ErrUnsupportedCodec reports a compression codec that this release cannot encode or decode.
	ErrUnsupportedCodec = errors.New("cache: unsupported compression codec")
	// ErrCorruptCompression reports a malformed compressed value envelope.
	ErrCorruptCompression = errors.New("cache: corrupt compressed payload")
	// ErrEncryptionKey reports an AES key whose length is not 16, 24, or 32 bytes.
	ErrEncryptionKey = errors.New("cache: encryption key must be 16, 24, or 32 bytes")
	// ErrDecryptFailed reports an invalid or wrong-key encrypted value envelope.
	ErrDecryptFailed = errors.New("cache: decrypt failed")
	// ErrEncryptValueTooBig is retained for compatibility with callers that classify shaping failures.
	ErrEncryptValueTooBig = errors.New("cache: encrypt value too large")
)

// compressionMagic distinguishes shaped gzip values from legacy raw bytes.
var compressionMagic = []byte("CMP1")

// encryptionMagic distinguishes shaped AES-GCM values from legacy raw bytes.
var encryptionMagic = []byte("ENC1")

// WrapStore applies BaseConfig value shaping to store while preserving atomic counter operations.
//
// Existing unmarked values remain readable. When both encryption and compression are enabled,
// the persisted format preserves the original cache package order: an ENC1 encryption envelope
// wrapped by a CMP1 compression envelope. A configuration error returns both an error and a
// driver-preserving Store whose operations report that error, allowing constructors without an
// error return to remain fail-closed.
func WrapStore(store Store, cfg BaseConfig) (Store, error) {
	if store == nil {
		panic("cachecore: store is required")
	}
	if err := ValidateBaseConfig(cfg); err != nil {
		return &failingStore{driver: store.Driver(), err: err}, err
	}
	codec := cfg.Compression
	if codec == "" {
		codec = CompressionNone
	}

	wrapped := store
	if codec != CompressionNone || cfg.MaxValueBytes > 0 {
		wrapped = &shapingStore{inner: wrapped, codec: codec, max: cfg.MaxValueBytes}
	}
	if len(cfg.EncryptionKey) > 0 {
		encrypted, err := newEncryptingStore(wrapped, cfg.EncryptionKey)
		if err != nil {
			return &failingStore{driver: store.Driver(), err: err}, err
		}
		wrapped = encrypted
	}
	return wrapped, nil
}

// ValidateBaseConfig checks shaping settings without constructing or contacting a backend.
func ValidateBaseConfig(cfg BaseConfig) error {
	if cfg.MaxValueBytes < 0 {
		return ErrInvalidMaxValueBytes
	}
	codec := cfg.Compression
	if codec != "" && codec != CompressionNone && codec != CompressionGzip {
		return ErrUnsupportedCodec
	}
	if size := len(cfg.EncryptionKey); size != 0 && size != 16 && size != 24 && size != 32 {
		return ErrEncryptionKey
	}
	return nil
}

// shapingStore applies compression and size limits without changing backend semantics.
type shapingStore struct {
	inner Store
	codec CompressionCodec
	max   int
}

// Driver reports the wrapped backend driver.
func (s *shapingStore) Driver() Driver { return s.inner.Driver() }

// Ready delegates readiness to the wrapped backend.
func (s *shapingStore) Ready(ctx context.Context) error { return s.inner.Ready(ctx) }

// Get decodes a marked compression envelope while allowing legacy raw values through.
func (s *shapingStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	body, ok, err := s.inner.Get(ctx, key)
	if err != nil || !ok {
		return body, ok, err
	}
	decoded, err := decodeValue(body, s.max)
	if err != nil {
		return nil, false, err
	}
	return decoded, true, nil
}

// Set encodes and validates a value before delegating the write.
func (s *shapingStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	encoded, err := encodeValue(s.codec, s.max, value)
	if err != nil {
		return err
	}
	return s.inner.Set(ctx, key, encoded, ttl)
}

// Add encodes and validates a value before delegating the conditional write.
func (s *shapingStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	encoded, err := encodeValue(s.codec, s.max, value)
	if err != nil {
		return false, err
	}
	return s.inner.Add(ctx, key, encoded, ttl)
}

// Increment bypasses shaping so backend-native numeric operations remain atomic.
func (s *shapingStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Increment(ctx, key, delta, ttl)
}

// Decrement bypasses shaping so backend-native numeric operations remain atomic.
func (s *shapingStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Decrement(ctx, key, delta, ttl)
}

// Delete delegates deletion without transforming the key.
func (s *shapingStore) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}

// DeleteMany delegates batch deletion without transforming keys.
func (s *shapingStore) DeleteMany(ctx context.Context, keys ...string) error {
	return s.inner.DeleteMany(ctx, keys...)
}

// Flush delegates scoped deletion to the wrapped backend.
func (s *shapingStore) Flush(ctx context.Context) error { return s.inner.Flush(ctx) }

// Capabilities preserves optional inspector metadata from the wrapped backend.
func (s *shapingStore) Capabilities() InspectorCapabilities {
	inspector, ok := s.inner.(Inspector)
	if !ok {
		return InspectorCapabilities{}
	}
	return inspector.Capabilities()
}

// ListPage delegates inspection because shaping changes values, not key metadata.
func (s *shapingStore) ListPage(ctx context.Context, opts ListPageOptions) (ListPageResult, error) {
	inspector, ok := s.inner.(Inspector)
	if !ok {
		return ListPageResult{}, ErrInspectorUnsupported
	}
	return inspector.ListPage(ctx, opts)
}

// encryptingStore applies AES-GCM envelopes while leaving backend counters raw.
type encryptingStore struct {
	inner Store
	aead  cipher.AEAD
}

// newEncryptingStore validates key and constructs the AES-GCM wrapper.
func newEncryptingStore(inner Store, key []byte) (Store, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrEncryptionKey
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &encryptingStore{inner: inner, aead: aead}, nil
}

// Driver reports the wrapped backend driver.
func (s *encryptingStore) Driver() Driver { return s.inner.Driver() }

// Ready delegates readiness to the wrapped backend.
func (s *encryptingStore) Ready(ctx context.Context) error { return s.inner.Ready(ctx) }

// Get decrypts a marked encryption envelope while allowing legacy raw values through.
func (s *encryptingStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	body, ok, err := s.inner.Get(ctx, key)
	if err != nil || !ok {
		return body, ok, err
	}
	plain, err := s.decrypt(body)
	if err != nil {
		return nil, false, err
	}
	return plain, true, nil
}

// Set encrypts a value before delegating the write.
func (s *encryptingStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	encoded, err := s.encrypt(value)
	if err != nil {
		return err
	}
	return s.inner.Set(ctx, key, encoded, ttl)
}

// Add encrypts a value before delegating the conditional write.
func (s *encryptingStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	encoded, err := s.encrypt(value)
	if err != nil {
		return false, err
	}
	return s.inner.Add(ctx, key, encoded, ttl)
}

// Increment bypasses encryption so backend-native numeric operations remain atomic.
func (s *encryptingStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Increment(ctx, key, delta, ttl)
}

// Decrement bypasses encryption so backend-native numeric operations remain atomic.
func (s *encryptingStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Decrement(ctx, key, delta, ttl)
}

// Delete delegates deletion without transforming the key.
func (s *encryptingStore) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}

// DeleteMany delegates batch deletion without transforming keys.
func (s *encryptingStore) DeleteMany(ctx context.Context, keys ...string) error {
	return s.inner.DeleteMany(ctx, keys...)
}

// Flush delegates scoped deletion to the wrapped backend.
func (s *encryptingStore) Flush(ctx context.Context) error { return s.inner.Flush(ctx) }

// Capabilities preserves optional inspector metadata from the wrapped backend.
func (s *encryptingStore) Capabilities() InspectorCapabilities {
	inspector, ok := s.inner.(Inspector)
	if !ok {
		return InspectorCapabilities{}
	}
	return inspector.Capabilities()
}

// ListPage delegates inspection because encryption changes values, not key metadata.
func (s *encryptingStore) ListPage(ctx context.Context, opts ListPageOptions) (ListPageResult, error) {
	inspector, ok := s.inner.(Inspector)
	if !ok {
		return ListPageResult{}, ErrInspectorUnsupported
	}
	return inspector.ListPage(ctx, opts)
}

// encrypt creates a versioned AES-GCM envelope with a fresh nonce.
func (s *encryptingStore) encrypt(plain []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := s.aead.Seal(nil, nonce, plain, nil)
	body := make([]byte, 0, len(encryptionMagic)+1+len(nonce)+len(ciphertext))
	body = append(body, encryptionMagic...)
	body = append(body, byte(len(nonce)))
	body = append(body, nonce...)
	body = append(body, ciphertext...)
	return body, nil
}

// decrypt opens a marked AES-GCM envelope and passes unmarked legacy bytes through unchanged.
func (s *encryptingStore) decrypt(body []byte) ([]byte, error) {
	if len(body) < len(encryptionMagic)+1 || !bytes.Equal(body[:len(encryptionMagic)], encryptionMagic) {
		return body, nil
	}
	nonceLen := int(body[len(encryptionMagic)])
	offset := len(encryptionMagic) + 1
	if nonceLen != s.aead.NonceSize() || nonceLen > len(body)-offset {
		return nil, ErrDecryptFailed
	}
	nonce := body[offset : offset+nonceLen]
	ciphertext := body[offset+nonceLen:]
	plain, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	return plain, nil
}

// encodeValue applies the configured size policy and compression envelope.
func encodeValue(codec CompressionCodec, max int, value []byte) ([]byte, error) {
	if max > 0 && len(value) > max {
		return nil, ErrValueTooLarge
	}
	switch codec {
	case CompressionNone:
		return value, nil
	case CompressionGzip:
		var buffer bytes.Buffer
		buffer.Write(compressionMagic)
		_ = buffer.WriteByte('g')
		writer, err := gzip.NewWriterLevel(&buffer, gzip.BestSpeed)
		if err != nil {
			return nil, err
		}
		if _, err := writer.Write(value); err != nil {
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		encoded := buffer.Bytes()
		if max > 0 && len(encoded) > max {
			return nil, ErrValueTooLarge
		}
		return encoded, nil
	default:
		return nil, ErrUnsupportedCodec
	}
}

// decodeValue decodes known compression envelopes without expanding beyond the configured limit.
func decodeValue(body []byte, max int) ([]byte, error) {
	if max > 0 && len(body) > max {
		return nil, ErrValueTooLarge
	}
	if len(body) < len(compressionMagic)+1 || !bytes.Equal(body[:len(compressionMagic)], compressionMagic) {
		return body, nil
	}
	payload := body[len(compressionMagic)+1:]
	switch body[len(compressionMagic)] {
	case 'g':
		reader, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, ErrCorruptCompression
		}
		defer reader.Close()
		var source io.Reader = reader
		if max > 0 {
			limit := int64(max)
			if limit < int64(^uint64(0)>>1) {
				limit++
			}
			source = io.LimitReader(reader, limit)
		}
		decoded, err := io.ReadAll(source)
		if err != nil {
			return nil, ErrCorruptCompression
		}
		if max > 0 && len(decoded) > max {
			return nil, ErrValueTooLarge
		}
		return decoded, nil
	default:
		return nil, ErrUnsupportedCodec
	}
}

// failingStore preserves driver identity while failing closed on invalid shaping configuration.
type failingStore struct {
	driver Driver
	err    error
}

// Driver reports the configured backend even when shaping configuration is invalid.
func (s *failingStore) Driver() Driver { return s.driver }

// Ready reports the shaping configuration error.
func (s *failingStore) Ready(context.Context) error { return s.err }

// Get reports the shaping configuration error.
func (s *failingStore) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, s.err
}

// Set reports the shaping configuration error.
func (s *failingStore) Set(context.Context, string, []byte, time.Duration) error { return s.err }

// Add reports the shaping configuration error.
func (s *failingStore) Add(context.Context, string, []byte, time.Duration) (bool, error) {
	return false, s.err
}

// Increment reports the shaping configuration error.
func (s *failingStore) Increment(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, s.err
}

// Decrement reports the shaping configuration error.
func (s *failingStore) Decrement(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, s.err
}

// Delete reports the shaping configuration error.
func (s *failingStore) Delete(context.Context, string) error { return s.err }

// DeleteMany reports the shaping configuration error.
func (s *failingStore) DeleteMany(context.Context, ...string) error { return s.err }

// Flush reports the shaping configuration error.
func (s *failingStore) Flush(context.Context) error { return s.err }
