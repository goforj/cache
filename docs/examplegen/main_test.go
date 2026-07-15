package main

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteMainFormatsGeneratedSource protects standalone examples from drifting away from canonical Go formatting.
func TestWriteMainFormatsGeneratedSource(t *testing.T) {
	base := t.TempDir()
	doc := &FuncDoc{
		Name:        "New",
		Slug:        "new",
		Description: "New builds a store.\n\nDefaults:",
		Examples: []Example{{
			Code: `fmt.Println("ready")`,
		}},
	}

	if err := writeMain(base, doc, "example.com/cache"); err != nil {
		t.Fatalf("write generated example: %v", err)
	}
	path := filepath.Join(base, "new", "main.go")
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated example: %v", err)
	}
	formatted, err := format.Source(generated)
	if err != nil {
		t.Fatalf("format generated example independently: %v", err)
	}
	if !bytes.Equal(generated, formatted) {
		t.Errorf("generated example is not gofmt-clean:\n%s", generated)
	}
	if bytes.Contains(generated, []byte("// \n")) {
		t.Error("generated blank documentation line retains trailing whitespace")
	}
	if count := bytes.Count(generated, []byte(generatedMainComment)); count != 1 {
		t.Errorf("generated main comment count = %d, want 1", count)
	}
}

// TestEnsureMainCommentsReplacesLegacyComment keeps regenerated legacy examples on the current why-oriented comment contract.
func TestEnsureMainCommentsReplacesLegacyComment(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "legacy")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("create legacy example directory: %v", err)
	}
	path := filepath.Join(dir, "main.go")
	source := "package main\n\n" + legacyGeneratedMainComment + "\nfunc main(){ }\n"
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write legacy example: %v", err)
	}

	if err := ensureMainComments(base); err != nil {
		t.Fatalf("normalize legacy example: %v", err)
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read normalized example: %v", err)
	}
	if bytes.Contains(generated, []byte(legacyGeneratedMainComment)) {
		t.Error("legacy generated main comment remains")
	}
	if count := bytes.Count(generated, []byte(generatedMainComment)); count != 1 {
		t.Errorf("generated main comment count = %d, want 1", count)
	}
	formatted, err := format.Source(generated)
	if err != nil {
		t.Fatalf("format normalized example independently: %v", err)
	}
	if !bytes.Equal(generated, formatted) {
		t.Errorf("normalized example is not gofmt-clean:\n%s", generated)
	}
}
