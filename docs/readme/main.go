//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	apiStart       = "<!-- api:embed:start -->"
	apiEnd         = "<!-- api:embed:end -->"
	testCountStart = "<!-- test-count:embed:start -->"
	testCountEnd   = "<!-- test-count:embed:end -->"
)

func main() {
	if err := run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println("✔ API section updated in README.md")
}

func run() error {
	root, err := findRoot()
	if err != nil {
		return err
	}

	funcs, err := parseFuncs(root)
	if err != nil {
		return err
	}

	api := renderAPI(funcs)

	readmePath := filepath.Join(root, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return err
	}

	out, err := replaceAPISection(string(data), api)
	if err != nil {
		return err
	}

	return os.WriteFile(readmePath, []byte(out), 0o644)
}

//
// ------------------------------------------------------------
// Data model
// ------------------------------------------------------------
//

type FuncDoc struct {
	Key         string
	Name        string
	DisplayName string
	Anchor      string
	Group       string
	Behavior    string
	Fluent      string
	Description string
	Examples    []Example
}

type Example struct {
	Label string
	Code  string
	Line  int
}

//
// ------------------------------------------------------------
// Parsing
// ------------------------------------------------------------
//

var (
	groupHeader    = regexp.MustCompile(`(?i)^\s*@group\s+(.+)$`)
	behaviorHeader = regexp.MustCompile(`(?i)^\s*@behavior\s+(.+)$`)
	fluentHeader   = regexp.MustCompile(`(?i)^\s*@fluent\s+(.+)$`)
	exampleHeader  = regexp.MustCompile(`(?i)^\s*Example:\s*(.*)$`)
)

func parseFuncs(root string) ([]*FuncDoc, error) {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(
		fset,
		root,
		func(info os.FileInfo) bool {
			return !strings.HasSuffix(info.Name(), "_test.go")
		},
		parser.ParseComments,
	)
	if err != nil {
		return nil, err
	}

	pkgName, err := selectPackage(pkgs)
	if err != nil {
		return nil, err
	}

	pkg, ok := pkgs[pkgName]
	if !ok {
		return nil, fmt.Errorf(`package %q not found`, pkgName)
	}

	funcs := map[string]*FuncDoc{}

	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Doc == nil {
				continue
			}

			if !ast.IsExported(fn.Name.Name) {
				continue
			}

			receiver := extractReceiverName(fn)
			displayName := fn.Name.Name
			anchor := strings.ToLower(fn.Name.Name)
			key := fn.Name.Name
			if receiver != "" {
				displayName = receiver + "." + fn.Name.Name
				anchor = strings.ToLower(receiver + "-" + fn.Name.Name)
				key = receiver + "." + fn.Name.Name
			}

			fd := &FuncDoc{
				Key:         key,
				Name:        fn.Name.Name,
				DisplayName: displayName,
				Anchor:      anchor,
				Group:       extractGroup(fn.Doc),
				Behavior:    extractBehavior(fn.Doc),
				Fluent:      extractFluent(fn.Doc),
				Description: extractDescription(fn.Doc),
				Examples:    extractExamples(fset, fn),
			}

			if existing, ok := funcs[fd.Key]; ok {
				existing.Examples = append(existing.Examples, fd.Examples...)
			} else {
				funcs[fd.Key] = fd
			}
		}
	}

	out := make([]*FuncDoc, 0, len(funcs))
	for _, fd := range funcs {
		sort.Slice(fd.Examples, func(i, j int) bool {
			return fd.Examples[i].Line < fd.Examples[j].Line
		})
		out = append(out, fd)
	}

	return out, nil
}

