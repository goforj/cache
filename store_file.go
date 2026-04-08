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
	"time"

	"github.com/goforj/cache/cachecore"
)

var (
	createTempFile = os.CreateTemp
	renameFile     = os.Rename
)

var fileRecordMagic = []byte("CFR1")
var fileRecordMagicV2 = []byte("CFR2")

type fileRecord struct {
	Key       string `json:"key,omitempty"`
	ExpiresAt int64  `json:"expires_at"`
	Value     []byte `json:"value"`
}

type fileStore struct {
	dir        string
	defaultTTL time.Duration
}

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
	}
}

func (s *fileStore) Driver() cachecore.Driver {
	return cachecore.DriverFile
}

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

func (s *fileStore) Get(_ context.Context, key string) ([]byte, bool, error) {
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

func (s *fileStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	expiresAt := time.Now().Add(ttl).UnixNano()

	tmp, err := createTempFile(s.dir, "cache-*")
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
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return renameFile(tmpPath, s.path(key))
}

func (s *fileStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	_, ok, err := s.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}
	return true, s.Set(ctx, key, value, ttl)
}

func (s *fileStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	current := int64(0)
	if body, ok, err := s.Get(ctx, key); err != nil {
		return 0, err
	} else if ok {
		n, err := strconv.ParseInt(string(body), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cache key %q does not contain a numeric value", key)
		}
		current = n
	}
	next := current + delta
	if err := s.Set(ctx, key, []byte(strconv.FormatInt(next, 10)), ttl); err != nil {
		return 0, err
	}
	return next, nil
}

func (s *fileStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

func (s *fileStore) Delete(_ context.Context, key string) error {
	if err := os.Remove(s.path(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *fileStore) DeleteMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *fileStore) Flush(_ context.Context) error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		_ = os.Remove(filepath.Join(s.dir, entry.Name()))
	}
	return nil
}

func (s *fileStore) Capabilities() cachecore.InspectorCapabilities {
	return cachecore.InspectorCapabilities{
		CanList:   true,
		CanRead:   true,
		CanDelete: true,
		CanTTL:    true,
	}
}

func (s *fileStore) ListPage(_ context.Context, opts cachecore.ListPageOptions) (cachecore.ListPageResult, error) {
	dirEntries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cachecore.ListPageResult{}, nil
		}
		return cachecore.ListPageResult{}, err
	}
	entries := make([]cachecore.CacheEntry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
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
		var expiresAt *int64
		if expiresAtValue > 0 {
			exp := expiresAtValue
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

func (s *fileStore) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(s.dir, name+".cache")
}

func decodeFileRecord(data []byte) (string, int64, []byte, error) {
	if len(data) >= 16 && bytes.Equal(data[:4], fileRecordMagicV2) {
		expiresAt := int64(binary.BigEndian.Uint64(data[4:12]))
		keyLen := int(binary.BigEndian.Uint32(data[12:16]))
		offset := 16
		if keyLen < 0 || len(data) < offset+keyLen {
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
