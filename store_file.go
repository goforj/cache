package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goforj/cache/cachecore"
)

var (
	createTempFile = os.CreateTemp
	renameFile     = os.Rename
)

var fileRecordMagic = []byte("CFR1")
var fileRecordMagicV2 = []byte("CFR2")

// fileStoreMutationLocks bound process-local coordination while ensuring equal directories share a lock.
var fileStoreMutationLocks [64]sync.Mutex

type fileRecord struct {
	Key       string `json:"key,omitempty"`
	ExpiresAt int64  `json:"expires_at"`
	Value     []byte `json:"value"`
}

type fileStore struct {
	dir        string
	defaultTTL time.Duration
	mu         *sync.Mutex
}

// newFileStore constructs a process-local filesystem backend with normalized defaults.
func newFileStore(dir string, defaultTTL time.Duration) cachecore.Store {
	if dir == "" {
		dir = defaultFileDir()
	}
	if defaultTTL <= 0 {
		defaultTTL = defaultCacheTTL
	}
	_ = os.MkdirAll(dir, 0o755)
	return &fileStore{
		dir:        dir,
		defaultTTL: defaultTTL,
		mu:         fileStoreMutationLock(dir),
	}
}

// fileStoreMutationLock maps equivalent directory paths to a bounded process-wide lock stripe.
func fileStoreMutationLock(dir string) *sync.Mutex {
	dir = filepath.Clean(dir)
	if absolute, err := filepath.Abs(dir); err == nil {
		dir = absolute
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	sum := sha256.Sum256([]byte(dir))
	return &fileStoreMutationLocks[int(sum[0])%len(fileStoreMutationLocks)]
}

// mutationLock supports directly assembled stores while normal constructors cache the shared lock.
func (s *fileStore) mutationLock() *sync.Mutex {
	if s.mu != nil {
		return s.mu
	}
	return fileStoreMutationLock(s.dir)
}

// Driver identifies the backend for diagnostics and capability-specific behavior.
func (s *fileStore) Driver() cachecore.Driver {
	return cachecore.DriverFile
}

// Ready verifies that the backend can serve cache operations.
func (s *fileStore) Ready(_ context.Context) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	info, err := os.Stat(s.dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("cache file store path %q is not a directory", s.dir)
	}
	return nil
}

// Get returns an owned copy of a stored value and distinguishes misses from failures.
func (s *fileStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	mu := s.mutationLock()
	mu.Lock()
	defer mu.Unlock()
	return s.get(key)
}

