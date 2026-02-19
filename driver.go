package cache

// Driver identifies cache backend.
type Driver string

const (
	DriverNull      Driver = "null"
	DriverFile      Driver = "file"
	DriverMemory    Driver = "memory"
	DriverMemcached Driver = "memcached"
	DriverDynamo    Driver = "dynamodb"
	DriverRedis     Driver = "redis"
)
