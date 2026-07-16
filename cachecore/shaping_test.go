package cachecore

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

// shapingTestStore provides an observable backend for wrapper contract tests.
type shapingTestStore struct {
	mu     sync.Mutex
	values map[string][]byte
}

// newShapingTestStore constructs an empty observable backend.
func newShapingTestStore() *shapingTestStore {
	return &shapingTestStore{values: make(map[string][]byte)}
}

// Driver identifies the test backend as memory.
func (s *shapingTestStore) Driver() Driver { return DriverMemory }

// Ready reports that the test backend is available.
func (s *shapingTestStore) Ready(context.Context) error { return nil }

// Get returns a clone so wrapper tests cannot mutate stored envelopes.
func (s *shapingTestStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[key]
	return bytes.Clone(value), ok, nil
}

// Set records a cloned value without interpreting its envelope.
func (s *shapingTestStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	s.values[key] = bytes.Clone(value)
	s.mu.Unlock()
	return nil
}

// Add records a value only when no value is present.
func (s *shapingTestStore) Add(_ context.Context, key string, value []byte, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.values[key]; ok {
		return false, nil
	}
	s.values[key] = bytes.Clone(value)
	return true, nil
}

// Increment updates a raw decimal value to model backend-native counter behavior.
func (s *shapingTestStore) Increment(_ context.Context, key string, delta int64, _ time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := strconv.ParseInt(string(s.values[key]), 10, 64)
	if len(s.values[key]) == 0 {
		err = nil
	}
	if err != nil {
		return 0, err
	}
	current += delta
	s.values[key] = []byte(strconv.FormatInt(current, 10))
	return current, nil
}

// Decrement updates a raw decimal value to model backend-native counter behavior.
func (s *shapingTestStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

// Delete removes one recorded value.
func (s *shapingTestStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	delete(s.values, key)
	s.mu.Unlock()
	return nil
}

// DeleteMany removes all requested values.
func (s *shapingTestStore) DeleteMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// Flush removes all recorded values.
func (s *shapingTestStore) Flush(context.Context) error {
	s.mu.Lock()
	s.values = make(map[string][]byte)
	s.mu.Unlock()
	return nil
}

// Capabilities reports deterministic inspector support.
func (s *shapingTestStore) Capabilities() InspectorCapabilities {
	return InspectorCapabilities{CanList: true, CanRead: true, CanDelete: true, CanTTL: true}
}

// ListPage returns a sentinel page so delegation is directly observable.
func (s *shapingTestStore) ListPage(context.Context, ListPageOptions) (ListPageResult, error) {
	expiresAt := int64(1234)
	return ListPageResult{Entries: []CacheEntry{{Key: "sentinel", SizeBytes: 7, ExpiresAt: &expiresAt}}}, nil
}

// raw returns a clone of the persisted backend bytes.
func (s *shapingTestStore) raw(key string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.values[key])
}

