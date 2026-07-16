package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// main exits nonzero on generation failures so CI cannot accept stale or partial examples.
func main() {
	if err := run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println("✔ Examples generated in ./examples/")
}

// run keeps generator orchestration separate from process exit so tests can exercise failures.
func run() error {
	root, err := findRoot()
	if err != nil {
		return err
	}

	examplesDir := filepath.Join(root, "examples")
	if err := os.MkdirAll(examplesDir, 0o755); err != nil {
		return err
	}

	modPath, err := modulePath(root)
	if err != nil {
		return err
	}

	funcs := map[string]*FuncDoc{}
	if err := collectExamplesFromDir(funcs, root, modPath, ""); err != nil {
		return err
	}
	if err := collectExamplesFromDir(funcs, filepath.Join(root, "cachefake"), modPath+"/cachefake", ""); err != nil {
		return err
	}
	for _, rel := range []string{
		"driver/rediscache",
		"driver/memcachedcache",
		"driver/natscache",
		"driver/dynamocache",
		"driver/sqlcore",
		"driver/sqlitecache",
		"driver/postgrescache",
		"driver/mysqlcache",
	} {
		dir := filepath.Join(root, rel)
		driverModPath, err := modulePath(dir)
		if err != nil {
			return err
		}
		if err := collectExamplesFromDir(funcs, dir, driverModPath, ""); err != nil {
			return err
		}
	}

	for _, fd := range funcs {
		sort.Slice(fd.Examples, func(i, j int) bool {
			return fd.Examples[i].Line < fd.Examples[j].Line
		})
	}

	for _, fd := range funcs {
		if err := writeMain(examplesDir, fd, fd.ImportPath); err != nil {
			return err
		}
	}
	if err := ensureMainComments(examplesDir); err != nil {
		return err
	}

	return nil
}

// findRoot accepts maintainer entry points from the repository or docs tree without changing generated paths.
func findRoot() (string, error) {
	wd, _ := os.Getwd()
	for _, c := range []string{wd, filepath.Join(wd, ".."), filepath.Join(wd, "..", ".."), filepath.Join(wd, "..", "..", "..")} {
		c = filepath.Clean(c)
		if fileExists(filepath.Join(c, "go.mod")) && fileExists(filepath.Join(c, "factory.go")) && fileExists(filepath.Join(c, "README.md")) {
			return c, nil
		}
	}
	return "", fmt.Errorf("could not find project root")
}

// fileExists treats absent markers as candidate misses while root discovery probes parent directories.
func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// modulePath derives generated imports from the manifest so forks and module-path changes remain buildable.
func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}

	return "", fmt.Errorf("module path not found in go.mod")
}

// collectExamplesFromDir coalesces declarations by slug so each documented API owns one executable directory.
func collectExamplesFromDir(funcs map[string]*FuncDoc, dir, importPath, slugPrefix string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	pkgName, err := selectPackage(pkgs)
	if err != nil {
		return err
	}
	pkg, ok := pkgs[pkgName]
	if !ok {
		return fmt.Errorf(`package %q not found in %s`, pkgName, dir)
	}

	prefix := slugPrefix
	if prefix == "" && pkgName != "cache" {
		prefix = pkgName + "_"
	}

	for filename, file := range pkg.Files {
		if strings.Contains(filename, "_test.go") {
			continue
		}
		for name, fd := range extractFuncDocs(fset, filename, file) {
			fd.ImportPath = importPath
			if prefix != "" {
				fd.Slug = prefix + strings.ToLower(fd.Slug)
				name = fd.Slug
			}
			if existing, ok := funcs[name]; ok {
				existing.Examples = append(existing.Examples, fd.Examples...)
			} else {
				funcs[name] = fd
			}
		}
	}

	return nil
}

// FuncDoc carries the declaration metadata needed for stable one-directory-per-API output.
type FuncDoc struct {
	Name        string
	Slug        string
	ImportPath  string
	Group       string
	Description string
	Examples    []Example
}

