package cache

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func startFakeMemcached(t *testing.T) (addr string, stop func(), accept chan net.Conn) {
	t.Helper()
	data := make(map[string][]byte)
	accept = make(chan net.Conn, 4)
	go func() {
		for conn := range accept {
			go handleMemcachedConn(conn, data)
		}
	}()
	return "pipe", func() { close(accept) }, accept
}

func handleMemcachedConn(conn net.Conn, data map[string][]byte) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		switch parts[0] {
		case "get":
			if len(parts) < 2 {
				continue
			}
			key := parts[1]
			if v, ok := data[key]; ok {
				fmt.Fprintf(w, "VALUE %s 0 %d\r\n", key, len(v))
				w.Write(v)
				w.WriteString("\r\n")
			}
			w.WriteString("END\r\n")
		case "set", "add":
			// set <key> <flags> <exptime> <bytes>
			if len(parts) < 5 {
				continue
			}
			key := parts[1]
			n, _ := strconv.Atoi(parts[4])
			buf := make([]byte, n)
			if _, err := r.Read(buf); err != nil {
				return
			}
			// consume trailing \r\n
			r.ReadString('\n')
			if parts[0] == "add" {
				if _, exists := data[key]; exists {
					w.WriteString("NOT_STORED\r\n")
					w.Flush()
					continue
				}
			}
			data[key] = buf
			w.WriteString("STORED\r\n")
		case "incr", "decr":
			if len(parts) < 3 {
				continue
			}
			key := parts[1]
			delta, _ := strconv.ParseInt(parts[2], 10, 64)
			val := int64(0)
			raw, ok := data[key]
			if ok {
				val, _ = strconv.ParseInt(string(raw), 10, 64)
			} else {
				w.WriteString("NOT_FOUND\r\n")
				w.Flush()
				continue
			}
			if parts[0] == "incr" {
				val += delta
			} else {
				val -= delta
			}
			data[key] = []byte(strconv.FormatInt(val, 10))
			w.WriteString(strconv.FormatInt(val, 10))
			w.WriteString("\r\n")
		case "touch":
			if len(parts) < 3 {
				continue
			}
			key := parts[1]
			if _, ok := data[key]; ok {
				w.WriteString("TOUCHED\r\n")
			} else {
				w.WriteString("NOT_FOUND\r\n")
			}
		case "delete":
			if len(parts) < 2 {
				continue
			}
			key := parts[1]
			delete(data, key)
			w.WriteString("DELETED\r\n")
		case "flush_all":
			for k := range data {
				delete(data, k)
			}
			w.WriteString("OK\r\n")
		default:
			// ignore
		}
		w.Flush()
	}
}

func TestMemcachedStoreAgainstFakeServer(t *testing.T) {
	origDial := dialMemcached
	defer func() { dialMemcached = origDial }()
	serverAddr, stop, accept := startFakeMemcached(t)
	defer stop()
	dialMemcached = func(ctx context.Context, network, addr string) (net.Conn, error) {
		_, _ = ctx, network
		_ = addr
		server, client := net.Pipe()
		accept <- server
		return client, nil
	}

	store := newMemcachedStore([]string{serverAddr}, time.Second, "pfx")
	ctx := context.Background()

	if err := store.Set(ctx, "a", []byte("1"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := store.Set(ctx, "b", []byte("2"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "a")
	if err != nil || !ok || string(body) != "1" {
		t.Fatalf("get failed: ok=%v err=%v val=%s", ok, err, string(body))
	}

	created, err := store.Add(ctx, "a", []byte("x"), 0)
	if err != nil || created {
		t.Fatalf("add duplicate unexpected: created=%v err=%v", created, err)
	}

	val, err := store.Increment(ctx, "cnt", 3, 0)
	if err != nil || val != 3 {
		t.Fatalf("incr failed: %v val=%d", err, val)
	}
	val, err = store.Decrement(ctx, "cnt", 1, 0)
	if err != nil || val != 2 {
		t.Fatalf("decr failed: %v val=%d", err, val)
	}

	if err := store.DeleteMany(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
}
