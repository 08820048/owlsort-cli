package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteReplacesTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("content = %q", got)
	}
	if runtime.GOOS == "windows" {
		// Windows has no file permission bits worth asserting on.
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %o", info.Mode().Perm())
	}
}

func TestWriteCreatesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.toml")
	if err := Write(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error(err)
	}
}

func TestNoTemporaryFileIsLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := Write(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".owlsort-") {
			t.Errorf("a temporary file was left behind: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("the folder holds %d files, want only the one written", len(entries))
	}
}

func TestWriteFailsWhenTheFolderIsNotThere(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "config.toml")
	if err := Write(path, []byte("x"), 0o600); err == nil {
		t.Error("writing into a folder that is not there was allowed")
	}
}