// get reads one complete atomic record while the process-local mutation lock is held.
func (s *fileStore) get(key string) ([]byte, bool, error) {
	path := s.path(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	_, expiresAt, value, err := decodeFileRecord(data)
	if err != nil {
		_ = os.Remove(path)
		return nil, false, err
	}

	if expiresAt > 0 && time.Now().UnixNano() > expiresAt {
		_ = os.Remove(path)
		return nil, false, nil
	}

	return value, true, nil
}

// Set stores an owned copy of a value using the requested or default TTL.
func (s *fileStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	mu := s.mutationLock()
	mu.Lock()
	defer mu.Unlock()
	return s.set(key, value, ttl)
}

// set uses fsync and rename so readers never observe a partially written record.
func (s *fileStore) set(key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	expiresAt := time.Now().Add(ttl).UnixNano()

	tmp, err := createTempFile(s.dir, ".cache-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	keyBytes := []byte(key)
	recordLen := 16 + len(keyBytes)
	header := make([]byte, recordLen)
	copy(header[:4], fileRecordMagicV2)
	binary.BigEndian.PutUint64(header[4:12], uint64(expiresAt))
	binary.BigEndian.PutUint32(header[12:16], uint32(len(keyBytes)))
	copy(header[16:], keyBytes)

	if _, err := tmp.Write(header); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if _, err := tmp.Write(value); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := renameFile(tmpPath, s.path(key)); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// Add stores a value only when the key is currently absent.
func (s *fileStore) Add(_ context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	mu := s.mutationLock()
	mu.Lock()
	defer mu.Unlock()
	_, ok, err := s.get(key)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}
	if err := s.set(key, value, ttl); err != nil {
		return false, err
	}
	return true, nil
}

// Increment atomically adds delta while preserving the store's TTL contract.
func (s *fileStore) Increment(_ context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	mu := s.mutationLock()
	mu.Lock()
	defer mu.Unlock()
	current := int64(0)
	if body, ok, err := s.get(key); err != nil {
		return 0, err
	} else if ok {
		n, err := strconv.ParseInt(string(body), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cache key %q does not contain a numeric value", key)
		}
		current = n
	}
	next := current + delta
	if err := s.set(key, []byte(strconv.FormatInt(next, 10)), ttl); err != nil {
		return 0, err
	}
	return next, nil
}

// Decrement atomically subtracts delta while preserving the store's TTL contract.
func (s *fileStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

// Delete removes a key and treats an existing miss as success.
func (s *fileStore) Delete(_ context.Context, key string) error {
	mu := s.mutationLock()
	mu.Lock()
	defer mu.Unlock()
	return s.delete(key)
}

// delete treats a missing cache file as an idempotent success.
func (s *fileStore) delete(key string) error {
	if err := os.Remove(s.path(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// DeleteMany removes every requested key under the store's namespace.
func (s *fileStore) DeleteMany(_ context.Context, keys ...string) error {
	mu := s.mutationLock()
	mu.Lock()
	defer mu.Unlock()
	for _, key := range keys {
		if err := s.delete(key); err != nil {
			return err
		}
	}
	return nil
}

// Flush removes entries within the store's configured scope.
func (s *fileStore) Flush(_ context.Context) error {
	mu := s.mutationLock()
	mu.Lock()
	defer mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isFileStoreEntry(name) {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// isFileStoreEntry recognizes only hashed records and the temporary files used to build them.
func isFileStoreEntry(name string) bool {
	if strings.HasPrefix(name, ".cache-tmp-") {
		return true
	}
	if len(name) != sha256.Size*2+len(".cache") || !strings.HasSuffix(name, ".cache") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimSuffix(name, ".cache"))
	return err == nil
}

// Capabilities reports the optional inspection operations supported by the store.
func (s *fileStore) Capabilities() cachecore.InspectorCapabilities {
	return cachecore.InspectorCapabilities{
		CanList:   true,
		CanRead:   true,
		CanDelete: true,
		CanTTL:    true,
	}
}

// ListPage returns a filtered, deterministic page of inspectable cache entries.
func (s *fileStore) ListPage(_ context.Context, opts cachecore.ListPageOptions) (cachecore.ListPageResult, error) {
	mu := s.mutationLock()
	mu.Lock()
	defer mu.Unlock()
	dirEntries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cachecore.ListPageResult{}, nil
		}
		return cachecore.ListPageResult{}, err
	}
	entries := make([]cachecore.CacheEntry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() || !isFileStoreRecord(dirEntry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, dirEntry.Name()))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return cachecore.ListPageResult{}, err
		}
		key, expiresAtValue, value, err := decodeFileRecord(data)
		if err != nil {
			continue
		}
		if key == "" {
			continue
		}
		if expiresAtValue > 0 && time.Now().UnixNano() >= expiresAtValue {
			_ = os.Remove(filepath.Join(s.dir, dirEntry.Name()))
			continue
		}
		var expiresAt *int64
		if expiresAtValue > 0 {
			exp := time.Unix(0, expiresAtValue).UnixMilli()
			expiresAt = &exp
		}
		entries = append(entries, cachecore.CacheEntry{
			Key:       key,
			SizeBytes: len(value),
			ExpiresAt: expiresAt,
		})
	}
	filtered := cachecore.FilterAndSortEntries(entries, cachecore.ListFilterTerm(opts))
	offset, err := cachecore.DecodeOffsetCursor(opts.Cursor)
	if err != nil {
		return cachecore.ListPageResult{}, err
	}
	return cachecore.SliceEntries(filtered, offset, opts.Limit), nil
}

// isFileStoreRecord recognizes the hashed filenames used for committed cache records.
func isFileStoreRecord(name string) bool {
	return !strings.HasPrefix(name, ".cache-tmp-") && isFileStoreEntry(name)
}

// path hashes logical keys so arbitrary input cannot escape the configured directory.
func (s *fileStore) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(s.dir, name+".cache")
}

// decodeFileRecord preserves reads of both binary generations and the legacy JSON format.
func decodeFileRecord(data []byte) (string, int64, []byte, error) {
	if len(data) >= 16 && bytes.Equal(data[:4], fileRecordMagicV2) {
		expiresAt := int64(binary.BigEndian.Uint64(data[4:12]))
		keyLen := int(binary.BigEndian.Uint32(data[12:16]))
		offset := 16
		if keyLen < 0 || keyLen > len(data)-offset {
			return "", 0, nil, errors.New("invalid cache file record")
		}
		key := string(data[offset : offset+keyLen])
		return key, expiresAt, data[offset+keyLen:], nil
	}
	if len(data) >= 12 && bytes.Equal(data[:4], fileRecordMagic) {
		expiresAt := int64(binary.BigEndian.Uint64(data[4:12]))
		return "", expiresAt, data[12:], nil
	}

	var rec fileRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return "", 0, nil, err
	}
	return rec.Key, rec.ExpiresAt, rec.Value, nil
}