// TestWrapStoreShapedRoundTripAndEnvelopeOrder protects the established combined wire format.
func TestWrapStoreShapedRoundTripAndEnvelopeOrder(t *testing.T) {
	base := newShapingTestStore()
	key := []byte("0123456789abcdef0123456789abcdef")
	store, err := WrapStore(base, BaseConfig{Compression: CompressionGzip, EncryptionKey: key})
	if err != nil {
		t.Fatalf("wrap store: %v", err)
	}
	ctx := context.Background()
	if err := store.Set(ctx, "secret", []byte("payload"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	persisted := base.raw("secret")
	if !bytes.HasPrefix(persisted, compressionMagic) {
		t.Fatalf("persisted value does not begin with CMP1: %x", persisted)
	}
	compressed, err := decodeValue(persisted, 0)
	if err != nil {
		t.Fatalf("decode outer envelope: %v", err)
	}
	if !bytes.HasPrefix(compressed, encryptionMagic) {
		t.Fatalf("inner value does not begin with ENC1: %x", compressed)
	}
	got, ok, err := store.Get(ctx, "secret")
	if err != nil || !ok || string(got) != "payload" {
		t.Fatalf("round trip: got=%q ok=%v err=%v", got, ok, err)
	}
}

// TestWrapStoreReadsLegacyCombinedEnvelope proves the centralized wrapper reads the original root-store format.
func TestWrapStoreReadsLegacyCombinedEnvelope(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("new GCM: %v", err)
	}
	nonce := bytes.Repeat([]byte{0xAB}, aead.NonceSize())
	encrypted := append(bytes.Clone(encryptionMagic), byte(len(nonce)))
	encrypted = append(encrypted, nonce...)
	encrypted = append(encrypted, aead.Seal(nil, nonce, []byte("legacy-combined"), nil)...)

	var persisted bytes.Buffer
	persisted.Write(compressionMagic)
	_ = persisted.WriteByte('g')
	writer, err := gzip.NewWriterLevel(&persisted, gzip.BestSpeed)
	if err != nil {
		t.Fatalf("new gzip writer: %v", err)
	}
	if _, err := writer.Write(encrypted); err != nil {
		t.Fatalf("write gzip fixture: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip fixture: %v", err)
	}

	base := newShapingTestStore()
	if err := base.Set(context.Background(), "legacy", persisted.Bytes(), time.Minute); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}
	store, err := WrapStore(base, BaseConfig{Compression: CompressionGzip, EncryptionKey: key})
	if err != nil {
		t.Fatalf("wrap store: %v", err)
	}
	got, ok, err := store.Get(context.Background(), "legacy")
	if err != nil || !ok || string(got) != "legacy-combined" {
		t.Fatalf("legacy combined read: got=%q ok=%v err=%v", got, ok, err)
	}
}

