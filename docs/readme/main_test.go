package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseFuncsInDirExcludesMethodsOnPrivateReceivers protects the public API boundary from commented implementation methods.
func TestParseFuncsInDirExcludesMethodsOnPrivateReceivers(t *testing.T) {
	dir := t.TempDir()
	source := `package fixture

// Visible is an exported package function.
func Visible() {}

// Exported is part of the package API.
type Exported struct{}

// PublicMethod is an exported method on an exported receiver.
func (Exported) PublicMethod() {}

type private struct{}

// LeakedMethod must remain internal even though the method itself is exported and documented.
func (*private) LeakedMethod() {}
`
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	funcs, err := parseFuncsInDir(dir)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	found := make(map[string]bool, len(funcs))
	for _, fn := range funcs {
		found[fn.Key] = true
	}
	for _, key := range []string{"Visible", "Exported.PublicMethod"} {
		if !found[key] {
			t.Errorf("public API entry %q was omitted", key)
		}
	}
	if found["private.LeakedMethod"] {
		t.Error("method on private receiver leaked into the public API")
	}
	if len(funcs) != 2 {
		t.Fatalf("parsed %d entries, want exactly the two public declarations", len(funcs))
	}
}
