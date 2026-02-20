package cache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const natsEnvelopeMarker = "cache-v1"

// NATSKeyValue captures the subset of nats.KeyValue used by the store.
type NATSKeyValue interface {
	Get(key string) (nats.KeyValueEntry, error)
	Put(key string, value []byte) (uint64, error)
	Create(key string, value []byte) (uint64, error)
	Update(key string, value []byte, last uint64) (uint64, error)
	Delete(key string, opts ...nats.DeleteOpt) error
	Purge(key string, opts ...nats.DeleteOpt) error
	ListKeys(opts ...nats.WatchOpt) (nats.KeyLister, error)
}

type natsStore struct {
	kv         NATSKeyValue
	defaultTTL time.Duration
	prefix     string
	bucketTTL  bool
}

type natsEnvelope struct {
	Marker    string `json:"m"`
	Value     []byte `json:"v"`
	ExpiresAt int64  `json:"ea"`
}

func newNATSStore(kv NATSKeyValue, defaultTTL time.Duration, prefix string, bucketTTL bool) Store {
	if defaultTTL <= 0 {
		defaultTTL = defaultCacheTTL
	}
	if prefix == "" {
		prefix = defaultCachePrefix
	}
	return &natsStore{
		kv:         kv,
		defaultTTL: defaultTTL,
		prefix:     prefix,
		bucketTTL:  bucketTTL,
	}
}

func (s *natsStore) Driver() Driver { return DriverNATS }

func (s *natsStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	if s.kv == nil {
		return nil, false, errors.New("nats cache key-value unavailable")
	}
	cacheKey := s.cacheKey(key)
	entry, err := s.kv.Get(cacheKey)
	if isNATSMiss(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if entry.Operation() == nats.KeyValueDelete || entry.Operation() == nats.KeyValuePurge {
		return nil, false, nil
	}
	if s.bucketTTL {
		return cloneBytes(entry.Value()), true, nil
	}
	envelope, wrapped, err := decodeNATSEnvelope(entry.Value())
	if err != nil {
		return nil, false, err
	}
	if wrapped {
		if envelope.ExpiresAt > 0 && time.Now().UnixMilli() > envelope.ExpiresAt {
			_ = s.kv.Purge(cacheKey)
			return nil, false, nil
		}
		return cloneBytes(envelope.Value), true, nil
	}
	return cloneBytes(entry.Value()), true, nil
}

func (s *natsStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if s.kv == nil {
		return errors.New("nats cache key-value unavailable")
	}
	body := cloneBytes(value)
	if !s.bucketTTL {
		var err error
		body, err = s.encodeNATSEnvelope(value, ttl)
		if err != nil {
			return err
		}
	}
	_, err := s.kv.Put(s.cacheKey(key), body)
	return err
}

func (s *natsStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if s.kv == nil {
		return false, errors.New("nats cache key-value unavailable")
	}
	_, ok, err := s.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}
	body := cloneBytes(value)
	if !s.bucketTTL {
		var err error
		body, err = s.encodeNATSEnvelope(value, ttl)
		if err != nil {
			return false, err
		}
	}
	_, err = s.kv.Create(s.cacheKey(key), body)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, nats.ErrKeyExists) {
		return false, nil
	}
	return false, err
}

