package memcachedcache

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goforj/cache/cachecore"
)

const (
	defaultTTL                = 5 * time.Minute
	defaultPrefix             = "app"
	expiredMemcachedTimestamp = 30*24*60*60 + 1
	ioTimeout                 = 3 * time.Second
)

var dialMemcached = func(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 3 * time.Second}
	return d.DialContext(ctx, network, addr)
}

// Config configures a Memcached-backed cache store.
type Config struct {
	cachecore.BaseConfig
	// Addresses identifies the peers used for deterministic key routing.
	Addresses []string
	// AllowServerFlush enables Flush on dedicated servers. It clears every prefix on every peer.
	AllowServerFlush bool
}

type store struct {
	addrs            []string
	defaultTTL       time.Duration
	prefix           string
	pools            map[string]chan *memcachedConn
	allowServerFlush bool
	mu               sync.Mutex
	closed           bool
}

type memcachedConn struct {
	addr   string
	conn   net.Conn
	reader *bufio.Reader
}

// New builds a lazy Memcached store that owns its connections. The caller can
// release them through io.Closer. Addresses are treated as a fixed peer set.
// A failed peer returns an error for its keys so locks never fail over to another peer.
//
// Defaults:
// - Addresses: []string{"127.0.0.1:11211"} when empty
// - DefaultTTL: 5*time.Minute when zero
// - Prefix: "app" when empty
// - AllowServerFlush: false, unless every server is dedicated to this cache
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
	addrs := slices.Clone(cfg.Addresses)
	slices.Sort(addrs)
	addrs = slices.Compact(addrs)
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
	backend := &store{addrs: addrs, defaultTTL: ttl, prefix: prefix, pools: pools, allowServerFlush: cfg.AllowServerFlush}
	wrapped, _ := cachecore.WrapStore(backend, cfg.BaseConfig)
	return wrapped
}

// Driver identifies the backend for diagnostics and capability-specific behavior.
func (s *store) Driver() cachecore.Driver { return cachecore.DriverMemcached }

// Ready verifies that the backend can serve cache operations.
func (s *store) Ready(ctx context.Context) error {
	for _, addr := range s.addrs {
		if err := s.serverCommand(ctx, addr, "version", "VERSION "); err != nil {
			return err
		}
	}
	return nil
}

// Get returns an owned copy of a stored value and distinguishes misses from failures.
func (s *store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	mc, release, err := s.acquireKey(ctx, key)
	if err != nil {
		return nil, false, err
	}
	bad := false
	defer func() { release(bad) }()

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
	if len(fields) != 4 || fields[0] != "VALUE" || fields[1] != full {
		bad = true
		return nil, false, fmt.Errorf("unexpected response: %s", strings.TrimSpace(line))
	}
	bytesLen, err := strconv.Atoi(fields[3])
	if err != nil || bytesLen < 0 {
		bad = true
		return nil, false, errors.New("invalid memcached value length")
	}
	value, err := readValue(mc.reader, bytesLen)
	if err != nil {
		bad = true
		return nil, false, err
	}

	return value, true, nil
}

