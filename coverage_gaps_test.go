package cache

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

// TestStoreReadinessMethods verifies local stores and diagnostic stores expose readiness consistently.
func TestStoreReadinessMethods(t *testing.T) {
	ctx := context.Background()
	for name, store := range map[string]cachecore.Store{
		"memory": newMemoryStore(0, 0),
		"file":   newFileStore(t.TempDir(), 0),
		"null":   newNullStore(),
	} {
		t.Run(name, func(t *testing.T) {
			if err := store.Ready(ctx); err != nil {
				t.Fatalf("Ready() error = %v", err)
			}
		})
	}

	expected := errors.New("construction failed")
	if err := (&errorStore{driver: cachecore.DriverMemory, err: expected}).Ready(ctx); !errors.Is(err, expected) {
		t.Fatalf("error store Ready() = %v, want %v", err, expected)
	}

	path := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write regular file: %v", err)
	}
	if err := (&fileStore{dir: path, defaultTTL: time.Minute}).Ready(ctx); err == nil {
		t.Fatal("file store Ready() accepted a regular file")
	}
}

// TestCacheInspectorAccess verifies nil, unsupported, and supported inspector paths.
func TestCacheInspectorAccess(t *testing.T) {
	var nilCache *Cache
	if inspector, ok := nilCache.Inspector(); ok || inspector != nil {
		t.Fatalf("nil cache Inspector() = %T, %v", inspector, ok)
	}
	if _, err := ListPage(context.Background(), nil, cachecore.ListPageOptions{}); !errors.Is(err, ErrInspectorUnsupported) {
		t.Fatalf("nil cache ListPage() error = %v", err)
	}

	unsupported := NewCache(&spyStore{driver: cachecore.DriverMemory})
	if inspector, ok := unsupported.Inspector(); ok || inspector != nil {
		t.Fatalf("unsupported Inspector() = %T, %v", inspector, ok)
	}
	if _, err := ListPage(context.Background(), unsupported, cachecore.ListPageOptions{}); !errors.Is(err, ErrInspectorUnsupported) {
		t.Fatalf("unsupported ListPage() error = %v", err)
	}

	supported := NewCache(newMemoryStore(0, 0))
	if err := supported.SetString("profile:1", "Ada", time.Minute); err != nil {
		t.Fatalf("SetString() error = %v", err)
	}
	inspector, ok := supported.Inspector()
	if !ok || !inspector.Capabilities().CanList {
		t.Fatalf("supported Inspector() = %T, %v", inspector, ok)
	}
	page, err := ListPage(context.Background(), supported, cachecore.ListPageOptions{Query: "profile:"})
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Key != "profile:1" {
		t.Fatalf("ListPage() = %+v, %v", page, err)
	}
}

// TestMemoStoreInspectorDelegation verifies memoization preserves optional store extensions.
func TestMemoStoreInspectorDelegation(t *testing.T) {
	ctx := context.Background()
	supported := NewMemoStore(newMemoryStore(0, 0))
	if err := supported.Ready(ctx); err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	inspector := supported.(cachecore.Inspector)
	if caps := inspector.Capabilities(); !caps.CanList || !caps.CanTTL {
		t.Fatalf("Capabilities() = %+v", caps)
	}
	if err := supported.Set(ctx, "memo:key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	page, err := inspector.ListPage(ctx, cachecore.ListPageOptions{Query: "memo:"})
	if err != nil || len(page.Entries) != 1 {
		t.Fatalf("ListPage() = %+v, %v", page, err)
	}

	unsupported := NewMemoStore(&spyStore{driver: cachecore.DriverMemory})
	unsupportedInspector := unsupported.(cachecore.Inspector)
	if caps := unsupportedInspector.Capabilities(); caps != (cachecore.InspectorCapabilities{}) {
		t.Fatalf("unsupported Capabilities() = %+v", caps)
	}
	if _, err := unsupportedInspector.ListPage(ctx, cachecore.ListPageOptions{}); !errors.Is(err, ErrInspectorUnsupported) {
		t.Fatalf("unsupported ListPage() error = %v", err)
	}
}

// TestMemoryStoreInspectorVariants verifies metadata sizing and invalid cursor handling.
func TestMemoryStoreInspectorVariants(t *testing.T) {
	store := newMemoryStore(0, 0).(*memoryStore)
	store.cache.Set("string", "value", -1)
	store.cache.Set("other", struct{}{}, -1)

	page, err := store.ListPage(context.Background(), cachecore.ListPageOptions{})
	if err != nil {
		t.Fatalf("ListPage() error = %v", err)
	}
	if len(page.Entries) != 2 || page.Entries[1].Key != "string" || page.Entries[1].SizeBytes != len("value") {
		t.Fatalf("ListPage() = %+v", page)
	}
	if page.Entries[0].ExpiresAt != nil || page.Entries[1].ExpiresAt != nil {
		t.Fatalf("non-expiring entries reported expiration: %+v", page.Entries)
	}
	if _, err := store.ListPage(context.Background(), cachecore.ListPageOptions{Cursor: "invalid"}); err == nil {
		t.Fatal("ListPage() accepted an invalid cursor")
	}
}

// TestFileStoreInspectorMissingDirectory verifies browsing a removed cache directory is an empty success.
func TestFileStoreInspectorMissingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	store := &fileStore{dir: dir, defaultTTL: time.Minute}
	page, err := store.ListPage(context.Background(), cachecore.ListPageOptions{})
	if err != nil || len(page.Entries) != 0 {
		t.Fatalf("ListPage() = %+v, %v", page, err)
	}
}

