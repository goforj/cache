package cachetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

// waitStore controls the sequence observed by waitForMiss.
type waitStore struct {
	get func() (bool, error)
}

// Driver identifies the test backend.
func (s *waitStore) Driver() cachecore.Driver { return cachecore.DriverMemory }

// Ready reports that the test backend is available.
func (s *waitStore) Ready(context.Context) error { return nil }

// Get returns the configured presence and failure state.
func (s *waitStore) Get(context.Context, string) ([]byte, bool, error) {
	ok, err := s.get()
	return nil, ok, err
}

// Set is unused by the focused polling tests.
func (s *waitStore) Set(context.Context, string, []byte, time.Duration) error { return nil }

// Add is unused by the focused polling tests.
func (s *waitStore) Add(context.Context, string, []byte, time.Duration) (bool, error) {
	return false, nil
}

// Increment is unused by the focused polling tests.
func (s *waitStore) Increment(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, nil
}

// Decrement is unused by the focused polling tests.
func (s *waitStore) Decrement(context.Context, string, int64, time.Duration) (int64, error) {
	return 0, nil
}

// Delete is unused by the focused polling tests.
func (s *waitStore) Delete(context.Context, string) error { return nil }

// DeleteMany is unused by the focused polling tests.
func (s *waitStore) DeleteMany(context.Context, ...string) error { return nil }

// Flush is unused by the focused polling tests.
func (s *waitStore) Flush(context.Context) error { return nil }

// TestWaitForMiss verifies immediate misses and backend failures terminate polling.
func TestWaitForMiss(t *testing.T) {
	miss := &waitStore{get: func() (bool, error) { return false, nil }}
	if err := waitForMiss(context.Background(), miss, "key", time.Second); err != nil {
		t.Fatalf("immediate miss error = %v", err)
	}

	expected := errors.New("backend failed")
	failed := &waitStore{get: func() (bool, error) { return false, expected }}
	if err := waitForMiss(context.Background(), failed, "key", time.Second); !errors.Is(err, expected) {
		t.Fatalf("backend error = %v, want %v", err, expected)
	}
}

// TestWaitForMissTimeout verifies a key that remains present reports a useful timeout.
func TestWaitForMissTimeout(t *testing.T) {
	present := &waitStore{get: func() (bool, error) { return true, nil }}
	if err := waitForMiss(context.Background(), present, "key", time.Millisecond); err == nil {
		t.Fatal("waitForMiss returned nil for a key that remained present")
	}
}

// TestSanitize verifies test names become backend-safe key fragments.
func TestSanitize(t *testing.T) {
	if got := sanitize("suite/case name"); got != "suite_case_name" {
		t.Fatalf("sanitize() = %q, want suite_case_name", got)
	}
}