// TestWrapStoreReadsLegacyRawValues proves enabling shaping does not strand existing entries.
func TestWrapStoreReadsLegacyRawValues(t *testing.T) {
	base := newShapingTestStore()
	if err := base.Set(context.Background(), "legacy", []byte("raw"), time.Minute); err != nil {
		t.Fatalf("seed legacy value: %v", err)
	}
	store, err := WrapStore(base, BaseConfig{
		Compression:   CompressionGzip,
		EncryptionKey: []byte("0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatalf("wrap store: %v", err)
	}
	got, ok, err := store.Get(context.Background(), "legacy")
	if err != nil || !ok || string(got) != "raw" {
		t.Fatalf("legacy read: got=%q ok=%v err=%v", got, ok, err)
	}
}

// TestWrapStoreRejectsWrongEncryptionKey proves encrypted entries fail closed.
func TestWrapStoreRejectsWrongEncryptionKey(t *testing.T) {
	base := newShapingTestStore()
	writer, err := WrapStore(base, BaseConfig{EncryptionKey: []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatalf("wrap writer: %v", err)
	}
	if err := writer.Set(context.Background(), "secret", []byte("payload"), time.Minute); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	reader, err := WrapStore(base, BaseConfig{EncryptionKey: []byte("abcdef0123456789abcdef0123456789")})
	if err != nil {
		t.Fatalf("wrap reader: %v", err)
	}
	if _, ok, err := reader.Get(context.Background(), "secret"); !errors.Is(err, ErrDecryptFailed) || ok {
		t.Fatalf("wrong-key read: ok=%v err=%v", ok, err)
	}
}

// TestWrapStoreConfigurationFailuresFailClosed verifies Store-only constructors can safely return the diagnostic store.
func TestWrapStoreConfigurationFailuresFailClosed(t *testing.T) {
	store, err := WrapStore(newShapingTestStore(), BaseConfig{EncryptionKey: []byte("short")})
	if !errors.Is(err, ErrEncryptionKey) {
		t.Fatalf("configuration error = %v, want ErrEncryptionKey", err)
	}
	ctx := context.Background()
	checks := []func() error{
		func() error { return store.Ready(ctx) },
		func() error { _, _, err := store.Get(ctx, "k"); return err },
		func() error { return store.Set(ctx, "k", []byte("v"), time.Minute) },
		func() error { _, err := store.Add(ctx, "k", []byte("v"), time.Minute); return err },
		func() error { _, err := store.Increment(ctx, "k", 1, time.Minute); return err },
		func() error { _, err := store.Decrement(ctx, "k", 1, time.Minute); return err },
		func() error { return store.Delete(ctx, "k") },
		func() error { return store.DeleteMany(ctx, "k") },
		func() error { return store.Flush(ctx) },
	}
	for i, check := range checks {
		if err := check(); !errors.Is(err, ErrEncryptionKey) {
			t.Fatalf("operation %d error = %v, want ErrEncryptionKey", i, err)
		}
	}
}

// TestWrapStoreEnforcesMaxValueBytes verifies Set and Add share the same shaping policy.
func TestWrapStoreEnforcesMaxValueBytes(t *testing.T) {
	store, err := WrapStore(newShapingTestStore(), BaseConfig{MaxValueBytes: 3})
	if err != nil {
		t.Fatalf("wrap store: %v", err)
	}
	ctx := context.Background()
	if err := store.Set(ctx, "set", []byte("four"), time.Minute); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("Set error = %v, want ErrValueTooLarge", err)
	}
	if _, err := store.Add(ctx, "add", []byte("four"), time.Minute); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("Add error = %v, want ErrValueTooLarge", err)
	}
}

// TestWrapStoreEnforcesMaxValueBytesOnReads rejects oversized raw and compressed backend values.
func TestWrapStoreEnforcesMaxValueBytesOnReads(t *testing.T) {
	ctx := context.Background()

	rawBase := newShapingTestStore()
	if err := rawBase.Set(ctx, "legacy", []byte("four"), time.Minute); err != nil {
		t.Fatalf("seed raw value: %v", err)
	}
	rawStore, err := WrapStore(rawBase, BaseConfig{MaxValueBytes: 3})
	if err != nil {
		t.Fatalf("wrap raw store: %v", err)
	}
	if _, ok, err := rawStore.Get(ctx, "legacy"); !errors.Is(err, ErrValueTooLarge) || ok {
		t.Fatalf("oversized raw read: ok=%v err=%v", ok, err)
	}

	compressedBase := newShapingTestStore()
	large := bytes.Repeat([]byte("a"), 256*1024)
	encoded, err := encodeValue(CompressionGzip, 0, large)
	if err != nil {
		t.Fatalf("encode compressed fixture: %v", err)
	}
	if len(encoded) >= 1024 {
		t.Fatalf("compressed fixture is too large to exercise bounded expansion: %d bytes", len(encoded))
	}
	if err := compressedBase.Set(ctx, "bomb", encoded, time.Minute); err != nil {
		t.Fatalf("seed compressed value: %v", err)
	}
	compressedStore, err := WrapStore(compressedBase, BaseConfig{Compression: CompressionGzip, MaxValueBytes: 1024})
	if err != nil {
		t.Fatalf("wrap compressed store: %v", err)
	}
	if _, ok, err := compressedStore.Get(ctx, "bomb"); !errors.Is(err, ErrValueTooLarge) || ok {
		t.Fatalf("oversized compressed read: ok=%v err=%v", ok, err)
	}
}

// TestWrapStoreLeavesCountersRaw verifies atomic backend primitives are never enveloped.
func TestWrapStoreLeavesCountersRaw(t *testing.T) {
	base := newShapingTestStore()
	store, err := WrapStore(base, BaseConfig{
		Compression:   CompressionGzip,
		EncryptionKey: []byte("0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatalf("wrap store: %v", err)
	}
	value, err := store.Increment(context.Background(), "counter", 3, time.Minute)
	if err != nil || value != 3 {
		t.Fatalf("increment: value=%d err=%v", value, err)
	}
	if got := string(base.raw("counter")); got != "3" {
		t.Fatalf("persisted counter = %q, want raw decimal", got)
	}
}

// TestWrapStoreDelegatesInspector proves wrappers preserve optional browsing behavior.
func TestWrapStoreDelegatesInspector(t *testing.T) {
	store, err := WrapStore(newShapingTestStore(), BaseConfig{Compression: CompressionGzip})
	if err != nil {
		t.Fatalf("wrap store: %v", err)
	}
	inspector, ok := store.(Inspector)
	if !ok {
		t.Fatalf("wrapped store does not implement Inspector")
	}
	if caps := inspector.Capabilities(); !caps.CanList || !caps.CanTTL {
		t.Fatalf("capabilities were not delegated: %+v", caps)
	}
	page, err := inspector.ListPage(context.Background(), ListPageOptions{})
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Key != "sentinel" {
		t.Fatalf("page was not delegated: page=%+v err=%v", page, err)
	}
}

// TestWrapStorePreservesUnsupportedInspectorError keeps error classification stable through shared wrappers.
func TestWrapStorePreservesUnsupportedInspectorError(t *testing.T) {
	base := &storeWithoutInspector{Store: newShapingTestStore()}
	store, err := WrapStore(base, BaseConfig{Compression: CompressionGzip})
	if err != nil {
		t.Fatalf("wrap store: %v", err)
	}
	inspector, ok := store.(Inspector)
	if !ok {
		t.Fatalf("wrapped store does not expose its unsupported inspector result")
	}
	if _, err := inspector.ListPage(context.Background(), ListPageOptions{}); !errors.Is(err, ErrInspectorUnsupported) {
		t.Fatalf("ListPage error = %v, want ErrInspectorUnsupported", err)
	}
}

// storeWithoutInspector delegates Store operations without implementing Inspector.
type storeWithoutInspector struct {
	Store
}

// TestWrapStoreRejectsUnsupportedCompression verifies reserved and unknown codecs fail at construction.
func TestWrapStoreRejectsUnsupportedCompression(t *testing.T) {
	for _, codec := range []CompressionCodec{CompressionSnappy, "unknown"} {
		store, err := WrapStore(newShapingTestStore(), BaseConfig{Compression: codec})
		if !errors.Is(err, ErrUnsupportedCodec) {
			t.Fatalf("codec %q error = %v, want ErrUnsupportedCodec", codec, err)
		}
		if err := store.Ready(context.Background()); !errors.Is(err, ErrUnsupportedCodec) {
			t.Fatalf("codec %q diagnostic store error = %v", codec, err)
		}
	}
}

// TestWrapStoreRejectsNegativeMaxValueBytes verifies the documented zero-or-positive configuration contract.
func TestWrapStoreRejectsNegativeMaxValueBytes(t *testing.T) {
	store, err := WrapStore(newShapingTestStore(), BaseConfig{MaxValueBytes: -1})
	if !errors.Is(err, ErrInvalidMaxValueBytes) {
		t.Fatalf("configuration error = %v, want ErrInvalidMaxValueBytes", err)
	}
	if err := store.Ready(context.Background()); !errors.Is(err, ErrInvalidMaxValueBytes) {
		t.Fatalf("diagnostic store error = %v, want ErrInvalidMaxValueBytes", err)
	}
}

// TestDecodeValueRejectsCorruptEnvelope verifies stable classification for malformed persisted data.
func TestDecodeValueRejectsCorruptEnvelope(t *testing.T) {
	if _, err := decodeValue([]byte("CMP1gnot-gzip"), 0); !errors.Is(err, ErrCorruptCompression) {
		t.Fatalf("corrupt gzip error = %v", err)
	}
	if _, err := decodeValue([]byte("CMP1xpayload"), 0); !errors.Is(err, ErrUnsupportedCodec) {
		t.Fatalf("unknown envelope error = %v", err)
	}
}
