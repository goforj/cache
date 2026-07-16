# Migration To v0.4

This quality pass tightens construction, concurrency, inspection, and value-shaping contracts while
the module is still pre-v1.

## Required Review

### Coordinate Optional-Driver Shaping Rollouts

`Compression`, `MaxValueBytes`, and `EncryptionKey` were documented and generated for every driver,
but optional drivers did not previously apply them. v0.4 enforces them through one shared
`cachecore.WrapStore` implementation.

- Existing unmarked values remain readable by upgraded processes.
- New combined values persist as `CMP1(ENC1(plaintext))`.
- Older optional-driver binaries cannot decode values written with compression or encryption.
- Upgrade all cache readers together, or pause writes until every reader is upgraded.
- Flush or naturally expire old encrypted values before removing an old encryption key.
- Counters stay raw and retain backend-native atomic behavior.

Invalid encryption keys, negative `MaxValueBytes`, or unsupported codecs now fail closed.
Constructors that return an error return it directly. Redis, Memcached, and NATS retain their
Store-only constructor signatures and return a driver-preserving Store whose `Ready` and data
operations report the configuration error.

### Inspector Expiration Unit

`cachecore.CacheEntry.ExpiresAt` is now consistently a Unix timestamp in milliseconds. Memory and
file inspectors previously returned Unix nanoseconds; NATS, SQL, and DynamoDB already returned
milliseconds. Consumers that special-cased memory or file values must remove the nanosecond
conversion.

### Required Store Dependencies

`cache.NewCache(nil)` and `cache.NewMemoStore(nil)` now panic immediately. These stores are required
dependencies; construction-time failure replaces a later operation-time nil dereference.

## Behavior Fixes Without Call-Site Changes

- Derived `LockHandle` values share ownership state, preventing a stale handle from releasing a
  later owner's lock.
- File-store mutation sequences targeting the same normalized directory are serialized within the
  process, making Add and counters atomic across local store instances.
- File-store Flush removes only cache-owned files and preserves unrelated files in the directory.
- Memoized in-flight reads can no longer republish stale data after a local mutation or Flush.
  Successful mutations advance a bounded store-wide generation, so an unrelated in-flight read may
  skip memoization and fetch again on its next call; existing unrelated memo hits are retained.
- File inspection omits expired entries.
- `MaxValueBytes` bounds legacy raw reads and decompression as well as writes.

These fixes do not add cross-process atomicity to the file backend. Use a shared backend when locks,
counters, or conditional writes must coordinate multiple processes.
