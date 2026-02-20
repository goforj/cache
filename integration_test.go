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
	"redis":        startRedisContainer,
	"memcached":    startMemcachedContainer,
	"dynamodb":     startDynamoContainer,
	"sql_postgres": startPostgresContainer,
	"sql_mysql":    startMySQLContainer,
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
		"null":         true,
		"file":         true,
		"memory":       true,
		"memcached":    true,
		"dynamodb":     true,
		"sql_sqlite":   true,
		"sql_postgres": true,
		"sql_mysql":    true,
		"redis":        true,
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

func startMemcachedContainer(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "memcached:alpine",
		ExposedPorts: []string{"11211/tcp"},
		WaitingFor:   wait.ForListeningPort("11211/tcp").WithStartupTimeout(30 * time.Second),
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
	port, err := container.MappedPort(ctx, "11211/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}
	return container, net.JoinHostPort(host, port.Port()), nil
}

func startDynamoContainer(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "amazon/dynamodb-local:latest",
		ExposedPorts: []string{"8000/tcp"},
		WaitingFor:   wait.ForListeningPort("8000/tcp").WithStartupTimeout(45 * time.Second),
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
	port, err := container.MappedPort(ctx, "8000/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}
	return container, "http://" + net.JoinHostPort(host, port.Port()), nil
}

func startPostgresContainer(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		Env:          map[string]string{"POSTGRES_PASSWORD": "pass", "POSTGRES_USER": "user", "POSTGRES_DB": "app"},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
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
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}
	return container, net.JoinHostPort(host, port.Port()), nil
}

func startMySQLContainer(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "mysql:8",
		Env:          map[string]string{"MYSQL_ROOT_PASSWORD": "pass", "MYSQL_DATABASE": "app", "MYSQL_USER": "user", "MYSQL_PASSWORD": "pass"},
		ExposedPorts: []string{"3306/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("3306/tcp").WithStartupTimeout(90*time.Second),
			wait.ForLog("ready for connections").WithOccurrence(2).WithStartupTimeout(90*time.Second),
		),
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
	port, err := container.MappedPort(ctx, "3306/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}
	return container, net.JoinHostPort(host, port.Port()), nil
}