// Example retains source order and labels so regeneration remains deterministic.
type Example struct {
	FuncName string
	File     string
	Label    string
	Line     int
	Code     string
}

var exampleHeader = regexp.MustCompile(`(?i)^\s*Example:\s*(.*)$`)
var groupHeader = regexp.MustCompile(`(?i)^\s*@group\s+(.+)$`)

const (
	generatedMainComment       = "// main keeps this generated example executable so API drift fails during compilation."
	legacyGeneratedMainComment = "// main runs this generated API example as a standalone program."
)

type docLine struct {
	text string
	pos  token.Pos
}

// extractFuncDocs limits output to exported declarations so generated docs cannot expose package internals.
func extractFuncDocs(
	fset *token.FileSet,
	filename string,
	file *ast.File,
) map[string]*FuncDoc {

	out := map[string]*FuncDoc{}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Doc == nil {
			continue
		}

		name := fn.Name.Name
		if !ast.IsExported(name) {
			continue
		}

		slug := funcSlug(fn)
		out[slug] = &FuncDoc{
			Name:        name,
			Slug:        slug,
			Group:       extractGroup(fn.Doc),
			Description: extractFuncDescription(fn.Doc),
			Examples:    extractBlocks(fset, filename, name, fn),
		}
	}

	return out
}

// funcSlug includes receiver identity so methods cannot collide with package functions or other receiver types.
func funcSlug(fn *ast.FuncDecl) string {
	name := fn.Name.Name
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return name
	}
	recv := recvTypeName(fn.Recv.List[0].Type)
	if recv == "" {
		return name
	}
	return recv + "_" + name
}

// recvTypeName normalizes pointer and generic receivers so equivalent declarations share a stable slug.
func recvTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.IndexExpr:
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	default:
		return ""
	}
}

// extractGroup defaults ungrouped declarations consistently so API sections remain stable across regeneration.
func extractGroup(group *ast.CommentGroup) string {
	lines := docLines(group)

	for _, dl := range lines {
		trimmed := strings.TrimSpace(dl.text)
		if m := groupHeader.FindStringSubmatch(trimmed); m != nil {
			return strings.TrimSpace(m[1])
		}
	}

	return "Other"
}

// extractFuncDescription isolates prose so generated summaries cannot absorb directives or runnable snippets.
func extractFuncDescription(group *ast.CommentGroup) string {
	lines := docLines(group)
	var desc []string

	for _, dl := range lines {
		trimmed := strings.TrimSpace(dl.text)

		// Directives and runnable snippets belong in separate generated sections, not the prose summary.
		if exampleHeader.MatchString(trimmed) || groupHeader.MatchString(trimmed) {
			break
		}

		if len(desc) == 0 && trimmed == "" {
			continue
		}

		desc = append(desc, dl.text)
	}

	for len(desc) > 0 && strings.TrimSpace(desc[len(desc)-1]) == "" {
		desc = desc[:len(desc)-1]
	}

	return strings.Join(desc, "\n")
}

// docLines retains source positions so examples with identical labels still sort by declaration order.
func docLines(group *ast.CommentGroup) []docLine {
	var lines []docLine

	for _, c := range group.List {
		text := c.Text

		if strings.HasPrefix(text, "//") {
			line := strings.TrimPrefix(text, "//")
			if strings.HasPrefix(line, " ") {
				line = line[1:]
			}
			if strings.HasPrefix(line, "\t") {
				line = line[1:]
			}
			lines = append(lines, docLine{
				text: line,
				pos:  c.Slash,
			})
		}
	}

	return lines
}

