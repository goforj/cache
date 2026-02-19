package cache

// Driver identifies cache backend.
type Driver string

const (
	DriverMemory Driver = "memory"
	DriverRedis  Driver = "redis"
	DriverFile   Driver = "file"
)
