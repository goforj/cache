package main

import (
	"bufio"
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

// Counts summarizes executed tests for generated documentation.
type Counts struct {
	Unit        int
	Integration int
}

// main runs the documentation generator and reports failures as a nonzero process exit.
func main() {
	if err := run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println("✔ Test badges updated from executed test runs")
}

// run executes the generator workflow and returns failures to main.
func run() error {
	root, err := findRoot()
	if err != nil {
		return err
	}

	integrationDir := filepath.Join(root, "integration")

	integrationNames, err := integrationTopLevelTests(integrationDir)
	if err != nil {
		return fmt.Errorf("integration top-level tests: %w", err)
	}

	unitCount, err := countUnitRunEvents(root)
	if err != nil {
		return fmt.Errorf("count unit test runs: %w", err)
	}

	integrationCount, err := countIntegrationRunEvents(integrationDir, integrationNames)
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

// countUnitRunEvents sums independently executed tests across every non-integration module.
func countUnitRunEvents(root string) (int, error) {
	moduleDirs, err := unitModuleDirs(root)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, moduleDir := range moduleDirs {
		count, err := countRunEvents(moduleDir, nil)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

// unitModuleDirs discovers module boundaries because go test does not cross nested go.mod files.
func unitModuleDirs(root string) ([]string, error) {
	dirs := make([]string, 0)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != root && (info.Name() == ".git" || info.Name() == "vendor" || path == filepath.Join(root, "integration")) {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == "go.mod" {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

// countRunEvents executes one module and counts the test run events emitted by the Go tool.
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
	env, err := testCommandEnv(root)
	if err != nil {
		return 0, err
	}
	cmd.Env = env

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, out.String())
	}

	var total int
	for _, event := range parseJSONEvents(out.Bytes()) {
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

// countIntegrationRunEvents executes the Docker-free integration matrix and counts matching tests.
func countIntegrationRunEvents(integrationDir string, integrationPrefixes map[string]struct{}) (int, error) {
	runPattern := buildTopLevelRunPattern(integrationPrefixes)
	if runPattern == "" {
		return 0, nil
	}

	args := []string{"test", "-tags=integration", "./root", "./all", "-run", runPattern, "-count=1", "-json"}
	cmd := exec.Command("go", args...)
	cmd.Dir = integrationDir
	// Keep this badge updater Docker-free by default.
	env, err := testCommandEnv(integrationDir)
	if err != nil {
		return 0, err
	}
	cmd.Env = append(env, "INTEGRATION_DRIVER=memory,file,null,sqlitecache")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, out.String())
	}

	var total int
	for _, event := range parseJSONEvents(out.Bytes()) {
		if event.Action != "run" || event.Test == "" {
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

// testCommandEnv keeps release checks isolated while allowing pre-tag badge generation through the repository workspace.
func testCommandEnv(start string) ([]string, error) {
	goWork := "off"
	if os.Getenv("CACHE_LOCAL_SIBLINGS") == "1" {
		dir := start
		for {
			candidate := filepath.Join(dir, "go.work")
			if _, err := os.Stat(candidate); err == nil {
				goWork = candidate
				break
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("inspect Go workspace %s: %w", candidate, err)
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				return nil, fmt.Errorf("Go workspace not found above %s", start)
			}
			dir = parent
		}
	}

	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOWORK=") {
			env = append(env, entry)
		}
	}
	return append(env, "GOWORK="+goWork), nil
}

type testEvent struct {
	Action string `json:"Action"`
	Test   string `json:"Test"`
}

// parseJSONEvents derives executed test counts from Go's structured test output.
func parseJSONEvents(data []byte) []testEvent {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	events := make([]testEvent, 0)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var event testEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	return events
}

// buildTopLevelRunPattern creates an exact test filter so nested subtests do not skew counts.
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

// integrationTopLevelTests discovers integration entry points for exact event filtering.
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

// updateTestsSection replaces README test metrics with counts derived from current source.
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

// hasIntegrationBuildTag identifies tests that belong to the opt-in integration suite.
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

// findRoot locates the repository root from supported generator working directories.
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

// fileExists reports whether a candidate project marker is present.
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// _sortedKeys keeps deterministic key ordering available to generator diagnostics.
func _sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
