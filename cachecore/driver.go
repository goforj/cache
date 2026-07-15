package cachecore

// Driver identifies cache backend.
type Driver string

const (
	// DriverNull identifies the no-op backend.
	DriverNull Driver = "null"
	// DriverFile identifies the local filesystem backend.
	DriverFile Driver = "file"
	// DriverMemory identifies the in-process memory backend.
	DriverMemory Driver = "memory"
	// DriverMemcached identifies the Memcached backend.
	DriverMemcached Driver = "memcached"
	// DriverDynamo identifies the DynamoDB backend.
	DriverDynamo Driver = "dynamodb"
	// DriverSQL identifies the shared SQL backend implementation.
	DriverSQL Driver = "sql"
	// DriverRedis identifies the Redis backend.
	DriverRedis Driver = "redis"
	// DriverNATS identifies the NATS JetStream key-value backend.
	DriverNATS Driver = "nats"
)
