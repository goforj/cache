package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestNATSStoreNilKeyValueErrors(t *testing.T) {
	store := newNATSStore(nil, 0, "", false)
	ctx := context.Background()

	if _, _, err := store.Get(ctx, "k"); err == nil {
		t.Fatalf("expected get error when nats key-value is nil")
	}
	if err := store.Set(ctx, "k", []byte("v"), 0); err == nil {
		t.Fatalf("expected set error when nats key-value is nil")
	}
	if _, err := store.Add(ctx, "k", []byte("v"), 0); err == nil {
		t.Fatalf("expected add error when nats key-value is nil")
	}
	if _, err := store.Increment(ctx, "k", 1, 0); err == nil {
		t.Fatalf("expected increment error when nats key-value is nil")
	}
	if _, err := store.Decrement(ctx, "k", 1, 0); err == nil {
		t.Fatalf("expected decrement error when nats key-value is nil")
	}
	if err := store.Delete(ctx, "k"); err == nil {
		t.Fatalf("expected delete error when nats key-value is nil")
	}
	if err := store.DeleteMany(ctx, "a", "b"); err == nil {
		t.Fatalf("expected delete many error when nats key-value is nil")
	}
	if err := store.Flush(ctx); err == nil {
		t.Fatalf("expected flush error when nats key-value is nil")
	}
}

