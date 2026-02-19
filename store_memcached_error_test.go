package cache

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMemcachedDialErrorIsPropagated(t *testing.T) {
	orig := dialMemcached
	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("dial fail")
	}
	defer func() { dialMemcached = orig }()

	store := newMemcachedStore([]string{"bad"}, time.Second, "p")
	if _, _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatalf("expected dial error")
	}
}

func TestMemcachedCacheKeyNoPrefix(t *testing.T) {
	s := newMemcachedStore([]string{"a"}, time.Second, "")
	if ms, ok := s.(*memcachedStore); ok {
		if ms.cacheKey("k") != "app:k" {
			t.Fatalf("unexpected key")
		}
	}
}

func TestMemcachedGetMissAndUnexpected(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()

	// Miss path with END response.
	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			_, _ = r.ReadString('\n')
			server.Write([]byte("END\r\n"))
			server.Close()
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if _, ok, err := store.Get(context.Background(), "k"); err != nil || ok {
		t.Fatalf("expected miss without error")
	}

	// Unexpected response path.
	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			_, _ = r.ReadString('\n')
			server.Write([]byte("BAD\r\n"))
			server.Close()
		}()
		return client, nil
	}
	if _, _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatalf("expected unexpected response error")
	}
}

func TestMemcachedSetUsesMinimumTTL(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()

	var exptime string
	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			line, _ := r.ReadString('\n')
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) >= 4 {
				exptime = parts[3]
			}
			if len(parts) >= 5 {
				n, _ := strconv.Atoi(parts[4])
				buf := make([]byte, n)
				_, _ = io.ReadFull(r, buf)
				_, _ = r.ReadString('\n')
			}
			server.Write([]byte("STORED\r\n"))
			server.Close()
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if err := store.Set(context.Background(), "short", []byte("v"), 500*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if exptime != "1" {
		t.Fatalf("expected min exptime 1, got %s", exptime)
	}
}

func TestMemcachedFlushError(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()
	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			_, _ = r.ReadString('\n')
			server.Write([]byte("ERR\r\n"))
			server.Close()
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if err := store.Flush(context.Background()); err == nil {
		t.Fatalf("expected flush error")
	}
}

func TestMemcachedDecrementNegativeDelta(t *testing.T) {
	data := map[string][]byte{}
	orig := dialMemcached
	dialMemcached = memcachedInMemoryDial(data)
	defer func() { dialMemcached = orig }()

	store := newMemcachedStore([]string{"ignored"}, time.Second, "p")
	if _, err := store.Decrement(context.Background(), "n", -2, time.Second); err != nil {
		t.Fatalf("decrement negative failed: %v", err)
	}
}

func TestMemcachedAddUnexpectedResponse(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()

	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			line, _ := r.ReadString('\n')
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) >= 5 {
				n, _ := strconv.Atoi(parts[4])
				buf := make([]byte, n)
				_, _ = io.ReadFull(r, buf)
				_, _ = r.ReadString('\n')
			}
			server.Write([]byte("WAT\r\n"))
			server.Close()
			_ = parts
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if _, err := store.Add(context.Background(), "odd", []byte("v"), 500*time.Millisecond); err == nil {
		t.Fatalf("expected add error")
	}
}

func TestMemcachedDialErrors(t *testing.T) {
	orig := dialMemcached
	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("dial fail")
	}
	defer func() { dialMemcached = orig }()

	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if err := store.Set(context.Background(), "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected set dial error")
	}
	if _, err := store.Add(context.Background(), "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected add dial error")
	}
	if err := store.Delete(context.Background(), "k"); err == nil {
		t.Fatalf("expected delete dial error")
	}
	if err := store.Flush(context.Background()); err == nil {
		t.Fatalf("expected flush dial error")
	}
}

func TestMemcachedGetParseLengthError(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()

	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			_, _ = r.ReadString('\n')
			server.Write([]byte("VALUE k 0 notanint\r\n"))
			server.Close()
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if _, _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestMemcachedSetUnexpectedResponse(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()

	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			line, _ := r.ReadString('\n')
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) >= 5 {
				n, _ := strconv.Atoi(parts[4])
				buf := make([]byte, n)
				_, _ = io.ReadFull(r, buf)
				_, _ = r.ReadString('\n')
			}
			server.Write([]byte("NOT_STORED\r\n"))
			server.Close()
			_ = parts
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if err := store.Set(context.Background(), "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected set error")
	}
}

func TestMemcachedIncrementErrorResponse(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()

	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			_, _ = r.ReadString('\n')
			server.Write([]byte("ERROR something\r\n"))
			server.Close()
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if _, err := store.Increment(context.Background(), "k", 1, time.Second); err == nil {
		t.Fatalf("expected incr error")
	}
}

func TestMemcachedDeleteManyError(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()
	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("dial fail")
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if err := store.DeleteMany(context.Background(), "a", "b"); err == nil {
		t.Fatalf("expected delete many error")
	}
}

func TestMemcachedGetReadError(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()

	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			_, _ = r.ReadString('\n')
			server.Write([]byte("VALUE k 0 5\r\nva")) // incomplete value
			server.Close()
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if _, _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatalf("expected read error")
	}
}

func TestMemcachedIncrementParseError(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()

	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			_, _ = r.ReadString('\n')
			server.Write([]byte("notnum\r\n"))
			server.Close()
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if _, err := store.Increment(context.Background(), "k", 1, time.Second); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestMemcachedSetWriteError(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()
	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		return errConn{}, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if err := store.Set(context.Background(), "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected write error")
	}
}

func TestMemcachedGetMalformedResponse(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()

	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			_, _ = r.ReadString('\n')
			server.Write([]byte("VALUE k\r\n"))
			server.Close()
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if _, _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatalf("expected malformed error")
	}
}

func TestMemcachedIncrementWriteError(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()
	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		return errConn{}, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if _, err := store.Increment(context.Background(), "k", 1, time.Second); err == nil {
		t.Fatalf("expected write error")
	}
}

func TestMemcachedGetTrailingReadError(t *testing.T) {
	orig := dialMemcached
	defer func() { dialMemcached = orig }()

	dialMemcached = func(context.Context, string, string) (net.Conn, error) {
		server, client := net.Pipe()
		go func() {
			r := bufio.NewReader(server)
			_, _ = r.ReadString('\n')
			server.Write([]byte("VALUE k 0 1\r\nv"))
			server.Close()
		}()
		return client, nil
	}
	store := newMemcachedStore([]string{"x"}, time.Second, "p")
	if _, _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatalf("expected trailing read error")
	}
}

type errConn struct{}

func (errConn) Read(b []byte) (int, error)       { return len(b), nil }
func (errConn) Write([]byte) (int, error)        { return 0, errors.New("write boom") }
func (errConn) Close() error                     { return nil }
func (errConn) LocalAddr() net.Addr              { return nil }
func (errConn) RemoteAddr() net.Addr             { return nil }
func (errConn) SetDeadline(time.Time) error      { return nil }
func (errConn) SetReadDeadline(time.Time) error  { return nil }
func (errConn) SetWriteDeadline(time.Time) error { return nil }