// TestTypedHelperFailureAdapters verifies typed wrappers preserve callback and decoding failures.
func TestTypedHelperFailureAdapters(t *testing.T) {
	ctx := context.Background()
	cache := NewCache(newMemoryStore(0, 0))

	if _, err := rememberValue[int](ctx, cache, "remember:nil", time.Minute, nil); err == nil {
		t.Fatal("rememberValue accepted a nil callback")
	}
	if _, _, err := rememberStaleValue[int](ctx, cache, "stale:nil", time.Minute, time.Minute, nil); err == nil {
		t.Fatal("rememberStaleValue accepted a nil callback")
	}
	if _, err := refreshAheadValue[int](ctx, cache, "refresh:nil", time.Minute, time.Second, nil); err == nil {
		t.Fatal("refreshAheadValue accepted a nil callback")
	}
	if _, _, err := cache.rememberStaleBytes(ctx, "stale:bytes:nil", time.Minute, time.Minute, nil); err == nil {
		t.Fatal("rememberStaleBytes accepted a nil callback")
	}

	if err := cache.SetBytes("malformed", []byte("{"), time.Minute); err != nil {
		t.Fatalf("SetBytes() error = %v", err)
	}
	if _, ok, err := getValue[map[string]string](ctx, cache, "malformed"); err == nil || ok {
		t.Fatalf("getValue malformed result = ok %v, error %v", ok, err)
	}

	expected := errors.New("decode failed")
	codec := ValueCodec[int]{
		Encode: func(value int) ([]byte, error) { return []byte("value"), nil },
		Decode: func([]byte) (int, error) { return 0, expected },
	}
	if _, err := RefreshAheadValueWithCodec(ctx, cache, "decode", time.Minute, time.Second, func() (int, error) {
		return 1, nil
	}, codec); !errors.Is(err, expected) {
		t.Fatalf("RefreshAheadValueWithCodec error = %v, want %v", err, expected)
	}
}

// TestRefreshAndStaleWriteFailures verifies loaders do not report success when the primary write fails.
func TestRefreshAndStaleWriteFailures(t *testing.T) {
	expected := errors.New("write failed")
	for name, call := range map[string]func(*Cache) error{
		"refresh": func(cache *Cache) error {
			_, err := cache.refreshAheadBytes(context.Background(), "key", time.Minute, time.Second, func(context.Context) ([]byte, error) {
				return []byte("value"), nil
			})
			return err
		},
		"stale": func(cache *Cache) error {
			_, _, err := cache.rememberStaleBytes(context.Background(), "key", time.Minute, time.Minute, func(context.Context) ([]byte, error) {
				return []byte("value"), nil
			})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			cache := NewCache(&spyStore{driver: cachecore.DriverMemory, setErr: expected})
			if err := call(cache); !errors.Is(err, expected) {
				t.Fatalf("error = %v, want %v", err, expected)
			}
		})
	}
}

// TestLockHandleReleaseFailureRetainsOwnership verifies callers can retry a failed unlock.
func TestLockHandleReleaseFailureRetainsOwnership(t *testing.T) {
	expected := errors.New("delete failed")
	store := &spyStore{driver: cachecore.DriverMemory, addOK: true, delErr: expected}
	lock := NewCache(store).NewLockHandle("key", time.Minute)
	if locked, err := lock.Acquire(); err != nil || !locked {
		t.Fatalf("Acquire() = %v, %v", locked, err)
	}
	if err := lock.Release(); !errors.Is(err, expected) {
		t.Fatalf("first Release() error = %v, want %v", err, expected)
	}
	store.delErr = nil
	if err := lock.Release(); err != nil {
		t.Fatalf("retry Release() error = %v", err)
	}
}

