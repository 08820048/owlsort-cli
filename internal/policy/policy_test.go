// Package policy holds tests that guard the structural rules in
// docs/safety-model.md: properties of the whole source tree rather than of one
// function. It has no code of its own.
package policy

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const root = "../.." // the module root, from internal/policy

// sources parses every non-test Go file of the module.
func sources(t *testing.T) map[string]*ast.File {
	t.Helper()
	files := map[string]*ast.File{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".git" || name == "dist" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = f
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 10 {
		t.Fatalf("found only %d source files: is the root path right?", len(files))
	}
	return files
}

// Rule 16: nothing in the program opens a connection.
func TestNoNetwork(t *testing.T) {
	forbidden := []string{"net", "net/http", "net/rpc", "net/smtp", "net/textproto", "crypto/tls"}
	for name, f := range sources(t) {
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range forbidden {
				if path == bad || strings.HasPrefix(path, bad+"/") && bad != "net" {
					t.Errorf("%s imports %q: the program must not use the network (docs/safety-model.md, rule 16)", name, path)
				}
			}
		}
	}
}

// Rule 4: nothing deletes a user's file except the code that does it on
// purpose, and that code is small and reviewed: `apply` (after a verified
// cross-volume copy, and to clean up its own half-written copy), `trash`, and
// `atomicfile` — whose only removal is the temporary file it just created beside
// the one it is replacing.
func TestOnlyApplyAndTrashRemoveFiles(t *testing.T) {
	allowed := func(name string) bool {
		return strings.HasPrefix(name, "internal/apply/") ||
			strings.HasPrefix(name, "internal/trash/") ||
			strings.HasPrefix(name, "internal/atomicfile/")
	}
	for name, f := range sources(t) {
		if allowed(name) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" && (sel.Sel.Name == "Remove" || sel.Sel.Name == "RemoveAll") {
				t.Errorf("%s calls os.%s: only internal/apply, internal/trash and internal/atomicfile may remove files (docs/safety-model.md, rule 4)", name, sel.Sel.Name)
			}
			return true
		})
	}
}
