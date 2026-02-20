# Behavior Semantics

This document defines runtime behavior for TTL/default TTL, stale and refresh-ahead flows, and lock/rate-limit guarantees.

## TTL And Default TTL Matrix

Cache resolves TTL with:

- if ttl > 0: use ttl
- otherwise: use Cache.defaultTTL (configured by NewCacheWithTTL or store config)

| API | TTL Input | Effective TTL Written |
|---|---|---|
| Set / SetCtx | ttl > 0 | ttl |
| Set / SetCtx | ttl <= 0 | defaultTTL |
| SetString / SetStringCtx | same as Set | same as Set |
| SetJSON / SetJSONCtx | same as Set | same as Set |
| Add / AddCtx | ttl > 0 | ttl |
| Add / AddCtx | ttl <= 0 | defaultTTL |
| Increment / IncrementCtx | ttl > 0 | ttl |
| Increment / IncrementCtx | ttl <= 0 | defaultTTL |
| Decrement / DecrementCtx | ttl > 0 | ttl |
| Decrement / DecrementCtx | ttl <= 0 | defaultTTL |
| BatchSet / BatchSetCtx | each key uses same rule as Set | per-key resolved TTL |
| Remember* (non-stale) | forwards ttl to SetCtx | resolved as above |
| RememberStale* primary key | forwards ttl to SetCtx | resolved as above |
| RateLimit* bucket key | uses window as TTL | exactly window (must be > 0) |
| TryLock* lock key | uses provided ttl | exactly provided ttl (must be > 0) |
| RefreshAhead* primary key | requires ttl > 0 | exactly ttl |
| RefreshAhead* metadata key | requires ttl > 0 | exactly ttl |

Notes:

- RefreshAhead* does not allow fallback TTL; it returns an error when ttl <= 0.
- RateLimit* and lock helpers enforce positive TTL/window and return errors otherwise.

## Stale And Refresh-Ahead Semantics

## RememberStale*

- Read order:
  - read fresh key (key)
  - if hit: return it (usedStale=false)
  - if miss: call loader
- Loader success path:
  - write fresh key with ttl
  - write stale key (key + ":__stale") with staleTTL
- Loader error path:
  - attempt stale key read
  - if stale exists: return stale (usedStale=true, err=nil)
  - else: return loader error (and stale read error joined when present)

staleTTL behavior:

- if staleTTL <= 0, code sets staleTTL = ttl
- stale key is written only if resulting staleTTL > 0
- practical implication: if both input ttl <= 0 and staleTTL <= 0, fresh key still uses defaultTTL, but stale key is not written

## RefreshAhead*

- Requires ttl > 0 and refreshAhead > 0.
- On cache miss:
  - synchronously compute via loader
  - write value key
  - write metadata key (key + ":__refresh_exp") storing expiration timestamp
- On cache hit:
  - return current value immediately
  - optionally trigger async refresh if key is near expiry

Async refresh trigger conditions:

- metadata key exists and parses
- time.Until(expiry) <= refreshAhead
- per-key refresh lock acquired ("__refresh_lock:" + key)

Refresh-ahead caveats:

- Uses goroutine with background context for async refresh.
- Metadata key drives near-expiry detection; keys not populated by RefreshAhead* will not auto-refresh until metadata exists.

## Lock And Rate-Limit Guarantees

## Locking (TryLock, Lock, Unlock)

- Mechanism: lock key ("__lock:"+key) using Add (set-if-absent).
- Scope:
  - distributed only when backend is shared/distributed
  - process-local when backend is process/local only (for example memory)
- Guarantee:
  - mutual exclusion best-effort based on backend atomic Add
  - lock expires by TTL
- Important caveat:
  - Unlock deletes lock key without owner token validation
  - use short TTLs and keep critical sections bounded

## Rate limiting (RateLimit, RateLimitWithRemaining)

- Algorithm: fixed-window counter.
- Keying: bucketKey = key + ":" + floor(now/window).
- Each call increments bucket counter with TTL=window.
- Returns:
  - allowed = count <= limit
  - count current bucket count
  - optional remaining and resetAt

Scope and semantics:

- Global/shared only when backend is shared/distributed.
- Process-local when backend is local/in-process.
- Boundary behavior: fixed-window rollover at window boundary (not sliding-window smoothing).
