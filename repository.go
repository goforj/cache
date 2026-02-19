package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Repository provides an ergonomic cache API on top of Store.
type Repository struct {
	store      Store
	defaultTTL time.Duration
}

// NewRepository creates a cache repository bound to a concrete store.
// @group Constructors
//
// Example: repository from store
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	_ = repo
func NewRepository(store Store) *Repository {
	return NewRepositoryWithTTL(store, defaultCacheTTL)
}

// NewRepositoryWithTTL lets callers override the default TTL applied when ttl <= 0.
// @group Constructors
//
// Example: repository with custom default TTL
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepositoryWithTTL(store, 2*time.Minute)
//	_ = ctx
//	_ = repo
func NewRepositoryWithTTL(store Store, defaultTTL time.Duration) *Repository {
	if defaultTTL <= 0 {
		defaultTTL = defaultCacheTTL
	}
	return &Repository{
		store:      store,
		defaultTTL: defaultTTL,
	}
}

// Store returns the underlying store implementation.
// @group Repository
//
// Example: access store
//
//	store := repo.Store()
func (r *Repository) Store() Store {
	return r.store
}

// Get returns raw bytes for key when present.
// @group Repository
//
// Example: get bytes
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	_ = repo.Set(ctx, "user:42", []byte("Ada"), 0)
//	value, ok, _ := repo.Get(ctx, "user:42")
//	_ = value
//	_ = ok
func (r *Repository) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return r.store.Get(ctx, key)
}

// GetString returns a UTF-8 string value for key when present.
// @group Repository
//
// Example: get string
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	_ = repo.SetString(ctx, "user:42:name", "Ada", 0)
//	name, ok, _ := repo.GetString(ctx, "user:42:name")
//	_ = name
//	_ = ok
func (r *Repository) GetString(ctx context.Context, key string) (string, bool, error) {
	body, ok, err := r.Get(ctx, key)
	if err != nil || !ok {
		return "", ok, err
	}
	return string(body), true, nil
}

// GetJSON decodes a JSON value into T when key exists.
// @group Repository JSON
//
// Example: get JSON
//
//	type Profile struct { Name string `json:"name"` }
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	_ = cache.SetJSON(ctx, repo, "profile:42", Profile{Name: "Ada"}, 0)
//	profile, ok, _ := cache.GetJSON[Profile](ctx, repo, "profile:42")
//	_ = profile
//	_ = ok
func GetJSON[T any](ctx context.Context, r *Repository, key string) (T, bool, error) {
	var zero T
	body, ok, err := r.Get(ctx, key)
	if err != nil || !ok {
		return zero, ok, err
	}
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return zero, false, err
	}
	return out, true, nil
}

// Set writes raw bytes to key.
// @group Repository
//
// Example: set bytes with ttl
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	_ = repo.Set(ctx, "token", []byte("abc"), time.Minute)
func (r *Repository) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.store.Set(ctx, key, value, r.resolveTTL(ttl))
}

// SetString writes a string value to key.
// @group Repository
//
// Example: set string
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	_ = repo.SetString(ctx, "user:42:name", "Ada", time.Minute)
func (r *Repository) SetString(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.Set(ctx, key, []byte(value), ttl)
}

// SetJSON encodes value as JSON and writes it to key.
// @group Repository JSON
//
// Example: set JSON
//
//	type Profile struct { Name string `json:"name"` }
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	_ = cache.SetJSON(ctx, repo, "profile:42", Profile{Name: "Ada"}, time.Minute)
func SetJSON[T any](ctx context.Context, r *Repository, key string, value T, ttl time.Duration) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.Set(ctx, key, body, ttl)
}

// Add writes value only when key is not already present.
// @group Repository
//
// Example: add once
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	created, _ := repo.Add(ctx, "boot:seeded", []byte("1"), time.Hour)
//	_ = created
func (r *Repository) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	return r.store.Add(ctx, key, value, r.resolveTTL(ttl))
}

// Increment increments a numeric value and returns the result.
// @group Repository
//
// Example: increment counter
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	value, _ := repo.Increment(ctx, "rate:login:42", 1, time.Minute)
//	_ = value
func (r *Repository) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return r.store.Increment(ctx, key, delta, r.resolveTTL(ttl))
}

