package cache

// Driver identifies cache backend.
type Driver string

const (
	DriverNull      Driver = "null"
	DriverFile      Driver = "file"
	DriverMemory    Driver = "memory"
	DriverMemcached Driver = "memcached"
	DriverRedis     Driver = "redis"
)
