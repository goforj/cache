package cache

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"time"
)

var (
	encryptionMagic = []byte("ENC1")

	ErrEncryptionKey      = errors.New("cache: encryption key must be 16, 24, or 32 bytes")
	ErrDecryptFailed      = errors.New("cache: decrypt failed")
	ErrEncryptValueTooBig = errors.New("cache: encrypt value too large")
)

type encryptingStore struct {
	inner Store
	aead  cipher.AEAD
}

func newEncryptingStore(inner Store, key []byte) (Store, error) {
	if len(key) == 0 {
		return inner, nil
	}
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

func (s *encryptingStore) Driver() Driver { return s.inner.Driver() }

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

func (s *encryptingStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	enc, err := s.encrypt(value)
	if err != nil {
		return err
	}
	return s.inner.Set(ctx, key, enc, ttl)
}

func (s *encryptingStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	enc, err := s.encrypt(value)
	if err != nil {
		return false, err
	}
	return s.inner.Add(ctx, key, enc, ttl)
}

func (s *encryptingStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Increment(ctx, key, delta, ttl)
}

func (s *encryptingStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Decrement(ctx, key, delta, ttl)
}

func (s *encryptingStore) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}

func (s *encryptingStore) DeleteMany(ctx context.Context, keys ...string) error {
	return s.inner.DeleteMany(ctx, keys...)
}

func (s *encryptingStore) Flush(ctx context.Context) error {
	return s.inner.Flush(ctx)
}

func (s *encryptingStore) encrypt(plain []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := s.aead.Seal(nil, nonce, plain, nil)
	buf := make([]byte, 0, len(encryptionMagic)+len(nonce)+len(ct))
	buf = append(buf, encryptionMagic...)
	buf = append(buf, byte(len(nonce)))
	buf = append(buf, nonce...)
	buf = append(buf, ct...)
	return buf, nil
}

func (s *encryptingStore) decrypt(in []byte) ([]byte, error) {
	if len(in) < len(encryptionMagic)+1 {
		return in, nil
	}
	if !bytes.Equal(in[:len(encryptionMagic)], encryptionMagic) {
		return in, nil
	}
	nonceLen := int(in[len(encryptionMagic)])
	offset := len(encryptionMagic) + 1
	if len(in) < offset+nonceLen {
		return nil, ErrDecryptFailed
	}
	nonce := in[offset : offset+nonceLen]
	ct := in[offset+nonceLen:]
	plain, err := s.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	return plain, nil
}
