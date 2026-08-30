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
		case "delete":
			stored = nil
			if _, err := io.WriteString(conn, "DELETED\r\n"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected command %q", fields[0])
		}
	}
}