func extractGroup(group *ast.CommentGroup) string {
	for _, c := range group.List {
		line := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if m := groupHeader.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return "Other"
}

func extractBehavior(group *ast.CommentGroup) string {
	for _, c := range group.List {
		line := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if m := behaviorHeader.FindStringSubmatch(line); m != nil {
			return strings.ToLower(strings.TrimSpace(m[1]))
		}
	}
	return ""
}

func extractFluent(group *ast.CommentGroup) string {
	for _, c := range group.List {
		line := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if m := fluentHeader.FindStringSubmatch(line); m != nil {
			return strings.ToLower(strings.TrimSpace(m[1]))
		}
	}
	return ""
}

func extractDescription(group *ast.CommentGroup) string {
	var lines []string

	for _, c := range group.List {
		line := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))

		if exampleHeader.MatchString(line) ||
			groupHeader.MatchString(line) ||
			behaviorHeader.MatchString(line) ||
			fluentHeader.MatchString(line) {
			break
		}

		if len(lines) == 0 && line == "" {
			continue
		}

		lines = append(lines, line)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractReceiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	return receiverTypeName(fn.Recv.List[0].Type)
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	default:
		return ""
	}
}

func extractExamples(fset *token.FileSet, fn *ast.FuncDecl) []Example {
	var out []Example
	var current []string
	var label string
	var start int
	inExample := false

	flush := func() {
		if len(current) == 0 {
			return
		}

		out = append(out, Example{
			Label: label,
			Code:  strings.Join(normalizeIndent(current), "\n"),
			Line:  start,
		})

		current = nil
		label = ""
		inExample = false
	}

	for _, c := range fn.Doc.List {
		raw := strings.TrimPrefix(c.Text, "//")
		line := strings.TrimSpace(raw)

		if m := exampleHeader.FindStringSubmatch(line); m != nil {
			flush()
			inExample = true
			label = strings.TrimSpace(m[1])
			start = fset.Position(c.Slash).Line
			continue
		}

		if !inExample {
			continue
		}

		current = append(current, raw)
	}

	flush()
	return out
}

// selectPackage picks the primary package to document.
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

//
// ------------------------------------------------------------
// Rendering
// ------------------------------------------------------------
//

func renderAPI(funcs []*FuncDoc) string {
	byGroup := map[string][]*FuncDoc{}
	nameCounts := map[string]int{}
	byKey := map[string]*FuncDoc{}
	hide := map[string]bool{}
	hasCtxVariant := map[string]bool{}

	for _, fd := range funcs {
		nameCounts[fd.Name]++
		byKey[fd.Key] = fd
	}

	for _, fd := range funcs {
		if !isHideableCtxVariant(fd) {
			continue
		}
		baseKey, ok := ctxBaseKey(fd)
		if !ok {
			continue
		}
		if _, ok := byKey[baseKey]; !ok {
			continue
		}
		hide[fd.Key] = true
		hasCtxVariant[baseKey] = true
	}

	for _, fd := range funcs {
		if hide[fd.Key] {
			continue
		}
		byGroup[fd.Group] = append(byGroup[fd.Group], fd)
	}

	groupNames := make([]string, 0, len(byGroup))
	for g := range byGroup {
		groupNames = append(groupNames, g)
	}
	sort.Strings(groupNames)

	var buf bytes.Buffer

	// ---------------- Index ----------------
	buf.WriteString("## API Index\n\n")
	buf.WriteString("| Group | Functions |\n")
	buf.WriteString("|------:|:-----------|\n")

	for _, group := range groupNames {
		sort.Slice(byGroup[group], func(i, j int) bool {
			li := renderLabel(byGroup[group][i], nameCounts, hasCtxVariant)
			lj := renderLabel(byGroup[group][j], nameCounts, hasCtxVariant)
			if li == lj {
				return byGroup[group][i].Anchor < byGroup[group][j].Anchor
			}
			return li < lj
		})

		var links []string
		for _, fn := range byGroup[group] {
			links = append(links, fmt.Sprintf("[%s](#%s)", renderName(fn, nameCounts), fn.Anchor))
		}

		buf.WriteString(fmt.Sprintf("| **%s** | %s |\n",
			group,
			strings.Join(links, " "),
		))
	}

	buf.WriteString("\n\n")
	buf.WriteString("_Examples assume `ctx := context.Background()` and `c := cache.NewCache(cache.NewMemoryStore(ctx))` unless shown otherwise._\n\n")

	// ---------------- Details ----------------
	for _, group := range groupNames {
		buf.WriteString("## " + group + "\n\n")

		for _, fn := range byGroup[group] {
			anchor := fn.Anchor

			header := renderLabel(fn, nameCounts, hasCtxVariant)
			if fn.Behavior != "" {
				header += " · " + fn.Behavior
			}
			if fn.Fluent == "true" {
				header += " · fluent"
			}

			buf.WriteString(fmt.Sprintf("### <a id=\"%s\"></a>%s\n\n", anchor, header))

			if fn.Description != "" {
				buf.WriteString(fn.Description + "\n\n")
			}

			for _, ex := range fn.Examples {
				if ex.Label != "" && len(fn.Examples) > 1 {
					buf.WriteString(fmt.Sprintf("_Example: %s_\n\n", ex.Label))
				}

				buf.WriteString("```go\n")
				for _, line := range strings.Split(ex.Code, "\n") {
					trimmed := strings.TrimSpace(line)
					if trimmed == "" {
						continue
					}
					if trimmed == "ctx := context.Background()" {
						continue
					}
					if trimmed == "c := cache.NewCache(cache.NewMemoryStore(ctx))" {
						continue
					}
					if strings.HasPrefix(trimmed, "_ =") {
						continue
					}
					buf.WriteString(line + "\n")
				}
				buf.WriteString("```\n\n")
			}
		}
	}

	return strings.TrimRight(buf.String(), "\n")
}

