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
)

var (
	createTempFile = os.CreateTemp
	renameFile     = os.Rename
)

var fileRecordMagic = []byte("CFR1")

type fileRecord struct {
	ExpiresAt int64  `json:"expires_at"`
	Value     []byte `json:"value"`
}

type fileStore struct {
	dir        string
	defaultTTL time.Duration
}

func newFileStore(dir string, defaultTTL time.Duration) Store {
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

func (s *fileStore) Driver() Driver {
	return DriverFile
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

	expiresAt, value, err := decodeFileRecord(data)
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

	var header [12]byte
	copy(header[:4], fileRecordMagic)
	binary.BigEndian.PutUint64(header[4:], uint64(expiresAt))

	if _, err := tmp.Write(header[:]); err != nil {
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

func (s *fileStore) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(s.dir, name+".cache")
}

func decodeFileRecord(data []byte) (int64, []byte, error) {
	if len(data) >= 12 && bytes.Equal(data[:4], fileRecordMagic) {
		expiresAt := int64(binary.BigEndian.Uint64(data[4:12]))
		return expiresAt, data[12:], nil
	}

	var rec fileRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return 0, nil, err
	}
	return rec.ExpiresAt, rec.Value, nil
}
