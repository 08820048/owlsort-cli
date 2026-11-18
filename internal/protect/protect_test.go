package protect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/plan"
)

// touch makes a file (or a folder when isDir) and returns its path.
func touch(t *testing.T, path string, isDir bool) string {
	t.Helper()
	if isDir {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCodeProjectsAreProtected(t *testing.T) {
	root := t.TempDir()
	guard := New(config.Config{})

	cases := []struct {
		name   string
		marker string
		isDir  bool
	}{
		{"git", ".git", true},
		{"gomod", "go.mod", false},
		{"packagejson", "package.json", false},
		{"swift", "Package.swift", false},
		{"solution", "Thing.sln", false},
		{"xcode", "Thing.xcodeproj", true},
	}
	for _, c := range cases {
		project := touch(t, filepath.Join(root, c.name), true)
		touch(t, filepath.Join(project, c.marker), c.isDir)
		file := touch(t, filepath.Join(project, "download.zip"), false)
		if got := guard.Protected(file); got != plan.SkipInCodeProject {
			t.Errorf("%s: got %q, want %q", c.name, got, plan.SkipInCodeProject)
		}
	}
}

func TestNotesLibrariesAreProtected(t *testing.T) {
	root := t.TempDir()
	guard := New(config.Config{})

	obsidian := touch(t, filepath.Join(root, "vault"), true)
	touch(t, filepath.Join(obsidian, ".obsidian"), true)
	if got := guard.Protected(touch(t, filepath.Join(obsidian, "note.pdf"), false)); got != plan.SkipInNotesLibrary {
		t.Errorf("obsidian: got %q, want %q", got, plan.SkipInNotesLibrary)
	}

	logseq := touch(t, filepath.Join(root, "logseq-vault"), true)
	touch(t, filepath.Join(logseq, "logseq", "config.edn"), false)
	if got := guard.Protected(touch(t, filepath.Join(logseq, "note.pdf"), false)); got != plan.SkipInNotesLibrary {
		t.Errorf("logseq: got %q, want %q", got, plan.SkipInNotesLibrary)
	}
}

func TestTheKeepListProtectsAFileAndEverythingBelowAFolder(t *testing.T) {
	root := t.TempDir()
	keptFile := touch(t, filepath.Join(root, "taxes.pdf"), false)
	keptDir := touch(t, filepath.Join(root, "Taxes"), true)
	guard := New(config.Config{Keep: []string{keptFile, keptDir}})

	if got := guard.Protected(keptFile); got != plan.SkipKept {
		t.Errorf("a kept file: got %q, want %q", got, plan.SkipKept)
	}
	if got := guard.Protected(touch(t, filepath.Join(keptDir, "2025", "return.pdf"), false)); got != plan.SkipKept {
		t.Errorf("a file below a kept folder: got %q, want %q", got, plan.SkipKept)
	}
	if got := guard.Protected(touch(t, filepath.Join(root, "other.pdf"), false)); got != "" {
		t.Errorf("an ordinary file: got %q, want nothing", got)
	}
}

func TestAKeepEntryDoesNotProtectASiblingWithTheSamePrefix(t *testing.T) {
	root := t.TempDir()
	kept := touch(t, filepath.Join(root, "Taxes"), true)
	sibling := touch(t, filepath.Join(root, "Taxes-2024"), true)
	guard := New(config.Config{Keep: []string{kept}})

	if got := guard.Protected(touch(t, filepath.Join(sibling, "return.pdf"), false)); got != "" {
		t.Errorf("got %q: Taxes-2024 is not below Taxes", got)
	}
}

func TestAnOrdinaryFileIsNotProtected(t *testing.T) {
	root := t.TempDir()
	guard := New(config.Config{})
	if got := guard.Protected(touch(t, filepath.Join(root, "photo.png"), false)); got != "" {
		t.Errorf("got %q, want nothing", got)
	}
}

func TestTheCacheDoesNotChangeTheAnswer(t *testing.T) {
	root := t.TempDir()
	project := touch(t, filepath.Join(root, "app"), true)
	touch(t, filepath.Join(project, ".git"), true)
	guard := New(config.Config{})

	first := touch(t, filepath.Join(project, "one.zip"), false)
	second := touch(t, filepath.Join(project, "two.zip"), false)
	for i := 0; i < 3; i++ {
		if got := guard.Protected(first); got != plan.SkipInCodeProject {
			t.Errorf("pass %d: got %q", i, got)
		}
		if got := guard.Protected(second); got != plan.SkipInCodeProject {
			t.Errorf("pass %d, second file: got %q", i, got)
		}
	}
}
