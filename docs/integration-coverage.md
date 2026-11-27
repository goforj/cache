# Integration Coverage

CI runs the [integration module](../integration) for each of the ten cache backends. The suite uses embedded SQLite, containers for networked drivers, and DynamoDB Local for DynamoDB.

| Suite | What it checks | Scope |
| :--- | :--- | :--- |
| Shared store contract | Reads and writes, TTL, conditional add, counters, deletion, and flush | Selected drivers, with driver-specific behavior such as null-store no-ops and explicitly enabled Memcached server flush |
| Cache helpers and options | Typed read-through, stale fallback, refresh-ahead, batch calls, rate-limit windows, default TTL, compression, encryption, and size limits | Memory, File, and Null in the root suite, where applicable |
| Cancellation and injected faults | Slow calls, cancellation, and error propagation through composed helpers | Memory and File in the root suite |
| Ownership and inspection | Expired lock owners cannot release a successor's lock. Inspection checks run on drivers that expose inspection. | Selected drivers that support the operation |
| SQL | Literal-prefix inspection and flush, preservation of nonnumeric values on failed increments, and atomic initialization of concurrent counters | Namespace and nonnumeric-value checks on SQLite, PostgreSQL, and MySQL. Concurrent first increments on PostgreSQL. |
| Memcached | Two peers grant a single lock owner and explicitly enabled server flush reaches both peers | Real Memcached processes |
| NATS | An expired snapshot cannot erase a newer value or cause a lost counter update | Real JetStream KV |
| Observer context | The cache observer receives the context bound to an operation | Selected drivers |
| Memoized reads | Lifetime and entry bounds, local invalidation, and concurrent reads and writes | Root unit tests |

Run both integration suites from the repository root with Docker available for networked backends:

```sh
make test-integration
```

Use `make test-integration sqlitecache` to run the embedded SQLite cases without Docker. The [CI workflow](../.github/workflows/test.yml) lists all supported driver selectors.