// TestLockHandleInternalNilCallbacks verifies context-aware adapters retain callback validation.
func TestLockHandleInternalNilCallbacks(t *testing.T) {
	ctx := context.Background()
	first := NewCache(&spyStore{driver: cachecore.DriverMemory, addOK: true}).NewLockHandle("get", time.Minute)
	if locked, err := first.get(ctx, nil); err == nil || !locked {
		t.Fatalf("get(nil) = %v, %v", locked, err)
	}
	second := NewCache(&spyStore{driver: cachecore.DriverMemory, addOK: true}).NewLockHandle("block", time.Minute)
	if locked, err := second.block(ctx, 0, nil); err == nil || !locked {
		t.Fatalf("block(nil) = %v, %v", locked, err)
	}
}

// TestFileStoreFailureAndLegacyBranches verifies corrupt records and filesystem failures remain classified.
func TestFileStoreFailureAndLegacyBranches(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := newFileStore(dir, time.Minute).(*fileStore)

	if !isFileStoreEntry(".cache-tmp-fixture") {
		t.Fatal("temporary cache record was not recognized")
	}
	if err := os.WriteFile(store.path("add-corrupt"), []byte("corrupt"), 0o600); err != nil {
		t.Fatalf("write corrupt Add fixture: %v", err)
	}
	if created, err := store.Add(ctx, "add-corrupt", []byte("value"), time.Minute); err == nil || created {
		t.Fatalf("Add() = %v, %v", created, err)
	}
	if err := os.WriteFile(store.path("increment-corrupt"), []byte("corrupt"), 0o600); err != nil {
		t.Fatalf("write corrupt Increment fixture: %v", err)
	}
	if _, err := store.Increment(ctx, "increment-corrupt", 1, time.Minute); err == nil {
		t.Fatal("Increment() accepted a corrupt record")
	}

	original := createTempFile
	expected := errors.New("create failed")
	createTempFile = func(string, string) (*os.File, error) { return nil, expected }
	_, incrementErr := store.Increment(ctx, "increment-write", 1, time.Minute)
	createTempFile = original
	if !errors.Is(incrementErr, expected) {
		t.Fatalf("Increment() write error = %v, want %v", incrementErr, expected)
	}

	invalidV2 := make([]byte, 16)
	copy(invalidV2, fileRecordMagicV2)
	binary.BigEndian.PutUint32(invalidV2[12:16], 99)
	if _, _, _, err := decodeFileRecord(invalidV2); err == nil {
		t.Fatal("decodeFileRecord accepted an invalid key length")
	}
	legacyV1 := make([]byte, 12)
	copy(legacyV1, fileRecordMagic)
	binary.BigEndian.PutUint64(legacyV1[4:12], uint64(time.Now().Add(time.Minute).UnixNano()))
	if err := os.WriteFile(store.path("legacy-v1"), append(legacyV1, []byte("value")...), 0o600); err != nil {
		t.Fatalf("write v1 fixture: %v", err)
	}
	if err := os.WriteFile(store.path("inspect-corrupt"), []byte("corrupt"), 0o600); err != nil {
		t.Fatalf("write inspector fixture: %v", err)
	}
	if _, err := store.ListPage(ctx, cachecore.ListPageOptions{Cursor: "invalid"}); err == nil {
		t.Fatal("ListPage accepted an invalid cursor")
	}

	filePath := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(filePath, []byte("value"), 0o600); err != nil {
		t.Fatalf("write regular file: %v", err)
	}
	fileStore := &fileStore{dir: filePath, defaultTTL: time.Minute}
	if err := fileStore.Flush(ctx); err == nil {
		t.Fatal("Flush() accepted a regular file as its directory")
	}
	if _, err := fileStore.ListPage(ctx, cachecore.ListPageOptions{}); err == nil {
		t.Fatal("ListPage() accepted a regular file as its directory")
	}
}

// TestMemoryStoreAdditionalNumericVariants verifies historical string and int counter representations.
func TestMemoryStoreAdditionalNumericVariants(t *testing.T) {
	store := newMemoryStore(0, 0).(*memoryStore)
	store.cache.Set("bad-string", "not-a-number", time.Minute)
	if _, _, err := store.readInt64("bad-string"); err == nil {
		t.Fatal("readInt64 accepted a malformed string")
	}
	store.cache.Set("int", int(12), time.Minute)
	if value, ok, err := store.readInt64("int"); err != nil || !ok || value != 12 {
		t.Fatalf("readInt64(int) = %d, %v, %v", value, ok, err)
	}
	store.cache.Set("uint16", uint16(13), time.Minute)
	if value, ok, err := store.readInt64("uint16"); err != nil || !ok || value != 13 {
		t.Fatalf("readInt64(uint16) = %d, %v, %v", value, ok, err)
	}
}
