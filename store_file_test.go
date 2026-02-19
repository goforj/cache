package cache

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTempFileStore(t *testing.T) Store {
	t.Helper()
	dir := t.TempDir()
	return newFileStore(dir, 0)
}

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

func TestFileStoreAddDefaultsTTL(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()
	created, err := store.Add(ctx, "x", []byte("1"), 0)
	if err != nil || !created {
		t.Fatalf("add failed: %v created=%v", err, created)
	}
}

func TestFileStoreFlushEmpty(t *testing.T) {
	store := newTempFileStore(t)
	ctx := context.Background()
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush empty failed: %v", err)
	}
}

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
	var rec fileRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rec.ExpiresAt <= time.Now().UnixNano() {
		t.Fatalf("expected future expiration")
	}
}

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

func TestFileStoreFlushMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing-dir")
	store := &fileStore{dir: dir, defaultTTL: time.Minute}
	if err := store.Flush(context.Background()); err != nil {
		t.Fatalf("flush missing dir should not error: %v", err)
	}
}

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
}

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

func TestNewFileStoreDefaultsDir(t *testing.T) {
	store := newFileStore("", 0)
	fs := store.(*fileStore)
	if fs.dir == "" {
		t.Fatalf("expected default dir")
	}
}
