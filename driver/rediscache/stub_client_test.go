package rediscache

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// stubClient is an in-memory Client used for unit tests.
type stubClient struct {
	store map[string]string
	ttl   map[string]time.Time

	expireErr error
	pingErr   error
	getErr    error
	setErr    error
	setNXErr  error
	incrErr   error
	scanErr   error
	delErr    error
}

// newStubClient creates an empty in-memory Redis client stub.
func newStubClient() *stubClient {
	return &stubClient{
		store: make(map[string]string),
		ttl:   make(map[string]time.Time),
	}
}

// expireIfNeeded removes values whose simulated Redis deadline has passed.
func (c *stubClient) expireIfNeeded(key string) {
	if deadline, ok := c.ttl[key]; ok && time.Now().After(deadline) {
		delete(c.ttl, key)
		delete(c.store, key)
	}
}

// Ping returns PONG or the configured readiness failure.
func (c *stubClient) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	if c.pingErr != nil {
		cmd.SetErr(c.pingErr)
		return cmd
	}
	cmd.SetVal("PONG")
	return cmd
}

// Get returns the live in-memory value or Redis's missing-key sentinel.
func (c *stubClient) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	if c.getErr != nil {
		cmd.SetErr(c.getErr)
		return cmd
	}
	c.expireIfNeeded(key)
	if val, ok := c.store[key]; ok {
		cmd.SetVal(val)
		return cmd
	}
	cmd.SetErr(redis.Nil)
	return cmd
}

// Set stores a value with the requested simulated expiration.
func (c *stubClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	if c.setErr != nil {
		cmd.SetErr(c.setErr)
		return cmd
	}
	bytes, _ := value.([]byte)
	c.store[key] = string(bytes)
	if expiration > 0 {
		c.ttl[key] = time.Now().Add(expiration)
	} else {
		delete(c.ttl, key)
	}
	cmd.SetVal("OK")
	return cmd
}

// SetNX stores a value only when no live value already exists.
func (c *stubClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx)
	if c.setNXErr != nil {
		cmd.SetErr(c.setNXErr)
		return cmd
	}
	c.expireIfNeeded(key)
	if _, exists := c.store[key]; exists {
		cmd.SetVal(false)
		return cmd
	}
	bytes, _ := value.([]byte)
	c.store[key] = string(bytes)
	if expiration > 0 {
		c.ttl[key] = time.Now().Add(expiration)
	}
	cmd.SetVal(true)
	return cmd
}

// IncrBy parses and updates the in-memory integer value.
func (c *stubClient) IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	if c.incrErr != nil {
		cmd.SetErr(c.incrErr)
		return cmd
	}
	c.expireIfNeeded(key)
	current := int64(0)
	if existing, ok := c.store[key]; ok {
		parsed, err := strconv.ParseInt(existing, 10, 64)
		if err != nil {
			cmd.SetErr(err)
			return cmd
		}
		current = parsed
	}
	current += value
	c.store[key] = strconv.FormatInt(current, 10)
	cmd.SetVal(current)
	return cmd
}

// Expire updates the simulated deadline for an existing value.
func (c *stubClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx)
	if c.expireErr != nil {
		cmd.SetErr(c.expireErr)
		return cmd
	}
	c.expireIfNeeded(key)
	if _, ok := c.store[key]; !ok {
		cmd.SetVal(false)
		return cmd
	}
	c.ttl[key] = time.Now().Add(expiration)
	cmd.SetVal(true)
	return cmd
}

// Del removes live values and reports how many existed.
func (c *stubClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	if c.delErr != nil {
		cmd.SetErr(c.delErr)
		return cmd
	}
	var removed int64
	for _, key := range keys {
		c.expireIfNeeded(key)
		if _, ok := c.store[key]; ok {
			delete(c.store, key)
			delete(c.ttl, key)
			removed++
		}
	}
	cmd.SetVal(removed)
	return cmd
}

// Scan returns live keys matching the prefix pattern used by store flushing.
func (c *stubClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	cmd := redis.NewScanCmd(ctx, nil)
	if c.scanErr != nil {
		cmd.SetErr(c.scanErr)
		return cmd
	}
	prefix := strings.TrimSuffix(match, "*")
	var keys []string
	for key := range c.store {
		c.expireIfNeeded(key)
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	cmd.SetVal(keys, 0)
	return cmd
}
