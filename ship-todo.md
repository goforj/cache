# Ship TODO

## Release Readiness

- [ ] Define and publish `v1` stability boundaries.
  - [ ] Document which APIs are stable vs experimental.
  - [ ] Document deprecation policy and support window.

- [ ] Strengthen CI integration matrix across drivers.
  - [ ] Ensure contract tests run across all supported drivers and variants.
  - [ ] Add retry policy for known transient infra flakes.
  - [ ] Keep container images/version pins explicit and consistent.

- [x] Document behavior semantics clearly.
- [x] Publish a TTL/default-TTL matrix by operation and helper.
- [x] Document stale/refresh-ahead behavior, including edge cases and fallback behavior.
- [x] Document lock and rate-limit guarantees, explicitly calling out process-local vs distributed semantics.

- [ ] Add concurrency hardening tests.
  - [ ] High-parallel remember contention tests.
  - [ ] Lock/TryLock contention and timeout behavior.
  - [ ] Refresh-ahead race and stale handoff behavior.

- [ ] Complete examples coverage for exported API.
  - [ ] Ensure every exported helper/option has a compile-tested example.
  - [ ] Keep example index discoverable by function category.

- [ ] Publish migration + versioning hygiene.
  - [ ] Keep `CHANGELOG.md` up to date per release.
  - [ ] Add upgrade notes for behavior changes.

- [ ] Add production guidance docs.
  - [ ] Recommended defaults and tuning guidance.
  - [ ] Key naming/versioning conventions.
  - [ ] TTL jitter and miss-storm mitigation patterns.
  - [ ] Observability instrumentation patterns.

- [ ] Benchmark reproducibility and determinism.
  - [ ] Document host/env and benchmark methodology.
  - [ ] Pin benchmark dependencies and runtime assumptions.
  - [ ] Ensure generated benchmark artifacts are deterministic.

- [ ] Expand static/security checks in CI.
  - [ ] `go vet`
  - [ ] `staticcheck`
  - [ ] `govulncheck`

- [ ] Add backward-compatibility tests for stored values.
  - [ ] Compression/encryption/codec compatibility across versions.
  - [ ] Validate decode paths for legacy payloads.
