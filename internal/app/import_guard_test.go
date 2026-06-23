package app

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCoreNeverImportsEE enforces the open-core boundary: no package under
// internal/* (the Apache-2.0 core) may import the closed ee/ tree. The dependency
// arrow is one-way (ee/ -> internal/). This guards the licensing + license-posture
// invariant at compile/test time.
func TestCoreNeverImportsEE(t *testing.T) {
	root := repoRoot(t)
	internalDir := filepath.Join(root, "internal")
	const eePrefix = "github.com/buktio/buktio/ee"

	fset := token.NewFileSet()
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if p == eePrefix || strings.HasPrefix(p, eePrefix+"/") {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s imports %q — the OSS core (internal/*) must never import ee/", rel, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// repoRoot walks up from the test's working directory to the directory containing go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from test dir")
		}
		dir = parent
	}
}