// Set stores an owned copy of a value using the requested or default TTL.
func (s *store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	mc, release, err := s.acquireKey(ctx, key)
	if err != nil {
		return err
	}
	bad := false
	defer func() { release(bad) }()

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

// Add stores a value only when the key is currently absent.
func (s *store) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	mc, release, err := s.acquireKey(ctx, key)
	if err != nil {
		return false, err
	}
	bad := false
	defer func() { release(bad) }()

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

// LockAcquire atomically records owner when key is absent.
func (s *store) LockAcquire(ctx context.Context, key string, owner []byte, ttl time.Duration) (bool, error) {
	return s.Add(ctx, key, owner, ttl)
}

// LockRelease uses one Memcached CAS to expire only the current owner's value.
func (s *store) LockRelease(ctx context.Context, key string, owner []byte) (bool, error) {
	mc, release, err := s.acquireKey(ctx, key)
	if err != nil {
		return false, err
	}
	bad := false
	defer func() { release(bad) }()
	full := s.cacheKey(key)
	if _, err := fmt.Fprintf(mc.conn, "gets %s\r\n", full); err != nil {
		bad = true
		return false, err
	}
	line, err := mc.reader.ReadString('\n')
	if err != nil {
		bad = true
		return false, err
	}
	if line == "END\r\n" {
		return false, nil
	}
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 5 || fields[0] != "VALUE" || fields[1] != full {
		bad = true
		return false, fmt.Errorf("unexpected response: %s", strings.TrimSpace(line))
	}
	bytesLen, err := strconv.Atoi(fields[3])
	if err != nil || bytesLen < 0 {
		bad = true
		return false, errors.New("invalid memcached value length")
	}
	value, err := readValue(mc.reader, bytesLen)
	if err != nil {
		bad = true
		return false, err
	}

	if !bytes.Equal(value, owner) {
		return false, nil
	}
	if _, err := fmt.Fprintf(mc.conn, "cas %s 0 %d 0 %s\r\n\r\n", full, expiredMemcachedTimestamp, fields[4]); err != nil {
		bad = true
		return false, err
	}
	line, err = mc.reader.ReadString('\n')
	if err != nil {
		bad = true
		return false, err
	}
	if strings.HasPrefix(line, "EXISTS") || strings.HasPrefix(line, "NOT_FOUND") {
		return false, nil
	}
	if !strings.HasPrefix(line, "STORED") {
		bad = true
		return false, fmt.Errorf("memcached lock release failed: %s", strings.TrimSpace(line))
	}
	return true, nil
}

// Increment atomically adds delta while preserving the store's TTL contract.
func (s *store) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if delta < 0 {
		return s.decrement(ctx, key, -delta, ttl)
	}
	return s.incr(ctx, key, delta, ttl, "incr")
}

// Decrement atomically subtracts delta while preserving the store's TTL contract.
func (s *store) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	if delta < 0 {
		return s.Increment(ctx, key, -delta, ttl)
	}
	return s.decrement(ctx, key, delta, ttl)
}

// decrement routes Memcached subtraction through the shared counter protocol.
func (s *store) decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.incr(ctx, key, delta, ttl, "decr")
}

