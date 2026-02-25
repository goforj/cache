package cache

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExamplesBuild(t *testing.T) {
	t.Parallel()
	examplesDir := "examples"

	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("cannot read examples directory: %v", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		// CAPTURE LOOP VARS
		name := e.Name()
		path := filepath.Join(examplesDir, name)

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := buildExampleWithoutTags(path); err != nil {
				t.Fatalf("example %q failed to build:\n%s", name, err)
			}
		})
	}
}

func abs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		panic(err)
	}
	return a
}

func buildExampleWithoutTags(exampleDir string) error {
	orig := filepath.Join(exampleDir, "main.go")

	src, err := os.ReadFile(orig)
	if err != nil {
		return fmt.Errorf("read main.go: %w", err)
	}

	clean := stripBuildTags(src)

	tmpDir, err := os.MkdirTemp("", "example-overlay-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(tmpFile, clean, 0644); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(exampleBuildGoMod()), 0644); err != nil {
		return err
	}

	overlay := map[string]any{
		"Replace": map[string]string{
			abs(orig): abs(tmpFile),
		},
	}

	overlayJSON, err := json.Marshal(overlay)
	if err != nil {
		return err
	}

	overlayPath := filepath.Join(tmpDir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlayJSON, 0644); err != nil {
		return err
	}

	cmd := exec.Command(
		"go", "build",
		"-mod=mod",
		"-overlay", overlayPath,
		"-o", os.DevNull,
		".",
	)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "GOWORK=off")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return errors.New(stderr.String())
	}

	return nil
}

func exampleBuildGoMod() string {
	root := abs(".")
	sep := string(filepath.Separator)
	rootSlash := filepath.ToSlash(root)
	if runtime.GOOS == "windows" {
		// keep local replaces forward-slashed in go.mod for portability/parsing
		rootSlash = strings.ReplaceAll(root, sep, "/")
	}
	lines := []string{
		"module examplebuild",
		"",
		"go 1.24.4",
		"",
		"require (",
		"\tgithub.com/goforj/cache v0.0.0",
		"\tgithub.com/goforj/cache/cachecore v0.0.0",
		"\tgithub.com/goforj/cache/driver/rediscache v0.0.0",
		"\tgithub.com/goforj/cache/driver/memcachedcache v0.0.0",
		"\tgithub.com/goforj/cache/driver/natscache v0.0.0",
		"\tgithub.com/goforj/cache/driver/dynamocache v0.0.0",
		"\tgithub.com/goforj/cache/driver/sqlcore v0.0.0",
		"\tgithub.com/goforj/cache/driver/sqlitecache v0.0.0",
		"\tgithub.com/goforj/cache/driver/postgrescache v0.0.0",
		"\tgithub.com/goforj/cache/driver/mysqlcache v0.0.0",
		")",
		"",
		"replace github.com/goforj/cache => " + rootSlash,
		"replace github.com/goforj/cache/cachecore => " + rootSlash + "/cachecore",
		"replace github.com/goforj/cache/driver/rediscache => " + rootSlash + "/driver/rediscache",
		"replace github.com/goforj/cache/driver/memcachedcache => " + rootSlash + "/driver/memcachedcache",
		"replace github.com/goforj/cache/driver/natscache => " + rootSlash + "/driver/natscache",
		"replace github.com/goforj/cache/driver/dynamocache => " + rootSlash + "/driver/dynamocache",
		"replace github.com/goforj/cache/driver/sqlcore => " + rootSlash + "/driver/sqlcore",
		"replace github.com/goforj/cache/driver/sqlitecache => " + rootSlash + "/driver/sqlitecache",
		"replace github.com/goforj/cache/driver/postgrescache => " + rootSlash + "/driver/postgrescache",
		"replace github.com/goforj/cache/driver/mysqlcache => " + rootSlash + "/driver/mysqlcache",
		"",
	}
	return strings.Join(lines, "\n")
}

func stripBuildTags(src []byte) []byte {
	lines := strings.Split(string(src), "\n")

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "//go:build") ||
			strings.HasPrefix(line, "// +build") ||
			line == "" {
			i++
			continue
		}

		break
	}

	return []byte(strings.Join(lines[i:], "\n"))
}
