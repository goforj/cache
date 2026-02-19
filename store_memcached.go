package cache

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type memcachedStore struct {
	addrs      []string
	defaultTTL time.Duration
	prefix     string
}

func newMemcachedStore(addrs []string, defaultTTL time.Duration, prefix string) Store {
	if len(addrs) == 0 {
		addrs = []string{"127.0.0.1:11211"}
	}
	if defaultTTL <= 0 {
		defaultTTL = defaultCacheTTL
	}
	if prefix == "" {
		prefix = defaultCachePrefix
	}
	return &memcachedStore{addrs: addrs, defaultTTL: defaultTTL, prefix: prefix}
}

func (s *memcachedStore) Driver() Driver { return DriverMemcached }

func (s *memcachedStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	conn, err := s.dial(ctx)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()

	full := s.cacheKey(key)
	if _, err := fmt.Fprintf(conn, "get %s\r\n", full); err != nil {
		return nil, false, err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, false, err
	}
	if line == "END\r\n" {
		return nil, false, nil
	}

	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 4 || fields[0] != "VALUE" {
		return nil, false, fmt.Errorf("unexpected response: %s", strings.TrimSpace(line))
	}
	bytesLen, err := strconv.Atoi(fields[3])
	if err != nil {
		return nil, false, fmt.Errorf("parse length: %w", err)
	}
	value := make([]byte, bytesLen)
	if _, err := reader.Read(value); err != nil {
		return nil, false, err
	}
	// consume trailing \r\n
	if _, err := reader.ReadString('\n'); err != nil {
		return nil, false, err
	}
	// consume END
	if _, err := reader.ReadString('\n'); err != nil {
		return nil, false, err
	}
	return cloneBytes(value), true, nil
}

func (s *memcachedStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	conn, err := s.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	full := s.cacheKey(key)
	seconds := int(ttl.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	if _, err := fmt.Fprintf(conn, "set %s 0 %d %d\r\n", full, seconds, len(value)); err != nil {
		return err
	}
	if _, err := conn.Write(value); err != nil {
		return err
	}
	if _, err := conn.Write([]byte("\r\n")); err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "STORED") {
		return fmt.Errorf("memcached set failed: %s", strings.TrimSpace(line))
	}
	return nil
}

func (s *memcachedStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	conn, err := s.dial(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	full := s.cacheKey(key)
	seconds := int(ttl.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	if _, err := fmt.Fprintf(conn, "add %s 0 %d %d\r\n", full, seconds, len(value)); err != nil {
		return false, err
	}
	if _, err := conn.Write(value); err != nil {
		return false, err
	}
	if _, err := conn.Write([]byte("\r\n")); err != nil {
		return false, err
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	switch {
	case strings.HasPrefix(line, "STORED"):
		return true, nil
	case strings.HasPrefix(line, "NOT_STORED"):
		return false, nil
	default:
		return false, fmt.Errorf("memcached add failed: %s", strings.TrimSpace(line))
	}
}

func (s *memcachedStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	// memcached incr/decr only accepts uint64; we emulate negative via decr when delta<0
	if delta < 0 {
		return s.decrement(ctx, key, -delta, ttl)
	}
	return s.incr(ctx, key, delta, ttl, "incr")
}

func (s *memcachedStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if delta < 0 {
		return s.Increment(ctx, key, -delta, ttl)
	}
	return s.decrement(ctx, key, delta, ttl)
}

func (s *memcachedStore) decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.incr(ctx, key, delta, ttl, "decr")
}

func (s *memcachedStore) incr(ctx context.Context, key string, delta int64, ttl time.Duration, verb string) (int64, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	conn, err := s.dial(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	full := s.cacheKey(key)
	if _, err := fmt.Fprintf(conn, "%s %s %d\r\n", verb, full, delta); err != nil {
		return 0, err
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	line = strings.TrimSpace(line)
	if line == "NOT_FOUND" {
		// Initialize value then retry.
		if err := s.Set(ctx, key, []byte("0"), ttl); err != nil {
			return 0, err
		}
		return s.incr(ctx, key, delta, ttl, verb)
	}
	if strings.HasPrefix(line, "ERROR") {
		return 0, errors.New(line)
	}
	val, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

func (s *memcachedStore) Delete(_ context.Context, key string) error {
	conn, err := s.dial(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	full := s.cacheKey(key)
	if _, err := fmt.Fprintf(conn, "delete %s\r\n", full); err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	_, _ = reader.ReadString('\n') // DELETED or NOT_FOUND; both considered success
	return nil
}

func (s *memcachedStore) DeleteMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *memcachedStore) Flush(_ context.Context) error {
	conn, err := s.dial(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "flush_all\r\n"); err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "OK") {
		return fmt.Errorf("memcached flush failed: %s", strings.TrimSpace(line))
	}
	return nil
}

func (s *memcachedStore) dial(ctx context.Context) (net.Conn, error) {
	if len(s.addrs) == 0 {
		return nil, errors.New("memcached: no addresses configured")
	}
	var errs bytes.Buffer
	for _, addr := range s.addrs {
		d := net.Dialer{Timeout: 3 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			return conn, nil
		}
		fmt.Fprintf(&errs, "%s: %v; ", addr, err)
	}
	return nil, fmt.Errorf("memcached dial failed: %s", errs.String())
}

func (s *memcachedStore) cacheKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return s.prefix + ":" + key
}
