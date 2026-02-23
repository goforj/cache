//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	testCountStart = "<!-- test-count:embed:start -->"
	testCountEnd   = "<!-- test-count:embed:end -->"
)

type Counts struct {
	Unit        int
	Integration int
}

func main() {
	if err := run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println("✔ Test badges updated from executed test runs")
}

func run() error {
	root, err := findRoot()
	if err != nil {
		return err
	}

	integrationNames, err := integrationTopLevelTests(root)
	if err != nil {
		return fmt.Errorf("integration top-level tests: %w", err)
	}

	unitCount, err := countRunEvents(root, nil)
	if err != nil {
		return fmt.Errorf("count unit test runs: %w", err)
	}

	integrationCount, err := countRunEvents(root, integrationNames)
	if err != nil {
		return fmt.Errorf("count integration test runs: %w", err)
	}

	readmePath := filepath.Join(root, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return err
	}

	out, err := updateTestsSection(string(data), Counts{
		Unit:        unitCount,
		Integration: integrationCount,
	})
	if err != nil {
		return err
	}

	return os.WriteFile(readmePath, []byte(out), 0o644)
}

func countRunEvents(root string, integrationPrefixes map[string]struct{}) (int, error) {
	args := []string{"test", "./...", "-run", "Test", "-count=1", "-json"}
	if integrationPrefixes != nil {
		runPattern := buildTopLevelRunPattern(integrationPrefixes)
		if runPattern == "" {
			return 0, nil
		}
		args = []string{"test", "-tags=integration", "./...", "-run", runPattern, "-count=1", "-json"}
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = root

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, out.String())
	}

	var total int
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))

	for dec.More() {
		var event struct {
			Action string `json:"Action"`
			Test   string `json:"Test"`
		}
		if err := dec.Decode(&event); err != nil {
			return 0, err
		}
		if event.Action != "run" || event.Test == "" {
			continue
		}

		if integrationPrefixes == nil {
			total++
			continue
		}

		top := event.Test
		if i := strings.IndexByte(top, '/'); i >= 0 {
			top = top[:i]
		}
		if _, ok := integrationPrefixes[top]; ok {
			total++
		}
	}

	return total, nil
}

func buildTopLevelRunPattern(names map[string]struct{}) string {
	if len(names) == 0 {
		return ""
	}
	keys := _sortedKeys(names)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, regexp.QuoteMeta(k))
	}
	// Match the top-level integration test and any subtests beneath it.
	return "^(" + strings.Join(parts, "|") + ")(/.*)?$"
}

func integrationTopLevelTests(root string) (map[string]struct{}, error) {
	names := map[string]struct{}{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !hasIntegrationBuildTag(src) {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if strings.HasPrefix(fn.Name.Name, "Test") {
				names[fn.Name.Name] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return names, nil
}

func updateTestsSection(readme string, counts Counts) (string, error) {
	start := strings.Index(readme, testCountStart)
	end := strings.Index(readme, testCountEnd)
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("test count anchors not found or malformed")
	}

	before := readme[:start+len(testCountStart)]
	body := readme[start+len(testCountStart) : end]
	after := readme[end:]

	leading := ""
	if strings.HasPrefix(body, "\n") {
		leading = "\n"
	}

	lines := []string{
		fmt.Sprintf("    <img src=\"https://img.shields.io/badge/unit_tests-%d-brightgreen\" alt=\"Unit tests (executed count)\">", counts.Unit),
		fmt.Sprintf("    <img src=\"https://img.shields.io/badge/integration_tests-%d-blue\" alt=\"Integration tests (executed count)\">", counts.Integration),
	}
	return before + leading + strings.Join(lines, "\n") + "\n" + after, nil
}

func hasIntegrationBuildTag(src []byte) bool {
	lines := strings.Split(string(src), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "package ") {
			break
		}
		if strings.Contains(trimmed, "go:build") && strings.Contains(trimmed, "integration") {
			return true
		}
		if strings.HasPrefix(trimmed, "// +build") && strings.Contains(trimmed, "integration") {
			return true
		}
	}
	return false
}

func findRoot() (string, error) {
	wd, _ := os.Getwd()
	candidates := []string{wd, filepath.Join(wd, ".."), filepath.Join(wd, "..", ".."), filepath.Join(wd, "..", "..", "..")}
	for _, c := range candidates {
		c = filepath.Clean(c)
		if fileExists(filepath.Join(c, "go.mod")) && fileExists(filepath.Join(c, "factory.go")) && fileExists(filepath.Join(c, "README.md")) {
			return filepath.Clean(c), nil
		}
	}
	return "", fmt.Errorf("could not find project root from %s", wd)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func _sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
