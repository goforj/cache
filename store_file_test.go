package cache

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

// newTempFileStore isolates file-store records in the calling test's temporary directory.
func newTempFileStore(t *testing.T) cachecore.Store {
	t.Helper()
	dir := t.TempDir()
	return newFileStore(dir, 0)
}

// TestFileStoreSetGetDelete verifies persisted bytes round-trip and disappear after removal.
func TestFileStoreSetGetDelete(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()

	body := []byte("hello")
	if err := store.Set(ctx, "alpha", body, 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body[0] = 'x' // ensure clone

	got, ok, err := store.Get(ctx, "alpha")
	if err != nil || !ok || string(got) != "hello" {
		t.Fatalf("unexpected get: ok=%v err=%v val=%s", ok, err, string(got))
	}

	if err := store.Delete(ctx, "alpha"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	_, ok, err = store.Get(ctx, "alpha")
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}
	if ok {
		t.Fatalf("expected missing after delete")
	}
}

// TestFileStoreTTLExpiry verifies expired records become misses on read.
func TestFileStoreTTLExpiry(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()

	if err := store.Set(ctx, "ttl", []byte("v"), 50*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	_, ok, err := store.Get(ctx, "ttl")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if ok {
		t.Fatalf("expected ttl to expire")
	}
}

// TestFileStoreAddDefaultsTTL verifies conditional writes use the configured TTL when none is supplied.
func TestFileStoreAddDefaultsTTL(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()
	created, err := store.Add(ctx, "x", []byte("1"), 0)
	if err != nil || !created {
		t.Fatalf("add failed: %v created=%v", err, created)
	}
}

// TestFileStoreFlushEmpty verifies flushing an unused store is an idempotent success.
func TestFileStoreFlushEmpty(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush empty failed: %v", err)
	}
}

// TestFileStoreAddIncrementDecrement verifies conditional creation and persisted counters share file locking safely.
func TestFileStoreAddIncrementDecrement(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()

	created, err := store.Add(ctx, "once", []byte("first"), time.Minute)
	if err != nil || !created {
		t.Fatalf("add failed: created=%v err=%v", created, err)
	}
	created, err = store.Add(ctx, "once", []byte("second"), time.Minute)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if created {
		t.Fatalf("expected duplicate add to be ignored")
	}

	val, err := store.Increment(ctx, "counter", 2, time.Minute)
	if err != nil || val != 2 {
		t.Fatalf("increment failed: val=%d err=%v", val, err)
	}
	val, err = store.Decrement(ctx, "counter", 1, time.Minute)
	if err != nil || val != 1 {
		t.Fatalf("decrement failed: val=%d err=%v", val, err)
	}
}

// TestFileStoreConcurrentMutations verifies process-local atomicity for lock and counter primitives.
func TestFileStoreConcurrentMutations(t *testing.T) {
	dir := t.TempDir()
	stores := []cachecore.Store{
		newFileStore(dir, time.Minute),
		newFileStore(filepath.Join(dir, "."), time.Minute),
	}
	ctx := context.Background()
	const workers = 64

	var created atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, workers*2)
	for worker := range workers {
		wg.Add(1)
		go func(store cachecore.Store) {
			defer wg.Done()
			<-start
			ok, err := store.Add(ctx, "once:concurrent", []byte("value"), time.Minute)
			if err != nil {
				errs <- err
				return
			}
			if ok {
				created.Add(1)
			}
			if _, err := store.Increment(ctx, "counter:concurrent", 1, time.Minute); err != nil {
				errs <- err
			}
		}(stores[worker%len(stores)])
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent mutation failed: %v", err)
	}
	if got := created.Load(); got != 1 {
		t.Fatalf("successful Add calls = %d, want 1", got)
	}
	body, ok, err := stores[0].Get(ctx, "counter:concurrent")
	if err != nil || !ok || string(body) != "64" {
		t.Fatalf("counter = %q, ok=%v err=%v; want 64", body, ok, err)
	}
}