// extractBlocks separates labeled snippets so each documented case can remain independently runnable.
func extractBlocks(
	fset *token.FileSet,
	filename, funcName string,
	fn *ast.FuncDecl,
) []Example {

	var out []Example
	lines := docLines(fn.Doc)

	var label string
	var collected []string
	var startLine int
	inExample := false

	flush := func() {
		if len(collected) == 0 {
			return
		}

		out = append(out, Example{
			FuncName: funcName,
			File:     filename,
			Label:    label,
			Line:     startLine,
			Code:     strings.Join(collected, "\n"),
		})

		collected = nil
		label = ""
		inExample = false
	}

	for _, dl := range lines {
		raw := dl.text
		trimmed := strings.TrimSpace(raw)

		if m := exampleHeader.FindStringSubmatch(trimmed); m != nil {
			flush()
			inExample = true
			label = strings.TrimSpace(m[1])
			startLine = fset.Position(dl.pos).Line
			continue
		}

		if !inExample {
			continue
		}

		collected = append(collected, raw)
	}

	flush()
	return out
}

// selectPackage resolves mixed package directories deterministically so test or command packages cannot steal the API index.
// Strategy:
//  1. If only one package exists, use it.
//  2. Prefer the non-"main" package with the most files.
//  3. Fall back to the first package alphabetically.
func selectPackage(pkgs map[string]*ast.Package) (string, error) {
	if len(pkgs) == 0 {
		return "", fmt.Errorf("no packages found")
	}

	if len(pkgs) == 1 {
		for name := range pkgs {
			return name, nil
		}
	}

	type candidate struct {
		name  string
		count int
	}

	candidates := make([]candidate, 0, len(pkgs))
	for name, pkg := range pkgs {
		candidates = append(candidates, candidate{
			name:  name,
			count: len(pkg.Files),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count == candidates[j].count {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].count > candidates[j].count
	})

	for _, cand := range candidates {
		if cand.name != "main" {
			return cand.name, nil
		}
	}

	return candidates[0].name, nil
}

// writeMain retains explicit opt-out tags so examples requiring external services do not enter default builds.
func writeMain(base string, fd *FuncDoc, importPath string) error {
	if len(fd.Examples) == 0 {
		return nil
	}

	if importPath == "" {
		return fmt.Errorf("import path cannot be empty")
	}

	slug := fd.Slug
	if slug == "" {
		slug = fd.Name
	}
	dir := filepath.Join(base, strings.ToLower(slug))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer

	target := filepath.Join(dir, "main.go")
	current, err := os.ReadFile(target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if bytes.HasPrefix(current, []byte("//go:build ignore\n")) {
		// Existing opt-out tags are retained because some examples require external services.
		buf.WriteString("//go:build ignore\n// +build ignore\n\n")
	}
	buf.WriteString("package main\n\n")

	imports := map[string]bool{
		importPath: true,
	}
	blankImports := map[string]bool{}

	for _, ex := range fd.Examples {
		if strings.Contains(ex.Code, "fmt.") {
			imports["fmt"] = true
		}
		if strings.Contains(ex.Code, "json.") {
			imports["encoding/json"] = true
		}
		if strings.Contains(ex.Code, "strings.") {
			imports["strings"] = true
		}
		if strings.Contains(ex.Code, "os.") {
			imports["os"] = true
		}
		if strings.Contains(ex.Code, "context.") {
			imports["context"] = true
		}
		if strings.Contains(ex.Code, "testing.") {
			imports["testing"] = true
		}
		if strings.Contains(ex.Code, "cachecore.") {
			imports["github.com/goforj/cache/cachecore"] = true
		}
		if strings.Contains(ex.Code, "regexp.") {
			imports["regexp"] = true
		}
		if strings.Contains(ex.Code, "syscall.") {
			imports["syscall"] = true
		}
		if strings.Contains(ex.Code, "redis.") ||
			strings.Contains(ex.Code, "WithRedisClient") ||
			strings.Contains(ex.Code, "redisClient") ||
			strings.Contains(ex.Code, "RedisClient") {
			imports["github.com/redis/go-redis/v9"] = true
		}
		if strings.Contains(ex.Code, "time.") {
			imports["time"] = true
		}
		if strings.Contains(ex.Code, "gocron") {
			imports["github.com/go-co-op/gocron/v2"] = true
		}
		if strings.Contains(ex.Code, "scheduler") {
			imports["github.com/goforj/scheduler"] = true
		}
		if strings.Contains(ex.Code, "filepath.") {
			imports["path/filepath"] = true
		}
		if strings.Contains(ex.Code, "godump.") {
			imports["github.com/goforj/godump"] = true
		}
		if strings.Contains(ex.Code, "rand.") {
			imports["crypto/rand"] = true
		}
		if strings.Contains(ex.Code, "base64.") {
			imports["encoding/base64"] = true
		}
		if strings.Contains(ex.Code, "sqlcore.") {
			switch {
			case strings.Contains(ex.Code, `DriverName: "sqlite"`):
				blankImports["modernc.org/sqlite"] = true
			case strings.Contains(ex.Code, `DriverName: "postgres"`), strings.Contains(ex.Code, `DriverName: "pgx"`):
				blankImports["github.com/jackc/pgx/v5/stdlib"] = true
			case strings.Contains(ex.Code, `DriverName: "mysql"`):
				blankImports["github.com/go-sql-driver/mysql"] = true
			}
		}
	}

	if len(imports) == 1 {
		buf.WriteString("import ")
		for imp := range imports {
			buf.WriteString(fmt.Sprintf("%q", imp))
		}
		buf.WriteString("\n\n")
	} else {
		buf.WriteString("import (\n")
		keys := make([]string, 0, len(imports))
		for k := range imports {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, imp := range keys {
			buf.WriteString("\t\"" + imp + "\"\n")
		}
		if len(blankImports) > 0 {
			blankKeys := make([]string, 0, len(blankImports))
			for k := range blankImports {
				blankKeys = append(blankKeys, k)
			}
			sort.Strings(blankKeys)
			for _, imp := range blankKeys {
				buf.WriteString("\t_ \"" + imp + "\"\n")
			}
		}
		buf.WriteString(")\n\n")
	}

	buf.WriteString(generatedMainComment + "\n")
	buf.WriteString("func main() {\n")

	if fd.Description != "" {
		for _, line := range strings.Split(fd.Description, "\n") {
			buf.WriteString("\t// " + line + "\n")
		}
		buf.WriteString("\n")
	}

	for _, ex := range fd.Examples {
		if ex.Label != "" {
			buf.WriteString("\t// Example: " + ex.Label + "\n")
		}

		ex.Code = strings.TrimLeft(ex.Code, "\n")

		for _, line := range strings.Split(ex.Code, "\n") {
			if strings.TrimSpace(line) == "" {
				buf.WriteString("\n")
			} else {
				buf.WriteString("\t" + line + "\n")
			}
		}
	}

	buf.WriteString("}\n")

	formatted, err := formatGeneratedSource(target, buf.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile(target, formatted, 0o644)
}

// ensureMainComments migrates legacy output so every compile-checked example follows the current comment contract.
func ensureMainComments(base string) error {
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(base, entry.Name(), "main.go")
		contents, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		declaration := []byte("func main() {")
		updated := bytes.ReplaceAll(contents, []byte(legacyGeneratedMainComment+"\n"), nil)
		updated = bytes.ReplaceAll(updated, []byte(generatedMainComment+"\n"), nil)
		updated, err = formatGeneratedSource(path, updated)
		if err != nil {
			return err
		}
		index := bytes.Index(updated, declaration)
		if index < 0 {
			continue
		}
		withComment := make([]byte, 0, len(updated)+len(generatedMainComment)+1)
		withComment = append(withComment, updated[:index]...)
		withComment = append(withComment, generatedMainComment...)
		withComment = append(withComment, '\n')
		withComment = append(withComment, updated[index:]...)
		updated = withComment
		formatted, err := formatGeneratedSource(path, updated)
		if err != nil {
			return err
		}
		if bytes.Equal(contents, formatted) {
			continue
		}
		if err := os.WriteFile(path, formatted, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// formatGeneratedSource makes generated examples deterministic and rejects snippets that do not form valid Go source.
func formatGeneratedSource(target string, source []byte) ([]byte, error) {
	formatted, err := format.Source(source)
	if err != nil {
		return nil, fmt.Errorf("format generated example %s: %w", target, err)
	}
	return formatted, nil
}