func (s *natsStore) Increment(_ context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if s.kv == nil {
		return 0, errors.New("nats cache key-value unavailable")
	}
	cacheKey := s.cacheKey(key)
	for attempt := 0; attempt < 16; attempt++ {
		var (
			current  int64
			revision uint64
		)

		entry, err := s.kv.Get(cacheKey)
		if err != nil {
			if !isNATSMiss(err) {
				return 0, err
			}
		} else {
			if entry.Operation() == nats.KeyValueDelete || entry.Operation() == nats.KeyValuePurge {
				revision = 0
			} else {
				raw := entry.Value()
				if !s.bucketTTL {
					envelope, wrapped, decodeErr := decodeNATSEnvelope(entry.Value())
					if decodeErr != nil {
						return 0, decodeErr
					}
					if wrapped {
						if envelope.ExpiresAt > 0 && time.Now().UnixMilli() > envelope.ExpiresAt {
							_ = s.kv.Purge(cacheKey)
							revision = 0
							raw = nil
						} else {
							raw = envelope.Value
							revision = entry.Revision()
						}
					} else {
						revision = entry.Revision()
					}
				} else {
					revision = entry.Revision()
				}
				if len(raw) > 0 {
					parsed, parseErr := strconv.ParseInt(string(raw), 10, 64)
					if parseErr != nil {
						return 0, fmt.Errorf("cache key %q does not contain a numeric value", key)
					}
					current = parsed
				}
			}
		}

		next := current + delta
		body := []byte(strconv.FormatInt(next, 10))
		if !s.bucketTTL {
			var err error
			body, err = s.encodeNATSEnvelope(body, ttl)
			if err != nil {
				return 0, err
			}
		}
		if revision == 0 {
			_, err = s.kv.Create(cacheKey, body)
			if err == nil {
				return next, nil
			}
			if errors.Is(err, nats.ErrKeyExists) {
				continue
			}
			return 0, err
		}
		_, err = s.kv.Update(cacheKey, body, revision)
		if err == nil {
			return next, nil
		}
		if errors.Is(err, nats.ErrKeyExists) || isNATSMiss(err) {
			continue
		}
		return 0, err
	}
	return 0, errors.New("nats increment exceeded retry limit")
}

func (s *natsStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

func (s *natsStore) Delete(_ context.Context, key string) error {
	if s.kv == nil {
		return errors.New("nats cache key-value unavailable")
	}
	err := s.kv.Delete(s.cacheKey(key))
	if isNATSMiss(err) {
		return nil
	}
	return err
}

func (s *natsStore) DeleteMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *natsStore) Flush(_ context.Context) error {
	if s.kv == nil {
		return errors.New("nats cache key-value unavailable")
	}
	lister, err := s.kv.ListKeys(nats.IgnoreDeletes())
	if err != nil {
		if errors.Is(err, nats.ErrNoKeysFound) {
			return nil
		}
		return err
	}
	defer func() { _ = lister.Stop() }()

	scopePrefix := s.scopePrefix()
	for key := range lister.Keys() {
		if !strings.HasPrefix(key, scopePrefix) {
			continue
		}
		if err := s.kv.Purge(key); err != nil && !isNATSMiss(err) {
			return err
		}
	}
	for err := range lister.Error() {
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *natsStore) cacheKey(key string) string {
	return s.scopePrefix() + encodeNATSKeyPart(key)
}

func (s *natsStore) scopePrefix() string {
	return "p." + encodeNATSKeyPart(s.prefix) + ".k."
}

func (s *natsStore) encodeNATSEnvelope(value []byte, ttl time.Duration) ([]byte, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	envelope := natsEnvelope{
		Marker:    natsEnvelopeMarker,
		Value:     cloneBytes(value),
		ExpiresAt: time.Now().Add(ttl).UnixMilli(),
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal nats cache envelope: %w", err)
	}
	return body, nil
}

func decodeNATSEnvelope(body []byte) (natsEnvelope, bool, error) {
	var envelope natsEnvelope
	if len(body) == 0 || body[0] != '{' {
		return envelope, false, nil
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return natsEnvelope{}, false, fmt.Errorf("decode nats cache envelope: %w", err)
	}
	if envelope.Marker != natsEnvelopeMarker {
		return natsEnvelope{}, false, nil
	}
	return envelope, true, nil
}

func isNATSMiss(err error) bool {
	return errors.Is(err, nats.ErrKeyNotFound) || errors.Is(err, nats.ErrKeyDeleted)
}

func encodeNATSKeyPart(part string) string {
	if part == "" {
		return "_"
	}
	return base64.RawURLEncoding.EncodeToString([]byte(part))
}