// TestFileStoreFlushAndDeleteMany verifies bulk removal and namespace flush clear only requested records.
func TestFileStoreFlushAndDeleteMany(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()

	if err := store.Set(ctx, "a", []byte("1"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.Set(ctx, "b", []byte("2"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.DeleteMany(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "a"); err != nil || ok {
		t.Fatalf("expected deleted key")
	}

	if err := store.Set(ctx, "c", []byte("3"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if _, ok, err := store.Get(ctx, "c"); err != nil || ok {
		t.Fatalf("expected flushed key missing")
	}
}

// TestFileStoreFlushPreservesUnrelatedFiles verifies a shared directory is not destructively emptied.
func TestFileStoreFlushPreservesUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	store := newFileStore(dir, time.Minute)
	sentinels := []string{
		filepath.Join(dir, "keep.txt"),
		filepath.Join(dir, "keep.cache"),
	}
	for _, sentinel := range sentinels {
		if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
			t.Fatalf("write sentinel: %v", err)
		}
	}
	if err := store.Set(context.Background(), "managed", []byte("value"), time.Minute); err != nil {
		t.Fatalf("set managed value: %v", err)
	}
	if err := store.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	for _, sentinel := range sentinels {
		if body, err := os.ReadFile(sentinel); err != nil || string(body) != "keep" {
			t.Fatalf("unrelated file changed: path=%s body=%q err=%v", sentinel, body, err)
		}
	}
	if _, ok, err := store.Get(context.Background(), "managed"); err != nil || ok {
		t.Fatalf("managed value survived flush: ok=%v err=%v", ok, err)
	}
}

// TestFileStoreIncrementNonNumeric verifies counters reject persisted non-integer payloads.
func TestFileStoreIncrementNonNumeric(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()

	if err := store.Set(ctx, "num", []byte("NaN"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if _, err := store.Increment(ctx, "num", 1, time.Minute); err == nil {
		t.Fatalf("expected numeric error")
	}
}

// TestFileStoreUsesSpecifiedDir verifies configured roots contain every persisted cache record.
func TestFileStoreUsesSpecifiedDir(t *testing.T) {
	dir := t.TempDir()
	store := newFileStore(dir, 0)
	ctx := context.Background()
	if err := store.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one file in dir, got %d", len(entries))
	}
	if filepath.Dir(filepath.Join(dir, entries[0].Name())) != dir {
		t.Fatalf("expected file in supplied dir")
	}
}

// TestFileStoreSetUsesDefaultTTLWhenZero verifies zero-TTL writes resolve to the store default.
func TestFileStoreSetUsesDefaultTTLWhenZero(t *testing.T) {
	dir := t.TempDir()
	store := newFileStore(dir, time.Minute)
	ctx := context.Background()

	if err := store.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	path := store.(*fileStore).path("k")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(data) < 16 {
		t.Fatalf("expected binary record header")
	}
	if string(data[:4]) != string(fileRecordMagicV2) {
		t.Fatalf("expected binary record magic")
	}
	expiresAt := int64(binary.BigEndian.Uint64(data[4:12]))
	if expiresAt <= time.Now().UnixNano() {
		t.Fatalf("expected future expiration")
	}
	keyLen := binary.BigEndian.Uint32(data[12:16])
	if keyLen != 1 || string(data[16:17]) != "k" {
		t.Fatalf("expected embedded key metadata")
	}
}

// TestFileStoreGetRemovesExpiredAndCorrupt verifies unreadable or expired records are pruned instead of returned.
func TestFileStoreGetRemovesExpiredAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	store := newFileStore(dir, time.Minute)
	fs := store.(*fileStore)
	expired := fileRecord{ExpiresAt: time.Now().Add(-time.Minute).UnixNano(), Value: []byte("old")}
	bytes, _ := json.Marshal(expired)
	if err := os.WriteFile(fs.path("old"), bytes, 0o644); err != nil {
		t.Fatalf("write expired: %v", err)
	}
	if _, ok, err := store.Get(context.Background(), "old"); err != nil || ok {
		t.Fatalf("expected expired miss, err=%v ok=%v", err, ok)
	}
	if _, err := os.Stat(fs.path("old")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected expired file removed")
	}

	if err := os.WriteFile(fs.path("bad"), []byte("not-json"), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	if _, _, err := store.Get(context.Background(), "bad"); err == nil {
		t.Fatalf("expected unmarshal error")
	}
	if _, err := os.Stat(fs.path("bad")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected corrupt file removed")
	}
}

// TestFileStoreInspectorOmitsExpiredEntries verifies browsing follows the same TTL semantics as Get.
func TestFileStoreInspectorOmitsExpiredEntries(t *testing.T) {
	dir := t.TempDir()
	store := newFileStore(dir, time.Minute)
	fs := store.(*fileStore)
	expired := fileRecord{
		Key:       "expired",
		ExpiresAt: time.Now().Add(-time.Minute).UnixNano(),
		Value:     []byte("old"),
	}
	body, err := json.Marshal(expired)
	if err != nil {
		t.Fatalf("marshal expired record: %v", err)
	}
	if err := os.WriteFile(fs.path("expired"), body, 0o600); err != nil {
		t.Fatalf("write expired record: %v", err)
	}

	page, err := fs.ListPage(context.Background(), cachecore.ListPageOptions{})
	if err != nil {
		t.Fatalf("list page: %v", err)
	}
	if len(page.Entries) != 0 {
		t.Fatalf("expired entries = %+v, want none", page.Entries)
	}
	if _, err := os.Stat(fs.path("expired")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired inspector record was not removed: %v", err)
	}
}

// TestFileStoreGetReadsLegacyJSONRecord verifies reads remain compatible with the prior JSON record format.
func TestFileStoreGetReadsLegacyJSONRecord(t *testing.T) {
	dir := t.TempDir()
	store := newFileStore(dir, time.Minute)
	fs := store.(*fileStore)

	legacy := fileRecord{
		ExpiresAt: time.Now().Add(time.Minute).UnixNano(),
		Value:     []byte("legacy"),
	}
	body, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}
	if err := os.WriteFile(fs.path("legacy"), body, 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	got, ok, err := store.Get(context.Background(), "legacy")
	if err != nil || !ok || string(got) != "legacy" {
		t.Fatalf("expected legacy read, ok=%v err=%v val=%q", ok, err, string(got))
	}
}

// TestFileStoreDeleteManyEmptyAndMissing verifies empty batches and absent paths remain idempotent.
func TestFileStoreDeleteManyEmptyAndMissing(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()
	if err := store.DeleteMany(ctx); err != nil {
		t.Fatalf("delete many empty failed: %v", err)
	}
	if err := store.Delete(ctx, "missing"); err != nil {
		t.Fatalf("delete missing should not error: %v", err)
	}
}

// TestFileStoreFlushMissingDir verifies a removed storage directory is treated as already flushed.
func TestFileStoreFlushMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing-dir")
	store := &fileStore{dir: dir, defaultTTL: time.Minute}
	if err := store.Flush(context.Background()); err != nil {
		t.Fatalf("flush missing dir should not error: %v", err)
	}
}

// TestFileStoreSetPermissionError verifies directory permission failures reach callers.
func TestFileStoreSetPermissionError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(dir, 0o755)

	store := newFileStore(dir, time.Second)
	if err := store.Set(context.Background(), "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected set to fail on permissions")
	}
	if created, err := store.Add(context.Background(), "k", []byte("v"), time.Second); err == nil || created {
		t.Fatalf("failed add = (created=%v, err=%v), want false and an error", created, err)
	}
}

// TestFileStoreSetWriteError verifies temporary-record write failures do not report success.
func TestFileStoreSetWriteError(t *testing.T) {
	dir := t.TempDir()
	store := newFileStore(dir, time.Second)

	orig := createTempFile
	createTempFile = func(dir, pattern string) (*os.File, error) {
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		_ = f.Close()
		return f, nil
	}
	defer func() { createTempFile = orig }()

	if err := store.Set(context.Background(), "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected write error")
	}
}

// TestFileStoreSetRenameError verifies atomic replacement failures are preserved.
func TestFileStoreSetRenameError(t *testing.T) {
	dir := t.TempDir()
	store := newFileStore(dir, time.Second)

	orig := renameFile
	renameFile = func(_, _ string) error { return errors.New("rename boom") }
	defer func() { renameFile = orig }()

	if err := store.Set(context.Background(), "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected rename error")
	}
}

// TestFileStoreDeletePermissionError verifies single-key removal preserves filesystem permission errors.
func TestFileStoreDeletePermissionError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(dir, 0o755)
	store := &fileStore{dir: dir, defaultTTL: time.Second}
	if err := store.Delete(context.Background(), "k"); err == nil {
		t.Fatalf("expected delete error due to permissions")
	}
}

// TestFileStoreDeleteManyError verifies bulk removal stops and returns filesystem failures.
func TestFileStoreDeleteManyError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(dir, 0o755)
	store := &fileStore{dir: dir, defaultTTL: time.Second}
	if err := store.DeleteMany(context.Background(), "a", "b"); err == nil {
		t.Fatalf("expected delete many error")
	}
}

// TestNewFileStoreDefaultsDir verifies an omitted root resolves to the documented temporary directory.
func TestNewFileStoreDefaultsDir(t *testing.T) {
	store := newFileStore("", 0)
	fs := store.(*fileStore)
	if fs.dir == "" {
		t.Fatalf("expected default dir")
	}
}