func renderName(fn *FuncDoc, nameCounts map[string]int) string {
	if nameCounts[fn.Name] <= 1 {
		return fn.Name
	}
	return fn.DisplayName
}

func renderLabel(fn *FuncDoc, nameCounts map[string]int, hasCtxVariant map[string]bool) string {
	name := renderName(fn, nameCounts)
	if hasCtxVariant[fn.Key] {
		return name + " (+Ctx)"
	}
	return name
}

func isHideableCtxVariant(fn *FuncDoc) bool {
	if !strings.HasSuffix(fn.Name, "Ctx") {
		return false
	}
	desc := strings.TrimSpace(strings.ToLower(fn.Description))
	if desc == "" {
		return false
	}
	return strings.Contains(desc, "context-aware variant")
}

func ctxBaseKey(fn *FuncDoc) (string, bool) {
	if !strings.HasSuffix(fn.Name, "Ctx") || !strings.HasSuffix(fn.Key, "Ctx") {
		return "", false
	}
	return strings.TrimSuffix(fn.Key, "Ctx"), true
}

//
// ------------------------------------------------------------
// README replacement
// ------------------------------------------------------------
//

func replaceAPISection(readme, api string) (string, error) {
	start := strings.Index(readme, apiStart)
	end := strings.Index(readme, apiEnd)

	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("API anchors not found or malformed")
	}

	var out bytes.Buffer
	out.WriteString(readme[:start+len(apiStart)])
	out.WriteString("\n\n")
	out.WriteString(api)
	out.WriteString("\n")
	out.WriteString(readme[end:])

	return out.String(), nil
}

type TestCounts struct {
	UnitFuncs                 int
	IntegrationFuncs          int
	UnitStaticSubtests        int
	IntegrationStaticSubtests int
}

type ContractMatrix struct {
	Drivers           int
	Cases             int
	InvariantSubtests int
}

func countTests(root string) (TestCounts, error) {
	var counts TestCounts

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

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return err
		}

		isIntegration := hasIntegrationBuildTag(src)
		subtests := countLiteralSubtests(file)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			if isIntegration {
				counts.IntegrationFuncs++
			} else {
				counts.UnitFuncs++
			}
		}
		if isIntegration {
			counts.IntegrationStaticSubtests += subtests
		} else {
			counts.UnitStaticSubtests += subtests
		}
		return nil
	})
	if err != nil {
		return TestCounts{}, err
	}

	return counts, nil
}

