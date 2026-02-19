package cache

import (
	"context"
	"time"
)

// shapingStore enforces data shaping concerns (compression, size limits)
// transparently on top of any concrete Store implementation.
type shapingStore struct {
	inner Store
	codec CompressionCodec
	max   int
}

func newShapingStore(inner Store, codec CompressionCodec, max int) Store {
	if codec == CompressionNone && max <= 0 {
		return inner
	}
	return &shapingStore{inner: inner, codec: codec, max: max}
}

func (s *shapingStore) Driver() Driver { return s.inner.Driver() }

func (s *shapingStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	body, ok, err := s.inner.Get(ctx, key)
	if err != nil || !ok {
		return body, ok, err
	}
	decoded, err := decodeValue(body)
	if err != nil {
		return nil, false, err
	}
	return decoded, true, nil
}

func (s *shapingStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	encoded, err := encodeValue(s.codec, s.max, value)
	if err != nil {
		return err
	}
	return s.inner.Set(ctx, key, encoded, ttl)
}

func (s *shapingStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	encoded, err := encodeValue(s.codec, s.max, value)
	if err != nil {
		return false, err
	}
	return s.inner.Add(ctx, key, encoded, ttl)
}

func (s *shapingStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Increment(ctx, key, delta, ttl)
}

func (s *shapingStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.inner.Decrement(ctx, key, delta, ttl)
}

func (s *shapingStore) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}

func (s *shapingStore) DeleteMany(ctx context.Context, keys ...string) error {
	return s.inner.DeleteMany(ctx, keys...)
}

func (s *shapingStore) Flush(ctx context.Context) error {
	return s.inner.Flush(ctx)
}
