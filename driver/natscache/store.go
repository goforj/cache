package natscache

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/nats-io/nats.go"
)

const (
	natsEnvelopeMarker = "cache-v1"
	defaultTTL         = 5 * time.Minute
	defaultPrefix      = "app"
)

var natsEnvelopeMagic = []byte("NCV1")

// KeyValue captures the subset of nats.KeyValue used by the store.
type KeyValue interface {
	Get(key string) (nats.KeyValueEntry, error)
	Put(key string, value []byte) (uint64, error)
	Create(key string, value []byte) (uint64, error)
	Update(key string, value []byte, last uint64) (uint64, error)
	Delete(key string, opts ...nats.DeleteOpt) error
	Purge(key string, opts ...nats.DeleteOpt) error
	ListKeys(opts ...nats.WatchOpt) (nats.KeyLister, error)
}

// Config configures a NATS JetStream KeyValue-backed cache store.
type Config struct {
	cachecore.BaseConfig
	KeyValue  KeyValue
	BucketTTL bool
}

type store struct {
	kv             KeyValue
	defaultTTL     time.Duration
	prefix         string
	scopePrefixStr string
	bucketTTL      bool
}

type envelope struct {
	Marker    string `json:"m"`
	Value     []byte `json:"v"`
	ExpiresAt int64  `json:"ea"`
}

// New builds a NATS-backed cachecore.Store.
//
// Defaults:
// - DefaultTTL: 5*time.Minute when zero
// - Prefix: "app" when empty
// - BucketTTL: false (TTL enforced in value envelope metadata)
// - KeyValue: required for real operations (nil allowed, operations return errors)
//
// Example: inject NATS key-value bucket via explicit driver config
//
//	var kv natscache.KeyValue // provided by your NATS setup
//	store := natscache.New(natscache.Config{
//		BaseConfig: cachecore.BaseConfig{
//			DefaultTTL: 5 * time.Minute,
//			Prefix:     "app",
//		},
//		KeyValue:  kv,
//		BucketTTL: false,
//	})
//	fmt.Println(store.Driver()) // nats
func New(cfg Config) cachecore.Store {
	ttl := cfg.DefaultTTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = defaultPrefix
	}
	return &store{
		kv:             cfg.KeyValue,
		defaultTTL:     ttl,
		prefix:         prefix,
		scopePrefixStr: "p." + encodeKeyPart(prefix) + ".k.",
		bucketTTL:      cfg.BucketTTL,
	}
}

func (s *store) Driver() cachecore.Driver { return cachecore.DriverNATS }

func (s *store) Get(_ context.Context, key string) ([]byte, bool, error) {
	if s.kv == nil {
		return nil, false, errors.New("nats cache key-value unavailable")
	}
	cacheKey := s.cacheKey(key)
	entry, err := s.kv.Get(cacheKey)
	if isMiss(err) {
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
	env, wrapped, err := decodeEnvelope(entry.Value())
	if err != nil {
		return nil, false, err
	}
	if wrapped {
		if env.ExpiresAt > 0 && time.Now().UnixMilli() > env.ExpiresAt {
			_ = s.kv.Purge(cacheKey)
			return nil, false, nil
		}
		return cloneBytes(env.Value), true, nil
	}
	return cloneBytes(entry.Value()), true, nil
}

func (s *store) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if s.kv == nil {
		return errors.New("nats cache key-value unavailable")
	}
	var (
		body []byte
		err  error
	)
	if s.bucketTTL {
		body = cloneBytes(value)
	} else {
		body, err = s.encodeEnvelope(value, ttl)
		if err != nil {
			return err
		}
	}
	_, err = s.kv.Put(s.cacheKey(key), body)
	return err
}

func (s *store) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
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
	var body []byte
	if s.bucketTTL {
		body = cloneBytes(value)
	} else {
		body, err = s.encodeEnvelope(value, ttl)
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

func (s *store) Increment(_ context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if s.kv == nil {
		return 0, errors.New("nats cache key-value unavailable")
	}
	cacheKey := s.cacheKey(key)
	for attempt := 0; attempt < 16; attempt++ {
		var current int64
		var revision uint64

		entry, err := s.kv.Get(cacheKey)
		if err != nil {
			if !isMiss(err) {
				return 0, err
			}
		} else {
			if entry.Operation() == nats.KeyValueDelete || entry.Operation() == nats.KeyValuePurge {
				revision = 0
			} else {
				raw := entry.Value()
				if !s.bucketTTL {
					env, wrapped, decodeErr := decodeEnvelope(raw)
					if decodeErr != nil {
						return 0, decodeErr
					}
					if wrapped {
						if env.ExpiresAt > 0 && time.Now().UnixMilli() > env.ExpiresAt {
							_ = s.kv.Purge(cacheKey)
							revision = 0
							raw = nil
						} else {
							raw = env.Value
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
			body, err = s.encodeEnvelope(body, ttl)
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
		if errors.Is(err, nats.ErrKeyExists) || isMiss(err) {
			continue
		}
		return 0, err
	}
	return 0, errors.New("nats increment exceeded retry limit")
}

func (s *store) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

func (s *store) Delete(_ context.Context, key string) error {
	if s.kv == nil {
		return errors.New("nats cache key-value unavailable")
	}
	err := s.kv.Delete(s.cacheKey(key))
	if isMiss(err) {
		return nil
	}
	return err
}

func (s *store) DeleteMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) Flush(_ context.Context) error {
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
		if err := s.kv.Purge(key); err != nil && !isMiss(err) {
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

func (s *store) cacheKey(key string) string {
	return s.scopePrefixStr + encodeKeyPart(key)
}

func (s *store) scopePrefix() string { return s.scopePrefixStr }

func (s *store) encodeEnvelope(value []byte, ttl time.Duration) ([]byte, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	expiresAt := time.Now().Add(ttl).UnixMilli()
	body := make([]byte, 12+len(value))
	copy(body[:4], natsEnvelopeMagic)
	binary.BigEndian.PutUint64(body[4:12], uint64(expiresAt))
	copy(body[12:], value)
	return body, nil
}

func decodeEnvelope(body []byte) (envelope, bool, error) {
	if len(body) >= 12 && bytes.Equal(body[:4], natsEnvelopeMagic) {
		return envelope{
			Marker:    natsEnvelopeMarker,
			ExpiresAt: int64(binary.BigEndian.Uint64(body[4:12])),
			Value:     body[12:],
		}, true, nil
	}

	var env envelope
	if len(body) == 0 || body[0] != '{' {
		return env, false, nil
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return envelope{}, false, fmt.Errorf("decode nats cache envelope: %w", err)
	}
	if env.Marker != natsEnvelopeMarker {
		return envelope{}, false, nil
	}
	return env, true, nil
}

func isMiss(err error) bool {
	return errors.Is(err, nats.ErrKeyNotFound) || errors.Is(err, nats.ErrKeyDeleted)
}

func encodeKeyPart(part string) string {
	if part == "" {
		return "_"
	}
	return base64.RawURLEncoding.EncodeToString([]byte(part))
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	out := make([]byte, len(value))
	copy(out, value)
	return out
}