func TestNATSStoreOperationsWithStubKV(t *testing.T) {
	ctx := context.Background()
	kv := newStubNATSKeyValue("bucket")
	store := newNATSStore(kv, 100*time.Millisecond, "pfx", false)

	if err := store.Set(ctx, "alpha", []byte("one"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "alpha")
	if err != nil || !ok || string(body) != "one" {
		t.Fatalf("unexpected get result: ok=%v err=%v body=%s", ok, err, string(body))
	}

	created, err := store.Add(ctx, "alpha", []byte("two"), 0)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if created {
		t.Fatalf("expected add false when key exists")
	}

	created, err = store.Add(ctx, "beta", []byte("two"), 0)
	if err != nil || !created {
		t.Fatalf("expected add true on missing key, created=%v err=%v", created, err)
	}

	val, err := store.Increment(ctx, "counter", 3, time.Second)
	if err != nil || val != 3 {
		t.Fatalf("increment failed: val=%d err=%v", val, err)
	}
	val, err = store.Decrement(ctx, "counter", 1, time.Second)
	if err != nil || val != 2 {
		t.Fatalf("decrement failed: val=%d err=%v", val, err)
	}

	if err := store.Delete(ctx, "alpha"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "alpha"); err != nil || ok {
		t.Fatalf("expected alpha deleted")
	}

	if err := store.Set(ctx, "a", []byte("1"), 0); err != nil {
		t.Fatalf("set a failed: %v", err)
	}
	if err := store.Set(ctx, "b", []byte("2"), 0); err != nil {
		t.Fatalf("set b failed: %v", err)
	}
	if err := store.DeleteMany(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}

	if err := store.Set(ctx, "flushme", []byte("x"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "flushme"); err != nil || ok {
		t.Fatalf("expected flushed key to be gone")
	}
}

func TestNATSStoreExpiryAndDefaultTTL(t *testing.T) {
	ctx := context.Background()
	kv := newStubNATSKeyValue("bucket")
	store := newNATSStore(kv, 25*time.Millisecond, "pfx", false)

	if err := store.Set(ctx, "exp", []byte("v"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	if _, ok, err := store.Get(ctx, "exp"); err != nil || ok {
		t.Fatalf("expected key expired; ok=%v err=%v", ok, err)
	}

	created, err := store.Add(ctx, "exp", []byte("new"), 0)
	if err != nil || !created {
		t.Fatalf("expected add after expiry to succeed, created=%v err=%v", created, err)
	}
}

func TestNATSStoreBucketTTLModeStoresRawValues(t *testing.T) {
	ctx := context.Background()
	kv := newStubNATSKeyValue("bucket")
	store := newNATSStore(kv, 10*time.Millisecond, "pfx", true)

	if err := store.Set(ctx, "raw", []byte("value"), 5*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "raw")
	if err != nil || !ok || string(body) != "value" {
		t.Fatalf("unexpected get result: ok=%v err=%v body=%q", ok, err, string(body))
	}

	created, err := store.Add(ctx, "addraw", []byte("x"), time.Millisecond)
	if err != nil || !created {
		t.Fatalf("add failed: created=%v err=%v", created, err)
	}

	val, err := store.Increment(ctx, "counter_raw", 2, time.Millisecond)
	if err != nil || val != 2 {
		t.Fatalf("increment failed: val=%d err=%v", val, err)
	}
}

func TestNATSStoreReadsLegacyJSONEnvelope(t *testing.T) {
	ctx := context.Background()
	kv := newStubNATSKeyValue("bucket")
	store := newNATSStore(kv, time.Second, "pfx", false)

	legacy := natsEnvelope{
		Marker:    natsEnvelopeMarker,
		Value:     []byte("legacy"),
		ExpiresAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	body, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy envelope: %v", err)
	}
	if _, err := kv.Put(store.(*natsStore).cacheKey("legacy"), body); err != nil {
		t.Fatalf("seed legacy envelope: %v", err)
	}

	got, ok, err := store.Get(ctx, "legacy")
	if err != nil || !ok || string(got) != "legacy" {
		t.Fatalf("expected legacy envelope read, ok=%v err=%v val=%q", ok, err, string(got))
	}
}

func TestNATSStoreIncrementOnNonNumericValue(t *testing.T) {
	ctx := context.Background()
	kv := newStubNATSKeyValue("bucket")
	store := newNATSStore(kv, time.Second, "pfx", false)
	if err := store.Set(ctx, "badnum", []byte("not-a-number"), time.Second); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if _, err := store.Increment(ctx, "badnum", 1, time.Second); err == nil {
		t.Fatalf("expected increment error on non-numeric value")
	}
}

func TestNATSStoreFlushRespectsPrefix(t *testing.T) {
	ctx := context.Background()
	kv := newStubNATSKeyValue("bucket")
	store := newNATSStore(kv, time.Second, "pfx", false)
	ns := store.(*natsStore)

	inKey := ns.cacheKey("in")
	if _, err := kv.Put(inKey, mustEncodeNATSEnvelope(t, []byte("1"), time.Second)); err != nil {
		t.Fatalf("put in failed: %v", err)
	}
	otherKey := "p." + encodeNATSKeyPart("other") + ".k." + encodeNATSKeyPart("keep")
	if _, err := kv.Put(otherKey, mustEncodeNATSEnvelope(t, []byte("2"), time.Second)); err != nil {
		t.Fatalf("put keep failed: %v", err)
	}

	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if _, ok := kv.entries[inKey]; ok {
		t.Fatalf("expected prefixed key removed")
	}
	if _, ok := kv.entries[otherKey]; !ok {
		t.Fatalf("expected other prefix key retained")
	}
}

func TestNATSStoreErrorPropagation(t *testing.T) {
	ctx := context.Background()
	kv := newStubNATSKeyValue("bucket")
	store := newNATSStore(kv, time.Second, "pfx", false)

	kv.getErr = errors.New("get")
	if _, _, err := store.Get(ctx, "k"); err == nil {
		t.Fatalf("expected get error")
	}
	kv.getErr = nil

	kv.putErr = errors.New("put")
	if err := store.Set(ctx, "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected set error")
	}
	kv.putErr = nil

	kv.createErr = errors.New("create")
	if _, err := store.Add(ctx, "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected add error")
	}
	kv.createErr = nil

	if _, err := store.Increment(ctx, "counter", 1, time.Second); err != nil {
		t.Fatalf("seed increment failed: %v", err)
	}
	kv.updateErr = errors.New("update")
	if _, err := store.Increment(ctx, "counter", 1, time.Second); err == nil {
		t.Fatalf("expected increment update error")
	}
	kv.updateErr = nil

	kv.deleteErr = errors.New("delete")
	if err := store.Delete(ctx, "k"); err == nil {
		t.Fatalf("expected delete error")
	}
	kv.deleteErr = nil

	kv.listErr = errors.New("list")
	if err := store.Flush(ctx); err == nil {
		t.Fatalf("expected flush list error")
	}
}

func mustEncodeNATSEnvelope(t *testing.T, value []byte, ttl time.Duration) []byte {
	t.Helper()
	store := &natsStore{defaultTTL: time.Second, bucketTTL: false}
	body, err := store.encodeNATSEnvelope(value, ttl)
	if err != nil {
		t.Fatalf("encode envelope: %v", err)
	}
	return body
}

type stubNATSKeyValue struct {
	bucket string
	rev    uint64

	entries map[string]*stubNATSKeyValueEntry

	getErr    error
	putErr    error
	createErr error
	updateErr error
	deleteErr error
	purgeErr  error
	listErr   error
}

func newStubNATSKeyValue(bucket string) *stubNATSKeyValue {
	return &stubNATSKeyValue{
		bucket:  bucket,
		entries: make(map[string]*stubNATSKeyValueEntry),
	}
}

func (s *stubNATSKeyValue) Get(key string) (nats.KeyValueEntry, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	entry, ok := s.entries[key]
	if !ok {
		return nil, nats.ErrKeyNotFound
	}
	if entry.op == nats.KeyValueDelete || entry.op == nats.KeyValuePurge {
		return nil, nats.ErrKeyDeleted
	}
	return entry.clone(), nil
}

func (s *stubNATSKeyValue) Put(key string, value []byte) (uint64, error) {
	if s.putErr != nil {
		return 0, s.putErr
	}
	s.rev++
	s.entries[key] = &stubNATSKeyValueEntry{
		bucket:   s.bucket,
		key:      key,
		value:    cloneBytes(value),
		revision: s.rev,
		created:  time.Now(),
		op:       nats.KeyValuePut,
	}
	return s.rev, nil
}

func (s *stubNATSKeyValue) Create(key string, value []byte) (uint64, error) {
	if s.createErr != nil {
		return 0, s.createErr
	}
	if existing, ok := s.entries[key]; ok && existing.op == nats.KeyValuePut {
		return 0, nats.ErrKeyExists
	}
	return s.Put(key, value)
}

func (s *stubNATSKeyValue) Update(key string, value []byte, last uint64) (uint64, error) {
	if s.updateErr != nil {
		return 0, s.updateErr
	}
	existing, ok := s.entries[key]
	if !ok || existing.op != nats.KeyValuePut {
		return 0, nats.ErrKeyNotFound
	}
	if existing.revision != last {
		return 0, nats.ErrKeyExists
	}
	return s.Put(key, value)
}

func (s *stubNATSKeyValue) Delete(key string, _ ...nats.DeleteOpt) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.rev++
	s.entries[key] = &stubNATSKeyValueEntry{
		bucket:   s.bucket,
		key:      key,
		revision: s.rev,
		created:  time.Now(),
		op:       nats.KeyValueDelete,
	}
	return nil
}

func (s *stubNATSKeyValue) Purge(key string, _ ...nats.DeleteOpt) error {
	if s.purgeErr != nil {
		return s.purgeErr
	}
	delete(s.entries, key)
	return nil
}

func (s *stubNATSKeyValue) ListKeys(_ ...nats.WatchOpt) (nats.KeyLister, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	keys := make([]string, 0, len(s.entries))
	for key := range s.entries {
		keys = append(keys, key)
	}
	return newStubNATSKeyLister(keys), nil
}

type stubNATSKeyValueEntry struct {
	bucket   string
	key      string
	value    []byte
	revision uint64
	created  time.Time
	delta    uint64
	op       nats.KeyValueOp
}

func (e *stubNATSKeyValueEntry) clone() *stubNATSKeyValueEntry {
	cp := *e
	cp.value = cloneBytes(e.value)
	return &cp
}

func (e *stubNATSKeyValueEntry) Bucket() string             { return e.bucket }
func (e *stubNATSKeyValueEntry) Key() string                { return e.key }
func (e *stubNATSKeyValueEntry) Value() []byte              { return cloneBytes(e.value) }
func (e *stubNATSKeyValueEntry) Revision() uint64           { return e.revision }
func (e *stubNATSKeyValueEntry) Created() time.Time         { return e.created }
func (e *stubNATSKeyValueEntry) Delta() uint64              { return e.delta }
func (e *stubNATSKeyValueEntry) Operation() nats.KeyValueOp { return e.op }

type stubNATSKeyLister struct {
	keysCh chan string
	errCh  chan error
}

func newStubNATSKeyLister(keys []string) *stubNATSKeyLister {
	keysCh := make(chan string, len(keys))
	errCh := make(chan error)
	for _, key := range keys {
		keysCh <- key
	}
	close(keysCh)
	close(errCh)
	return &stubNATSKeyLister{keysCh: keysCh, errCh: errCh}
}

func (l *stubNATSKeyLister) Keys() <-chan string { return l.keysCh }
func (l *stubNATSKeyLister) Error() <-chan error { return l.errCh }
func (l *stubNATSKeyLister) Stop() error         { return nil }