func countLiteralSubtests(file *ast.File) int {
	count := 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil || sel.Sel.Name != "Run" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if _, ok := call.Args[1].(*ast.FuncLit); !ok {
			return true
		}
		count++
		return true
	})
	return count
}

func readContractMatrix(root string) (ContractMatrix, error) {
	path := filepath.Join(root, "store_contract_integration_test.go")
	src, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ContractMatrix{}, nil
		}
		return ContractMatrix{}, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return ContractMatrix{}, err
	}

	var m ContractMatrix
	invariantFns := map[string]bool{
		"runCacheHelperInvariantSuite":         true,
		"runLockHelperInvariantSuite":          true,
		"runRateLimitHelperInvariantSuite":     true,
		"runRefreshAheadHelperInvariantSuite":  true,
		"runRememberStaleDeeperInvariantSuite": true,
		"runBatchHelperInvariantSuite":         true,
		"runCounterHelperInvariantSuite":       true,
		"runDriverFactoryInvariantSuite":       true,
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		switch fn.Name.Name {
		case "integrationContractCases":
			m.Cases = countReturnedCompositeElements(fn, "contractCase")
		case "integrationFixtures":
			m.Drivers = countCompositeLitsByType(fn.Body, "storeFactory")
		default:
			if invariantFns[fn.Name.Name] {
				m.InvariantSubtests += countLiteralSubtestsInBlock(fn.Body)
			}
		}
	}

	return m, nil
}

func countReturnedCompositeElements(fn *ast.FuncDecl, elemType string) int {
	if fn.Body == nil {
		return 0
	}
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		cl, ok := ret.Results[0].(*ast.CompositeLit)
		if !ok {
			continue
		}
		arr, ok := cl.Type.(*ast.ArrayType)
		if !ok {
			continue
		}
		id, ok := arr.Elt.(*ast.Ident)
		if !ok || id.Name != elemType {
			continue
		}
		return len(cl.Elts)
	}
	return 0
}

func countCompositeLitsByType(body *ast.BlockStmt, typeName string) int {
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		id, ok := cl.Type.(*ast.Ident)
		if ok && id.Name == typeName {
			count++
		}
		return true
	})
	return count
}

func countLiteralSubtestsInBlock(body *ast.BlockStmt) int {
	if body == nil {
		return 0
	}
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil || sel.Sel.Name != "Run" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if _, ok := call.Args[1].(*ast.FuncLit); !ok {
			return true
		}
		count++
		return true
	})
	return count
}

func updateTestsSection(readme string, tests TestCounts, matrix ContractMatrix) (string, error) {
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

	_ = matrix
	lines := []string{
		fmt.Sprintf("    <img src=\"https://img.shields.io/badge/unit_tests-%d-brightgreen\" alt=\"Unit tests (static count of Test funcs)\">", tests.UnitFuncs),
		fmt.Sprintf("    <img src=\"https://img.shields.io/badge/integration_tests-%d-blue\" alt=\"Integration tests (static count of Test funcs)\">", tests.IntegrationFuncs),
	}
	badge := leading + strings.Join(lines, "\n") + "\n"

	return before + badge + after, nil
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

//
// ------------------------------------------------------------
// Helpers
// ------------------------------------------------------------
//

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

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func normalizeIndent(lines []string) []string {
	min := -1

	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if min == -1 || n < min {
			min = n
		}
	}

	if min <= 0 {
		return lines
	}

	out := make([]string, len(lines))
	for i, l := range lines {
		if len(l) >= min {
			out[i] = l[min:]
		} else {
			out[i] = strings.TrimLeft(l, " \t")
		}
	}

	return out
}