// Decrement decrements a numeric value and returns the result.
// @group Repository
//
// Example: decrement counter
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	value, _ := repo.Decrement(ctx, "rate:login:42", 1, time.Minute)
//	_ = value
func (r *Repository) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return r.store.Decrement(ctx, key, delta, r.resolveTTL(ttl))
}

// Pull returns value and removes it from cache.
// @group Repository
//
// Example: pull and delete
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	_ = repo.SetString(ctx, "reset:token:42", "abc", time.Minute)
//	body, ok, _ := repo.Pull(ctx, "reset:token:42")
//	_ = body
//	_ = ok
func (r *Repository) Pull(ctx context.Context, key string) ([]byte, bool, error) {
	body, ok, err := r.Get(ctx, key)
	if err != nil || !ok {
		return nil, ok, err
	}
	if err := r.Delete(ctx, key); err != nil {
		return nil, false, err
	}
	return body, true, nil
}

// Delete removes a single key.
// @group Repository
//
// Example: delete key
//
//	ctx := context.Background()
//	repo := cache.NewRepository(cache.NewMemoryStore(ctx))
//	_ = repo.Delete(ctx, "a")
func (r *Repository) Delete(ctx context.Context, key string) error {
	return r.store.Delete(ctx, key)
}

// DeleteMany removes multiple keys.
// @group Repository
//
// Example: delete many keys
//
//	ctx := context.Background()
//	repo := cache.NewRepository(cache.NewMemoryStore(ctx))
//	_ = repo.DeleteMany(ctx, "a", "b")
func (r *Repository) DeleteMany(ctx context.Context, keys ...string) error {
	return r.store.DeleteMany(ctx, keys...)
}

// Flush clears all keys for this store scope.
// @group Repository
//
// Example: flush all keys
//
//	ctx := context.Background()
//	repo := cache.NewRepository(cache.NewMemoryStore(ctx))
//	_ = repo.Flush(ctx)
func (r *Repository) Flush(ctx context.Context) error {
	return r.store.Flush(ctx)
}

// Remember returns key value or computes/stores it when missing.
// @group Repository
//
// Example: remember bytes
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	data, err := repo.Remember(ctx, "dashboard:summary", time.Minute, func(context.Context) ([]byte, error) {
//		return []byte("payload"), nil
//	})
//	_ = data
//	_ = err
func (r *Repository) Remember(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	body, ok, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if ok {
		return body, nil
	}
	if fn == nil {
		return nil, errors.New("cache remember requires a callback")
	}
	body, err = fn(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.Set(ctx, key, body, ttl); err != nil {
		return nil, err
	}
	return body, nil
}

// RememberString returns key value or computes/stores it when missing.
// @group Repository
//
// Example: remember string
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	value, err := repo.RememberString(ctx, "settings:mode", time.Minute, func(context.Context) (string, error) {
//		return "on", nil
//	})
//	_ = value
//	_ = err
func (r *Repository) RememberString(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) (string, error)) (string, error) {
	value, err := r.Remember(ctx, key, ttl, func(ctx context.Context) ([]byte, error) {
		if fn == nil {
			return nil, errors.New("cache remember string requires a callback")
		}
		body, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return []byte(body), nil
	})
	if err != nil {
		return "", err
	}
	return string(value), nil
}

// RememberJSON returns key value or computes/stores JSON when missing.
// @group Repository JSON
//
// Example: remember JSON
//
//	type Settings struct { Enabled bool `json:"enabled"` }
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	repo := cache.NewRepository(store)
//	settings, err := cache.RememberJSON[Settings](ctx, repo, "settings:alerts", time.Minute, func(context.Context) (Settings, error) {
//		return Settings{Enabled: true}, nil
//	})
//	_ = settings
//	_ = err
func RememberJSON[T any](ctx context.Context, r *Repository, key string, ttl time.Duration, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	out, ok, err := GetJSON[T](ctx, r, key)
	if err != nil {
		return zero, err
	}
	if ok {
		return out, nil
	}
	if fn == nil {
		return zero, errors.New("cache remember json requires a callback")
	}
	value, err := fn(ctx)
	if err != nil {
		return zero, err
	}
	if err := SetJSON(ctx, r, key, value, ttl); err != nil {
		return zero, err
	}
	return value, nil
}

func (r *Repository) resolveTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return r.defaultTTL
}
