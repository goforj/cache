// Package cachetest provides reusable store contract tests for cache.Store implementations.
//
// Driver modules can use this package from their own tests without importing root test helpers.
//
// Example pattern (driver module test):
//
//	func TestRedisStoreContract(t *testing.T) {
//		client := newTestRedisClient(t)
//		store, err := rediscache.New(rediscache.Config{Client: client, Prefix: "test"})
//		if err != nil {
//			t.Fatalf("new redis store: %v", err)
//		}
//
//		// Namespace keys per test and tune TTL waits for backend semantics as needed.
//		cachetest.RunStoreContract(t, store, cachetest.Options{
//			CaseName: t.Name(),
//			TTL:      time.Second,
//			TTLWait:  1500 * time.Millisecond,
//		})
//	}
//
// Example factory/cleanup wrapper:
//
//	func runContractWithFactory(t *testing.T, mk func(t *testing.T) (cache.Store, func())) {
//		t.Helper()
//		store, cleanup := mk(t)
//		t.Cleanup(cleanup)
//		cachetest.RunStoreContract(t, store, cachetest.Options{CaseName: t.Name()})
//	}
package cachetest
