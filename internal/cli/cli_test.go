package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/exitcode"
	"github.com/08820048/owlsort-cli/internal/i18n"
	"github.com/08820048/owlsort-cli/internal/plan"
)

func run(args ...string) (string, error) {
	root := New(BuildInfo{Version: "test", Commit: "abc", Date: "today"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestEveryCommandIsListed(t *testing.T) {
	out, err := run("--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"init", "preview", "tidy", "undo", "history", "explain", "uncovered", "rules", "folders", "keep", "watch", "service", "doctor"} {
		if !strings.Contains(out, name) {
			t.Errorf("--help does not list %q", name)
		}
	}
}

func TestVersion(t *testing.T) {
	out, err := run("--version")
	if err != nil || !strings.Contains(out, "test") {
		t.Errorf("%q, %v", out, err)
	}
}

func TestUnbuiltCommandsSayWhenTheyComeAndExitWithAnError(t *testing.T) {
	// tidy lands in M1; it must say so rather than pretend to have run.
	_, err := run("tidy")
	var coded *ExitError
	if !errors.As(err, &coded) {
		t.Fatalf("got %v", err)
	}
	if coded.Code != exitcode.Error || !strings.Contains(coded.Message, "M1") {
		t.Errorf("%+v", coded)
	}
}

func TestBadUsageIsAnError(t *testing.T) {
	if _, err := run("explain"); err == nil {
		t.Error("explain without a file was accepted")
	}
}

// writeConfig writes a configuration that watches one temporary folder.
func writeConfig(t *testing.T, folder string) string {
	t.Helper()
	c := config.Default(i18n.English)
	c.Folders = []string{folder}
	data, err := config.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// makeFile writes a file that is old enough not to look like a download.
func makeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	sort.Strings(names)
	return names
}

func TestInitWritesTheConfigAndNeverOverwritesIt(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(t.TempDir(), "nested", "config.toml")

	out, err := run("init", "--config", path, "--folders", folder, "--lang", "en")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, path) {
		t.Errorf("init said %q, want the path it wrote", out)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("what init wrote is not a valid config: %v", err)
	}
	if len(cfg.Folders) != 1 || cfg.Folders[0] != folder {
		t.Errorf("folders = %v, want [%s]", cfg.Folders, folder)
	}

	_, err = run("init", "--config", path)
	var coded *ExitError
	if !errors.As(err, &coded) || coded.Code != exitcode.Error {
		t.Fatalf("a second init: got %v, want a refusal", err)
	}
	if !strings.Contains(coded.Message, "already exists") {
		t.Errorf("message = %q", coded.Message)
	}
}

func TestPreviewShowsThePlanAndMovesNothing(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)
	makeFile(t, filepath.Join(folder, "Screenshot 2026-11-18 at 09.14.png"), 1024)
	makeFile(t, filepath.Join(folder, "photos.zip"), 1024)
	before := listDir(t, folder)

	out, err := run("preview", "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"report-final.pdf", "Documents/PDF", "Pictures/Screenshots", "Archives", "owlsort tidy"} {
		if !strings.Contains(out, want) {
			t.Errorf("preview does not mention %q:\n%s", want, out)
		}
	}
	after := listDir(t, folder)
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Errorf("preview changed the folder: before %v, after %v", before, after)
	}
}

func TestPreviewJSONHasTheDocumentedShape(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)

	out, err := run("preview", "--json", "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if raw["version"] != float64(plan.Version) {
		t.Errorf("version = %v, want %d", raw["version"], plan.Version)
	}

	var p plan.Plan
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Folders) != 1 {
		t.Fatalf("folders = %+v", p.Folders)
	}
	f := p.Folders[0]
	if f.Path != folder {
		t.Errorf("folder = %q, want %q", f.Path, folder)
	}
	if len(f.Items) != 1 {
		t.Fatalf("items = %+v", f.Items)
	}
	item := f.Items[0]
	if item.Action != plan.Move || item.Rule != "PDFs" {
		t.Errorf("item = %+v", item)
	}
	if !strings.HasSuffix(item.Destination, filepath.Join("Documents", "PDF", "report-final.pdf")) {
		t.Errorf("destination = %q", item.Destination)
	}
}

func TestPreviewWithoutAConfigSaysToRunInit(t *testing.T) {
	_, err := run("preview", "--config", filepath.Join(t.TempDir(), "missing.toml"))
	var coded *ExitError
	if !errors.As(err, &coded) {
		t.Fatalf("got %v", err)
	}
	if coded.Code != exitcode.Usage {
		t.Errorf("exit code = %d, want %d", coded.Code, exitcode.Usage)
	}
	if !strings.Contains(coded.Message, "owlsort init") {
		t.Errorf("message = %q", coded.Message)
	}
}

func TestAnInvalidConfigStopsEverything(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("conflict = \"overwrite\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := run("preview", "--config", path)
	var coded *ExitError
	if !errors.As(err, &coded) || coded.Code != exitcode.Usage {
		t.Fatalf("got %v, want a usage error (safety rule 12)", err)
	}
}

func TestExplainNamesTheRuleThatTakesTheFile(t *testing.T) {
	folder := t.TempDir()
	file := filepath.Join(folder, "report-final.pdf")
	makeFile(t, file, 1024)

	out, err := run("explain", file, "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "moves to") || !strings.Contains(out, `"PDFs"`) {
		t.Errorf("explain said %q", out)
	}
}

func TestExplainSaysWhyAFileStays(t *testing.T) {
	folder := t.TempDir()
	file := filepath.Join(folder, "thing.weird")
	makeFile(t, file, 1024)

	out, err := run("explain", file, "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no rule covers it") {
		t.Errorf("explain said %q", out)
	}
}
