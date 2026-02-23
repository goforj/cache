package cachecore

// Driver identifies cache backend.
type Driver string

const (
	DriverNull      Driver = "null"
	DriverFile      Driver = "file"
	DriverMemory    Driver = "memory"
	DriverMemcached Driver = "memcached"
	DriverDynamo    Driver = "dynamodb"
	DriverSQL       Driver = "sql"
	DriverRedis     Driver = "redis"
	DriverNATS      Driver = "nats"
)
