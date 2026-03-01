package cache

import "github.com/goforj/cache/cachecore"

import (
	"context"
	"time"
)

type nullStore struct{}

func newNullStore() cachecore.Store { return &nullStore{} }

func (s *nullStore) Driver() cachecore.Driver { return cachecore.DriverNull }
func (s *nullStore) Ready(context.Context) error { return nil }

func (s *nullStore) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, nil
}

func (s *nullStore) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (s *nullStore) Add(context.Context, string, []byte, time.Duration) (bool, error) {
	return true, nil
}

func (s *nullStore) Increment(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, nil
}

func (s *nullStore) Decrement(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, nil
}

func (s *nullStore) Delete(context.Context, string) error { return nil }

func (s *nullStore) DeleteMany(context.Context, ...string) error { return nil }

func (s *nullStore) Flush(context.Context) error { return nil }