// incr implements both Memcached counter verbs and refreshes their expiration consistently.
func (s *store) incr(ctx context.Context, key string, delta int64, ttl time.Duration, verb string) (int64, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	mc, release, err := s.acquireKey(ctx, key)
	if err != nil {
		return 0, err
	}
	bad := false
	defer func() { release(bad) }()

	full := s.cacheKey(key)
	var line string
	for attempt := 0; ; attempt++ {
		if _, err := fmt.Fprintf(mc.conn, "%s %s %d\r\n", verb, full, delta); err != nil {
			bad = true
			return 0, err
		}
		response, err := mc.reader.ReadString('\n')
		if err != nil {
			bad = true
			return 0, err
		}
		line = strings.TrimSpace(response)
		if line != "NOT_FOUND" {
			break
		}
		if attempt == 3 {
			return 0, errors.New("memcached counter disappeared during initialization")
		}
		if _, err := s.Add(ctx, key, []byte("0"), ttl); err != nil {
			return 0, err
		}
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

// Delete removes a key and treats an existing miss as success.
func (s *store) Delete(ctx context.Context, key string) error {
	mc, release, err := s.acquireKey(ctx, key)
	if err != nil {
		return err
	}
	bad := false
	defer func() { release(bad) }()
	if _, err := fmt.Fprintf(mc.conn, "delete %s\r\n", s.cacheKey(key)); err != nil {
		bad = true
		return err
	}
	line, err := mc.reader.ReadString('\n')
	if err != nil {
		bad = true
		return err
	}
	if line != "DELETED\r\n" && line != "NOT_FOUND\r\n" {
		bad = true
		return fmt.Errorf("memcached delete failed: %s", strings.TrimSpace(line))
	}
	return nil
}

// DeleteMany removes every requested key under the store's namespace.
func (s *store) DeleteMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// Flush clears all configured peers when they are explicitly dedicated to this cache.
func (s *store) Flush(ctx context.Context) error {
	if !s.allowServerFlush {
		return errors.New("memcached cannot flush one prefix; enable AllowServerFlush only for dedicated servers")
	}
	var result error
	for _, addr := range s.addrs {
		result = errors.Join(result, s.serverCommand(ctx, addr, "flush_all", "OK\r\n"))
	}
	return result
}

// serverCommand executes a server-wide operation against one specific peer.
func (s *store) serverCommand(ctx context.Context, addr, command, expected string) error {
	mc, release, err := s.acquire(ctx, addr)
	if err != nil {
		return err
	}
	bad := true
	defer func() { release(bad) }()
	if _, err := fmt.Fprint(mc.conn, command+"\r\n"); err != nil {
		return err
	}
	line, err := mc.reader.ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, expected) {
		return fmt.Errorf("memcached %s failed on %s: %s", command, addr, strings.TrimSpace(line))
	}
	bad = false
	return nil
}

// acquireKey validates the wire key and routes every operation to the same peer.
func (s *store) acquireKey(ctx context.Context, key string) (*memcachedConn, func(bool), error) {
	full := s.cacheKey(key)
	if len(full) > 250 || strings.IndexFunc(full, func(r rune) bool { return r <= ' ' || r == 0x7f }) >= 0 {
		return nil, nil, errors.New("memcached key must fit within 250 bytes and contain no whitespace or control characters")
	}
	addr := s.addrs[int(uint64(crc32.ChecksumIEEE([]byte(full)))%uint64(len(s.addrs)))]
	return s.acquire(ctx, addr)
}

// acquire sets an operation deadline on each socket, including pooled sockets.
func (s *store) acquire(ctx context.Context, addr string) (*memcachedConn, func(bool), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	mc, err := s.takeConnection(ctx, addr)
	if err != nil {
		return nil, nil, err
	}
	deadline := time.Now().Add(ioTimeout)
	if sooner, ok := ctx.Deadline(); ok && sooner.Before(deadline) {
		deadline = sooner
	}
	if err := mc.conn.SetDeadline(deadline); err != nil {
		s.release(mc, true)
		return nil, nil, err
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = mc.conn.SetDeadline(time.Now())
		close(done)
	})
	return mc, func(bad bool) {
		if !stop() {
			<-done
			bad = true
		}
		s.release(mc, bad)
	}, nil
}

// takeConnection takes a socket from the selected peer or dials only that peer.
func (s *store) takeConnection(ctx context.Context, addr string) (*memcachedConn, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, net.ErrClosed
	}
	select {
	case mc := <-s.pools[addr]:
		s.mu.Unlock()
		return mc, nil
	default:
		s.mu.Unlock()
	}
	conn, err := dialMemcached(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("memcached dial %s: %w", addr, err)
	}
	return &memcachedConn{addr: addr, conn: conn, reader: bufio.NewReader(conn)}, nil
}

// release returns a healthy Memcached connection to its pool and closes unusable connections.
func (s *store) release(mc *memcachedConn, bad bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !bad && !s.closed {
		select {
		case s.pools[mc.addr] <- mc:
			return
		default:
		}
	}
	_ = mc.conn.Close()
}

// Close drains idle connections. In-flight operations close their sockets when they return.
func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for _, pool := range s.pools {
		for len(pool) > 0 {
			_ = (<-pool).conn.Close()
		}
	}
	return nil
}

// cacheKey applies the configured namespace before a key reaches the backend.
func (s *store) cacheKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return s.prefix + ":" + key
}

// readValue verifies the whole response before a connection can return to the pool.
func readValue(reader *bufio.Reader, size int) ([]byte, error) {
	value := bytes.NewBuffer([]byte{})
	if _, err := io.CopyN(value, reader, int64(size)); err != nil {
		return nil, err
	}
	for _, expected := range []string{"\r\n", "END\r\n"} {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if line != expected {
			return nil, errors.New("invalid memcached value terminator")
		}
	}
	return value.Bytes(), nil
}
