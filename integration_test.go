//go:build integration

package cache

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"net"

	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var integrationRedis struct {
	container testcontainers.Container
	addr      string
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	drivers := selectedIntegrationDrivers()
	needsRedis := drivers["redis"]

	if needsRedis {
		redisContainer, redisAddr, err := startRedisContainer(ctx)
		if err != nil {
			// Surface error and exit early to avoid running partial suites.
			_, _ = os.Stderr.WriteString("failed to start redis integration container: " + err.Error() + "\n")
			os.Exit(1)
		}
		integrationRedis.container = redisContainer
		integrationRedis.addr = redisAddr
	}

	exitCode := m.Run()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if integrationRedis.container != nil {
		_ = integrationRedis.container.Terminate(shutdownCtx)
	}

	os.Exit(exitCode)
}

// selectedIntegrationDrivers chooses which drivers run under integration tag.
// INTEGRATION_DRIVER may be "all" (default) or a comma-separated list such as "redis,memory".
func selectedIntegrationDrivers() map[string]bool {
	selected := map[string]bool{
		"memory": true,
		"redis":  true,
	}
	value := strings.TrimSpace(strings.ToLower(os.Getenv("INTEGRATION_DRIVER")))
	if value == "" || value == "all" {
		return selected
	}
	for key := range selected {
		selected[key] = false
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		selected[part] = true
	}
	return selected
}

func integrationDriverEnabled(name string) bool {
	return selectedIntegrationDrivers()[strings.ToLower(name)]
}

func startRedisContainer(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}
	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}
	return container, net.JoinHostPort(host, port.Port()), nil
}
