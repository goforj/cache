package memcachedcache

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

type scriptedConnection struct {
	net.Conn
	response    *strings.Reader
	writes      strings.Builder
	writeErr    error
	deadline    time.Time
	deadlineErr error
	closed      bool
}

// Read serves the response bytes recorded for this connection.
func (c *scriptedConnection) Read(p []byte) (int, error) { return c.response.Read(p) }

// Write records the commands sent before a connection is returned or discarded.
func (c *scriptedConnection) Write(p []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return c.writes.Write(p)
}

// SetDeadline records the deadline and returns any configured socket error.
func (c *scriptedConnection) SetDeadline(deadline time.Time) error {
	c.deadline = deadline
	return c.deadlineErr
}

// Close records that the connection can no longer return to the pool.
func (c *scriptedConnection) Close() error {
	c.closed = true
	return nil
}

// serveScriptedConnections replaces dialing in tests that need reproducible protocol failures.
func serveScriptedConnections(t *testing.T, conns ...*scriptedConnection) {
	t.Helper()
	originalDial := dialMemcached
	t.Cleanup(func() { dialMemcached = originalDial })
	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		if len(conns) == 0 {
			t.Fatal("unexpected additional connection")
		}
		conn := conns[0]
		conns = conns[1:]
		return conn, nil
	}
}

// TestMalformedResponsesDiscardConnections verifies protocol errors cannot leave a socket in the pool.
func TestMalformedResponsesDiscardConnections(t *testing.T) {
	for _, tc := range []struct {
		name, response, want string
		call                 func(context.Context, *store) error
	}{
		{
			name: "get checks the returned key", response: "VALUE app:other 0 1\r\n", want: "unexpected response",
			call: func(ctx context.Context, s *store) error { _, _, err := s.Get(ctx, "key"); return err },
		},
		{
			name: "get rejects an invalid length", response: "VALUE app:key 0 -1\r\n", want: "invalid memcached value length",
			call: func(ctx context.Context, s *store) error { _, _, err := s.Get(ctx, "key"); return err },
		},
		{
			name: "lock release rejects an invalid length", response: "VALUE app:key 0 invalid 7\r\n", want: "invalid memcached value length",
			call: func(ctx context.Context, s *store) error {
				_, err := s.LockRelease(ctx, "key", []byte("owner"))
				return err
			},
		},
		{
			name: "delete propagates server errors", response: "SERVER_ERROR unavailable\r\n", want: "memcached delete failed",
			call: func(ctx context.Context, s *store) error { return s.Delete(ctx, "key") },
		},
		{
			name: "ready checks the version response", response: "SERVER_ERROR unavailable\r\n", want: "memcached version failed",
			call: func(ctx context.Context, s *store) error { return s.Ready(ctx) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := &scriptedConnection{response: strings.NewReader(tc.response)}
			serveScriptedConnections(t, conn)
			err := tc.call(t.Context(), New(Config{}).(*store))
			if err == nil || !strings.Contains(err.Error(), tc.want) || !conn.closed {
				t.Fatalf("operation error = %v, connection closed = %t; want %q and a closed connection", err, conn.closed, tc.want)
			}
		})
	}
}

// TestCounterNetworkErrors verifies read and write errors discard the socket and return the underlying error.
func TestCounterNetworkErrors(t *testing.T) {
	writeErr := errors.New("write failed")
	for _, tc := range []struct {
		name     string
		writeErr error
		want     error
	}{
		{"write failure", writeErr, writeErr},
		{"read failure", nil, io.EOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := &scriptedConnection{response: strings.NewReader(""), writeErr: tc.writeErr}
			serveScriptedConnections(t, conn)
			_, err := New(Config{}).Increment(t.Context(), "counter", 1, time.Minute)
			if !errors.Is(err, tc.want) || !conn.closed {
				t.Fatalf("Increment error = %v, connection closed = %t; want %v and a closed connection", err, conn.closed, tc.want)
			}
		})
	}
}

// TestCounterStopsAfterRepeatedMisses verifies eviction during initialization cannot produce unbounded retries.
func TestCounterStopsAfterRepeatedMisses(t *testing.T) {
	counter := &scriptedConnection{response: strings.NewReader(strings.Repeat("NOT_FOUND\r\n", 4))}
	initializer := &scriptedConnection{response: strings.NewReader(strings.Repeat("STORED\r\n", 3))}
	serveScriptedConnections(t, counter, initializer)
	s := New(Config{}).(*store)
	t.Cleanup(func() { _ = s.Close() })

	_, err := s.Increment(t.Context(), "counter", 1, time.Minute)
	if err == nil || !strings.Contains(err.Error(), "counter disappeared during initialization") {
		t.Fatalf("Increment error = %v", err)
	}
	if calls := strings.Count(counter.writes.String(), "incr "); calls != 4 {
		t.Fatalf("incr commands = %d, want 4", calls)
	}
	if calls := strings.Count(initializer.writes.String(), "add "); calls != 3 {
		t.Fatalf("add commands = %d, want 3", calls)
	}
}

// TestConnectionDeadline verifies cancellation, the caller's deadline, and socket setup errors.
func TestConnectionDeadline(t *testing.T) {
	t.Run("already canceled", func(t *testing.T) {
		serveScriptedConnections(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, _, err := New(Config{}).Get(ctx, "key")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Get error = %v, want context.Canceled", err)
		}
	})
	t.Run("shorter deadline", func(t *testing.T) {
		deadline := time.Now().Add(2 * time.Second)
		ctx, cancel := context.WithDeadline(t.Context(), deadline)
		defer cancel()
		conn := &scriptedConnection{response: strings.NewReader("END\r\n")}
		serveScriptedConnections(t, conn)
		s := New(Config{}).(*store)
		t.Cleanup(func() { _ = s.Close() })
		if _, hit, err := s.Get(ctx, "key"); err != nil || hit || !conn.deadline.Equal(deadline) {
			t.Fatalf("Get hit = %t, error = %v, deadline = %v; want miss with deadline %v", hit, err, conn.deadline, deadline)
		}
	})
	t.Run("deadline setup failure", func(t *testing.T) {
		deadlineErr := errors.New("set deadline failed")
		conn := &scriptedConnection{deadlineErr: deadlineErr}
		serveScriptedConnections(t, conn)
		_, _, err := New(Config{}).Get(t.Context(), "key")
		if !errors.Is(err, deadlineErr) || !conn.closed || conn.writes.Len() != 0 {
			t.Fatalf("Get error = %v, connection closed = %t, bytes sent = %d", err, conn.closed, conn.writes.Len())
		}
	})
}

// TestReleaseClosesConnectionWhenPoolIsFull verifies a burst of sockets cannot overflow the idle pool.
func TestReleaseClosesConnectionWhenPoolIsFull(t *testing.T) {
	s := New(Config{Addresses: []string{"peer"}}).(*store)
	t.Cleanup(func() { _ = s.Close() })
	pool := s.pools["peer"]
	for range cap(pool) {
		pool <- &memcachedConn{addr: "peer", conn: &scriptedConnection{}}
	}
	extra := &scriptedConnection{}
	s.release(&memcachedConn{addr: "peer", conn: extra}, false)
	if !extra.closed || len(pool) != cap(pool) {
		t.Fatalf("extra connection closed = %t, pool length = %d", extra.closed, len(pool))
	}
}
