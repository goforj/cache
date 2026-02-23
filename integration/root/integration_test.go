//go:build integration

package cache_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

type integrationContainer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context, timeout *time.Duration) error
}

type integrationRuntime struct {
	container integrationContainer
	addr      string
}

var integrationRuntimes = map[string]*integrationRuntime{}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// selectedIntegrationDrivers chooses which drivers run under integration tag.
// INTEGRATION_DRIVER may be "all" (default) or a comma-separated list such as "memory,file".
func selectedIntegrationDrivers() map[string]bool {
	selected := map[string]bool{
		"null":   true,
		"file":   true,
		"memory": true,
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
