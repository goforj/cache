package memcachedcache

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/goforj/cache/cachecore"
)

const (
	defaultTTL    = 5 * time.Minute
	defaultPrefix = "app"
)

var dialMemcached = func(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 3 * time.Second}
	return d.DialContext(ctx, network, addr)
}

// Config configures a Memcached-backed cache store.
type Config struct {
	cachecore.BaseConfig
	Addresses []string
}

type store struct {
	addrs      []string
	defaultTTL time.Duration
	prefix     string
	pools      map[string]chan *memcachedConn
	rr         uint32
}

type memcachedConn struct {
	addr   string
	conn   net.Conn
	reader *bufio.Reader
}

// New builds a Memcached-backed cachecore.Store.
//
// Defaults:
// - Addresses: []string{"127.0.0.1:11211"} when empty
// - DefaultTTL: 5*time.Minute when zero
// - Prefix: "app" when empty
//
// Example: memcached cluster via explicit driver config
//
//	store := memcachedcache.New(memcachedcache.Config{
//		BaseConfig: cachecore.BaseConfig{
//			DefaultTTL: 5 * time.Minute,
//			Prefix:     "app",
//		},
//		Addresses: []string{"127.0.0.1:11211"},
//	})
//	fmt.Println(store.Driver()) // memcached
func New(cfg Config) cachecore.Store {
	addrs := cfg.Addresses
	if len(addrs) == 0 {
		addrs = []string{"127.0.0.1:11211"}
	}
	ttl := cfg.DefaultTTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = defaultPrefix
	}
	pools := make(map[string]chan *memcachedConn, len(addrs))
	for _, addr := range addrs {
		pools[addr] = make(chan *memcachedConn, 16)
	}
	return &store{addrs: addrs, defaultTTL: ttl, prefix: prefix, pools: pools}
}

func (s *store) Driver() cachecore.Driver { return cachecore.DriverMemcached }

func (s *store) Ready(ctx context.Context) error {
	mc, err := s.acquire(ctx)
	if err != nil {
		return err
	}
	bad := false
	defer func() { s.release(mc, bad) }()
	if _, err := fmt.Fprintf(mc.conn, "version\r\n"); err != nil {
		bad = true
		return err
	}
	line, err := mc.reader.ReadString('\n')
	if err != nil {
		bad = true
		return err
	}
	if !strings.HasPrefix(line, "VERSION ") {
		bad = true
		return fmt.Errorf("memcached readiness failed: %s", strings.TrimSpace(line))
	}
	return nil
}

func (s *store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	mc, err := s.acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	bad := false
	defer func() { s.release(mc, bad) }()

	full := s.cacheKey(key)
	if _, err := fmt.Fprintf(mc.conn, "get %s\r\n", full); err != nil {
		bad = true
		return nil, false, err
	}
	line, err := mc.reader.ReadString('\n')
	if err != nil {
		bad = true
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
	if _, err := io.ReadFull(mc.reader, value); err != nil {
		bad = true
		return nil, false, err
	}
	if _, err := mc.reader.ReadString('\n'); err != nil { // trailing CRLF
		bad = true
		return nil, false, err
	}
	if _, err := mc.reader.ReadString('\n'); err != nil { // END
		bad = true
		return nil, false, err
	}
	return cloneBytes(value), true, nil
}

func (s *store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	mc, err := s.acquire(ctx)
	if err != nil {
		return err
	}
	bad := false
	defer func() { s.release(mc, bad) }()

	full := s.cacheKey(key)
	seconds := int(ttl.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	if _, err := fmt.Fprintf(mc.conn, "set %s 0 %d %d\r\n", full, seconds, len(value)); err != nil {
		bad = true
		return err
	}
	if _, err := mc.conn.Write(value); err != nil {
		bad = true
		return err
	}
	if _, err := mc.conn.Write([]byte("\r\n")); err != nil {
		bad = true
		return err
	}
	line, err := mc.reader.ReadString('\n')
	if err != nil {
		bad = true
		return err
	}
	if !strings.HasPrefix(line, "STORED") {
		bad = true
		return fmt.Errorf("memcached set failed: %s", strings.TrimSpace(line))
	}
	return nil
}

func (s *store) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	mc, err := s.acquire(ctx)
	if err != nil {
		return false, err
	}
	bad := false
	defer func() { s.release(mc, bad) }()

	full := s.cacheKey(key)
	seconds := int(ttl.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	if _, err := fmt.Fprintf(mc.conn, "add %s 0 %d %d\r\n", full, seconds, len(value)); err != nil {
		bad = true
		return false, err
	}
	if _, err := mc.conn.Write(value); err != nil {
		bad = true
		return false, err
	}
	if _, err := mc.conn.Write([]byte("\r\n")); err != nil {
		bad = true
		return false, err
	}
	line, err := mc.reader.ReadString('\n')
	if err != nil {
		bad = true
		return false, err
	}
	switch {
	case strings.HasPrefix(line, "STORED"):
		return true, nil
	case strings.HasPrefix(line, "NOT_STORED"):
		return false, nil
	default:
		bad = true
		return false, fmt.Errorf("memcached add failed: %s", strings.TrimSpace(line))
	}
}

func (s *store) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if delta < 0 {
		return s.decrement(ctx, key, -delta, ttl)
	}
	return s.incr(ctx, key, delta, ttl, "incr")
}

func (s *store) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if delta < 0 {
		return s.Increment(ctx, key, -delta, ttl)
	}
	return s.decrement(ctx, key, delta, ttl)
}

