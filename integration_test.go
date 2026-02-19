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

// integrationBackends define how to start infrastructure per driver.
// Add new drivers here with a testcontainers-backed start function.
var integrationBackends = map[string]func(context.Context) (testcontainers.Container, string, error){
	"redis": startRedisContainer,
}

type integrationRuntime struct {
	container testcontainers.Container
	addr      string
}

var integrationRuntimes = map[string]*integrationRuntime{}

func TestMain(m *testing.M) {
	ctx := context.Background()
	drivers := selectedIntegrationDrivers()

	for name, enabled := range drivers {
		if !enabled {
			continue
		}
		start, ok := integrationBackends[name]
		if !ok {
			continue // driver without infra (e.g., memory)
		}
		container, addr, err := start(ctx)
		if err != nil {
			_, _ = os.Stderr.WriteString("failed to start " + name + " integration container: " + err.Error() + "\n")
			os.Exit(1)
		}
		integrationRuntimes[name] = &integrationRuntime{container: container, addr: addr}
	}

	exitCode := m.Run()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, rt := range integrationRuntimes {
		if rt != nil && rt.container != nil {
			_ = rt.container.Terminate(shutdownCtx)
		}
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

// integrationAddr returns the bound host:port for a driver started via testcontainers.
func integrationAddr(name string) string {
	if rt, ok := integrationRuntimes[strings.ToLower(name)]; ok && rt != nil {
		return rt.addr
	}
	return ""
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
