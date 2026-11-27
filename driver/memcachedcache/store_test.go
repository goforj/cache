package memcachedcache

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

// TestNewNilAddrErrors verifies Memcached construction rejects a nil server address.
func TestNewNilAddrErrors(t *testing.T) {
	store := New(Config{})
	if err := store.Ready(context.Background()); err == nil {
		t.Fatalf("expected ready dial error")
	}
	_, _, err := store.Get(context.Background(), "k")
	if err == nil {
		t.Fatalf("expected dial error")
	}
}

// TestNewShapingConfigFailureFailsClosed verifies Store-only construction preserves the config error.
func TestNewShapingConfigFailureFailsClosed(t *testing.T) {
	store := New(Config{BaseConfig: cachecore.BaseConfig{EncryptionKey: []byte("short")}})
	if err := store.Ready(context.Background()); !errors.Is(err, cachecore.ErrEncryptionKey) {
		t.Fatalf("Ready error = %v, want ErrEncryptionKey", err)
	}
}

// TestLockReleaseRequiresCurrentOwner verifies Memcached CAS protects successor lock values.
func TestLockReleaseRequiresCurrentOwner(t *testing.T) {
	tests := []struct {
		name     string
		stored   []byte
		owner    []byte
		released bool
	}{
		{name: "stale", stored: []byte("successor"), owner: []byte("stale"), released: false},
		{name: "owner", stored: []byte("owner"), owner: []byte("owner"), released: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, server := net.Pipe()
			serverErr := make(chan error, 1)
			go func() { serverErr <- serveLockRelease(server, tc.stored) }()
			originalDial := dialMemcached
			dialMemcached = func(context.Context, string, string) (net.Conn, error) { return client, nil }
			t.Cleanup(func() { dialMemcached = originalDial })
			store := New(Config{Addresses: []string{"test"}}).(*store)
			released, err := store.LockRelease(context.Background(), "lock:key", tc.owner)
			if err != nil || released != tc.released {
				t.Fatalf("LockRelease() = %v, %v, want %v", released, err, tc.released)
			}
			for _, pool := range store.pools {
				select {
				case conn := <-pool:
					_ = conn.conn.Close()
				default:
				}
			}
			if err := <-serverErr; err != nil {
				t.Fatalf("memcached test server: %v", err)
			}
		})
	}
}

// serveLockRelease implements the small Memcached protocol slice used by LockRelease.
func serveLockRelease(conn net.Conn, stored []byte) error {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "gets":
			if _, err := fmt.Fprintf(conn, "VALUE %s 0 %d 7\r\n%s\r\nEND\r\n", fields[1], len(stored), stored); err != nil {
				return err
			}
		case "cas":
			if len(fields) != 6 || fields[3] != strconv.Itoa(expiredMemcachedTimestamp) {
				return fmt.Errorf("unexpected ownership CAS: %q", strings.TrimSpace(line))
			}
			length, err := strconv.Atoi(fields[4])
			if err != nil {
				return err
			}
			payload := make([]byte, length+2)
			if _, err := io.ReadFull(reader, payload); err != nil {
				return err
			}
			stored = append([]byte(nil), payload[:length]...)
			if _, err := io.WriteString(conn, "STORED\r\n"); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("unexpected command %q", fields[0])
		}
	}
}

// TestKeysOnlyDialTheirSelectedPeer verifies routing ignores address order and fails without fallback.
func TestKeysOnlyDialTheirSelectedPeer(t *testing.T) {
	originalDial := dialMemcached
	t.Cleanup(func() { dialMemcached = originalDial })
	offline := errors.New("peer unavailable")
	var calls []string
	dialMemcached = func(_ context.Context, _, addr string) (net.Conn, error) {
		calls = append(calls, addr)
		return nil, offline
	}
	ctx := context.Background()
	for _, addresses := range [][]string{{"a", "b"}, {"b", "a", "b"}} {
		store := New(Config{Addresses: addresses})
		if err := store.Set(ctx, "session", []byte("value"), 0); !errors.Is(err, offline) {
			t.Fatalf("Set error = %v", err)
		}
		if _, _, err := store.Get(ctx, "session"); !errors.Is(err, offline) {
			t.Fatalf("Get error = %v", err)
		}
	}
	if len(calls) != 4 {
		t.Fatalf("dials = %v, want one attempted peer per operation", calls)
	}
	for _, call := range calls {
		if call != calls[0] {
			t.Fatalf("same key selected different peers: %v", calls)
		}
	}
}

// TestReadCancellation verifies canceling an operation interrupts established socket I/O.
func TestReadCancellation(t *testing.T) {
	originalDial := dialMemcached
	t.Cleanup(func() { dialMemcached = originalDial })
	client, server := net.Pipe()
	t.Cleanup(func() { _ = server.Close() })
	dialMemcached = func(context.Context, string, string) (net.Conn, error) { return client, nil }
	store := New(Config{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, _, err := store.Get(ctx, "key")
		result <- err
	}()
	if err := server.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := bufio.NewReader(server).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("canceled operation succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not interrupt the read")
	}
}

// TestFlushRequiresDedicatedServers verifies Flush rejects the default shared-server configuration.
func TestFlushRequiresDedicatedServers(t *testing.T) {
	store := New(Config{})
	if err := store.Flush(context.Background()); err == nil || !strings.Contains(err.Error(), "AllowServerFlush") {
		t.Fatalf("Flush error = %v", err)
	}
}

// TestCloseDrainsIdleConnections verifies shutdown releases pooled sockets and rejects further reads.
func TestCloseDrainsIdleConnections(t *testing.T) {
	store := New(Config{Addresses: []string{"peer"}}).(*store)
	client, server := net.Pipe()
	t.Cleanup(func() { _ = server.Close() })
	store.pools["peer"] <- &memcachedConn{addr: "peer", conn: client, reader: bufio.NewReader(client)}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := server.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("pooled peer read error = %v", err)
	}
	if _, _, err := store.Get(context.Background(), "key"); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Get after Close error = %v", err)
	}
}

// TestInvalidWireKeys verifies invalid key names fail before touching a server.
func TestInvalidWireKeys(t *testing.T) {
	store := New(Config{})
	for _, key := range []string{"space separated", strings.Repeat("x", 250)} {
		if _, _, err := store.Get(context.Background(), key); err == nil || !strings.Contains(err.Error(), "250 bytes") {
			t.Fatalf("Get error = %v", err)
		}
	}
}

// TestReadValueValidatesFraming verifies that incomplete and malformed responses fail before reuse.
func TestReadValueValidatesFraming(t *testing.T) {
	for _, tc := range []struct {
		name, response string
	}{
		{"short body", "a"},
		{"missing terminator", "abc\r\n"},
		{"bad terminator", "abc\r\nNO\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readValue(bufio.NewReader(strings.NewReader(tc.response)), 3)
			if err == nil {
				t.Fatal("readValue accepted incomplete or malformed framing")
			}
		})
	}
}