func (s *store) decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.incr(ctx, key, delta, ttl, "decr")
}

func (s *store) incr(ctx context.Context, key string, delta int64, ttl time.Duration, verb string) (int64, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	mc, err := s.acquire(ctx)
	if err != nil {
		return 0, err
	}
	bad := false
	defer func() { s.release(mc, bad) }()

	full := s.cacheKey(key)
	if _, err := fmt.Fprintf(mc.conn, "%s %s %d\r\n", verb, full, delta); err != nil {
		bad = true
		return 0, err
	}
	line, err := mc.reader.ReadString('\n')
	if err != nil {
		bad = true
		return 0, err
	}
	line = strings.TrimSpace(line)
	if line == "NOT_FOUND" {
		if err := s.Set(ctx, key, []byte("0"), ttl); err != nil {
			return 0, err
		}
		return s.incr(ctx, key, delta, ttl, verb)
	}
	if strings.HasPrefix(line, "ERROR") {
		bad = true
		return 0, errors.New(line)
	}
	val, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		bad = true
		return 0, err
	}
	if ttl > 0 {
		seconds := int(ttl.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		if _, err := fmt.Fprintf(mc.conn, "touch %s %d\r\n", full, seconds); err != nil {
			bad = true
			return 0, err
		}
		touchLine, err := mc.reader.ReadString('\n')
		if err != nil {
			bad = true
			return 0, err
		}
		if !strings.HasPrefix(touchLine, "TOUCHED") && !strings.HasPrefix(touchLine, "NOT_FOUND") {
			bad = true
			return 0, fmt.Errorf("memcached touch failed: %s", strings.TrimSpace(touchLine))
		}
	}
	return val, nil
}

func (s *store) Delete(ctx context.Context, key string) error {
	mc, err := s.acquire(ctx)
	if err != nil {
		return err
	}
	bad := false
	defer func() { s.release(mc, bad) }()
	if _, err := fmt.Fprintf(mc.conn, "delete %s\r\n", s.cacheKey(key)); err != nil {
		bad = true
		return err
	}
	if _, err := mc.reader.ReadString('\n'); err != nil {
		bad = true
		return err
	}
	return nil
}

func (s *store) DeleteMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) Flush(ctx context.Context) error {
	mc, err := s.acquire(ctx)
	if err != nil {
		return err
	}
	bad := false
	defer func() { s.release(mc, bad) }()
	if _, err := fmt.Fprintf(mc.conn, "flush_all\r\n"); err != nil {
		bad = true
		return err
	}
	line, err := mc.reader.ReadString('\n')
	if err != nil {
		bad = true
		return err
	}
	if !strings.HasPrefix(line, "OK") {
		bad = true
		return fmt.Errorf("memcached flush failed: %s", strings.TrimSpace(line))
	}
	return nil
}

func (s *store) acquire(ctx context.Context) (*memcachedConn, error) {
	if len(s.addrs) == 0 {
		return nil, errors.New("memcached: no addresses configured")
	}
	var errs bytes.Buffer
	start := int(atomic.AddUint32(&s.rr, 1)-1) % len(s.addrs)
	for i := 0; i < len(s.addrs); i++ {
		addr := s.addrs[(start+i)%len(s.addrs)]
		if pool, ok := s.pools[addr]; ok {
			select {
			case mc := <-pool:
				if mc != nil {
					return mc, nil
				}
			default:
			}
		}
		conn, err := dialMemcached(ctx, "tcp", addr)
		if err == nil {
			return &memcachedConn{addr: addr, conn: conn, reader: bufio.NewReader(conn)}, nil
		}
		fmt.Fprintf(&errs, "%s: %v; ", addr, err)
	}
	return nil, fmt.Errorf("memcached dial failed: %s", errs.String())
}

func (s *store) release(mc *memcachedConn, bad bool) {
	if mc == nil || mc.conn == nil {
		return
	}
	if bad {
		_ = mc.conn.Close()
		return
	}
	pool, ok := s.pools[mc.addr]
	if !ok {
		_ = mc.conn.Close()
		return
	}
	select {
	case pool <- mc:
	default:
		_ = mc.conn.Close()
	}
}

func (s *store) cacheKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return s.prefix + ":" + key
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	out := make([]byte, len(value))
	copy(out, value)
	return out
}
