package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/exitcode"
	"github.com/08820048/owlsort-cli/internal/i18n"
	"github.com/08820048/owlsort-cli/internal/plan"
	"github.com/08820048/owlsort-cli/internal/protect"
	"github.com/08820048/owlsort-cli/internal/rules"
	"github.com/08820048/owlsort-cli/internal/scan"
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
	// watch lands in M3; it must say so rather than pretend to have run.
	_, err := run("watch")
	var coded *ExitError
	if !errors.As(err, &coded) {
		t.Fatalf("got %v", err)
	}
	if coded.Code != exitcode.Error || !strings.Contains(coded.Message, "M3") {
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

// withStdin runs the command with a canned answer, and pretends to be (or not be)
// a terminal.
func withStdin(t *testing.T, terminal bool, answer string, args ...string) (string, error) {
	t.Helper()
	previous := stdinIsTerminal
	stdinIsTerminal = func() bool { return terminal }
	t.Cleanup(func() { stdinIsTerminal = previous })

	root := New(BuildInfo{Version: "test", Commit: "abc", Date: "today"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(answer))
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// stateDir keeps the journal of a test out of the real one. Both variables are
// set because the journal follows XDG_STATE_HOME on macOS and Linux and
// LOCALAPPDATA on Windows; both give the same <dir>/owlsort/journal.jsonl.
func stateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
	return dir
}

// treeOf is every path below dir with its content.
func treeOf(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			out[rel] = "<dir>"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameTree(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("the folder changed: had %v, has %v", keysOf(before), keysOf(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("%s: was %q, is %q", path, content, after[path])
		}
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// exitCodeOf is the code an error maps to, or 0.
func exitCodeOf(err error) int {
	var coded *ExitError
	if errors.As(err, &coded) {
		return coded.Code
	}
	if err != nil {
		return exitcode.Error
	}
	return exitcode.OK
}

func TestTidyWithoutATerminalOnlyPreviews(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)
	before := treeOf(t, folder)
	state := stateDir(t)

	out, err := withStdin(t, false, "", "tidy", "--config", writeConfig(t, folder))
	if code := exitCodeOf(err); code != exitcode.NotConfirmed {
		t.Fatalf("exit code = %d, want %d (%v)", code, exitcode.NotConfirmed, err)
	}
	if !strings.Contains(out, "report-final.pdf") {
		t.Errorf("a tidy with no terminal must still show the plan:\n%s", out)
	}
	if !strings.Contains(out, "--yes") {
		t.Errorf("it should say how to go ahead anyway:\n%s", out)
	}
	sameTree(t, before, treeOf(t, folder))
	if _, err := os.Stat(filepath.Join(state, "owlsort", "journal.jsonl")); !os.IsNotExist(err) {
		t.Error("a run that moved nothing wrote a journal")
	}
}

func TestTidyWithANoLeavesTheFolderUntouched(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)
	before := treeOf(t, folder)
	stateDir(t)

	out, err := withStdin(t, true, "n\n", "tidy", "--config", writeConfig(t, folder))
	if code := exitCodeOf(err); code != exitcode.NotConfirmed {
		t.Fatalf("exit code = %d, want %d (%v)", code, exitcode.NotConfirmed, err)
	}
	if !strings.Contains(out, "Nothing moved") {
		t.Errorf("said %q", out)
	}
	sameTree(t, before, treeOf(t, folder))
}

func TestTidyWithAYesMovesTheFiles(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)
	makeFile(t, filepath.Join(folder, "photos.zip"), 1024)
	state := stateDir(t)

	out, err := withStdin(t, true, "y\n", "tidy", "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(folder, "Documents", "PDF", "report-final.pdf")); err != nil {
		t.Errorf("the PDF did not move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(folder, "Archives", "photos.zip")); err != nil {
		t.Errorf("the archive did not move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "owlsort", "journal.jsonl")); err != nil {
		t.Errorf("no journal was written: %v", err)
	}
	if !strings.Contains(out, "owlsort undo") {
		t.Errorf("it should say how to undo:\n%s", out)
	}
}

func TestTidyWithYesDoesNotAsk(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)
	stateDir(t)

	out, err := withStdin(t, true, "", "tidy", "--yes", "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "[y/N]") {
		t.Errorf("--yes still asked:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(folder, "Documents", "PDF", "report-final.pdf")); err != nil {
		t.Error("the file did not move")
	}
}

func TestTidyThenUndoRestoresTheFolder(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)
	makeFile(t, filepath.Join(folder, "photos.zip"), 2048)
	makeFile(t, filepath.Join(folder, "Screenshot 1.png"), 512)
	before := treeOf(t, folder)
	stateDir(t)
	cfg := writeConfig(t, folder)

	if _, err := withStdin(t, true, "y\n", "tidy", "--config", cfg); err != nil {
		t.Fatal(err)
	}
	if len(treeOf(t, folder)) == len(before) {
		t.Fatal("nothing moved")
	}

	out, err := withStdin(t, true, "", "undo", "--config", cfg)
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	sameTree(t, before, treeOf(t, folder))
}

func TestUndoWithNothingToUndoExitsWithFive(t *testing.T) {
	stateDir(t)
	folder := t.TempDir()
	out, err := withStdin(t, true, "", "undo", "--config", writeConfig(t, folder))
	if code := exitCodeOf(err); code != exitcode.NothingToUndo {
		t.Fatalf("exit code = %d, want %d (%v)\n%s", code, exitcode.NothingToUndo, err, out)
	}
}

func TestUndoWithAnUnknownBatchExitsWithFive(t *testing.T) {
	stateDir(t)
	folder := t.TempDir()
	_, err := withStdin(t, true, "", "undo", "--batch", "20260101-000000-0000", "--config", writeConfig(t, folder))
	if code := exitCodeOf(err); code != exitcode.NothingToUndo {
		t.Fatalf("exit code = %d, want %d (%v)", code, exitcode.NothingToUndo, err)
	}
}

func TestHistoryListsARunAndSaysItWasUndone(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)
	stateDir(t)
	cfg := writeConfig(t, folder)

	if _, err := withStdin(t, true, "y\n", "tidy", "--config", cfg); err != nil {
		t.Fatal(err)
	}
	out, err := withStdin(t, true, "", "history", "--config", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "BATCH") || !strings.Contains(out, "Documents/PDF") && !strings.Contains(out, "Documents") {
		t.Errorf("history said:\n%s", out)
	}
	if strings.Contains(out, "undone") {
		t.Error("nothing has been undone yet")
	}

	if _, err := withStdin(t, true, "", "undo", "--config", cfg); err != nil {
		t.Fatal(err)
	}
	out, err = withStdin(t, true, "", "history", "--config", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "undone") {
		t.Errorf("history does not mention the undo:\n%s", out)
	}
}

// writeConfigWith is writeConfig with the clash strategy changed.
func writeConfigWith(t *testing.T, folder string, conflict config.Conflict) string {
	t.Helper()
	c := config.Default(i18n.English)
	c.Folders = []string{folder}
	c.Conflict = conflict
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

func TestTidyNeverOverwritesAndExitsWithThree(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)
	taken := filepath.Join(folder, "Documents", "PDF", "report-final.pdf")
	if err := os.MkdirAll(filepath.Dir(taken), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(taken, []byte("the one that was already there"), 0o644); err != nil {
		t.Fatal(err)
	}
	stateDir(t)

	out, err := withStdin(t, true, "y\n", "tidy", "--config", writeConfigWith(t, folder, config.ConflictSkip))
	if code := exitCodeOf(err); code != exitcode.Partial {
		t.Fatalf("exit code = %d, want %d (%v)\n%s", code, exitcode.Partial, err, out)
	}
	if got, err := os.ReadFile(taken); err != nil || string(got) != "the one that was already there" {
		t.Errorf("the existing file was overwritten: %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(folder, "report-final.pdf")); err != nil {
		t.Error("the file the strategy said to skip was moved anyway")
	}
}

// writeConfigText writes a configuration from TOML source, so a test can be
// deliberately wrong.
//
// A path spliced into the source has to go in a TOML literal string ('...'): in a
// basic string ("...") a backslash is an escape, and a Windows path would make the
// file invalid before the test ever looked at it.
func writeConfigText(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRulesCheckSaysNothingAboutAGoodConfiguration(t *testing.T) {
	folder := t.TempDir()
	out, err := withStdin(t, false, "", "rules", "check", "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if !strings.Contains(out, "no problems found") {
		t.Errorf("said %q", out)
	}
}

func TestRulesCheckFailsOnAMistake(t *testing.T) {
	path := writeConfigText(t, `conflict = "rename"
[[rule]]
name = "bad"
when = [{ ext = ["pdf"] }]
move_to = "../outside"
`)
	out, err := withStdin(t, false, "", "rules", "check", "--config", path)
	if code := exitCodeOf(err); code != exitcode.Error {
		t.Fatalf("exit code = %d, want %d (%v)\n%s", code, exitcode.Error, err, out)
	}
	if !strings.Contains(out, "climbs out") {
		t.Errorf("said %q", out)
	}
}

func TestRulesCheckWarnsAboutAShadowedRuleWithoutFailing(t *testing.T) {
	folder := t.TempDir()
	path := writeConfigText(t, `conflict = "rename"
folders = ['`+folder+`']
[[rule]]
name = "PDFs first"
when = [{ ext = ["pdf"] }]
move_to = "A"
[[rule]]
name = "Documents"
when = [{ ext = ["pdf", "docx"] }]
move_to = "B"
`)
	out, err := withStdin(t, false, "", "rules", "check", "--config", path)
	if err != nil {
		t.Fatalf("a warning must not fail: %v\n%s", err, out)
	}
	if !strings.Contains(out, "warning") || !strings.Contains(out, `"Documents"`) {
		t.Errorf("said %q", out)
	}
	if !strings.Contains(out, "does not stop the program") {
		t.Errorf("it does not say the warning is harmless: %q", out)
	}
}

func TestRulesCheckReportsAFileThatIsNotValidTOML(t *testing.T) {
	path := writeConfigText(t, "this is not toml = = =\n")
	out, err := withStdin(t, false, "", "rules", "check", "--config", path)
	if code := exitCodeOf(err); code != exitcode.Error {
		t.Fatalf("exit code = %d, want %d\n%s", code, exitcode.Error, out)
	}
	if !strings.Contains(out, "error") {
		t.Errorf("said %q", out)
	}
}

func TestRulesCheckJSON(t *testing.T) {
	folder := t.TempDir()
	path := writeConfigText(t, `conflict = "rename"
folders = ['`+folder+`']
[[rule]]
name = "a"
when = [{ ext = ["pdf"] }]
move_to = "A"
[[rule]]
name = "b"
when = [{ ext = ["pdf"] }]
move_to = "B"
`)
	out, err := withStdin(t, false, "", "rules", "check", "--json", "--config", path)
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	var problems []struct {
		Severity string `json:"severity"`
		Rule     string `json:"rule"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal([]byte(out), &problems); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if len(problems) != 1 || problems[0].Severity != "warning" || problems[0].Rule != "b" {
		t.Errorf("got %+v", problems)
	}
}

func TestUncoveredListsWhatNoRuleTakes(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)
	makeFile(t, filepath.Join(folder, "one.weird"), 10)
	makeFile(t, filepath.Join(folder, "two.weird"), 10)

	out, err := withStdin(t, false, "", "uncovered", "--config", writeConfig(t, folder))
	if code := exitCodeOf(err); code != exitcode.Partial {
		t.Fatalf("exit code = %d, want %d (%v)\n%s", code, exitcode.Partial, err, out)
	}
	if !strings.Contains(out, "weird") || strings.Contains(out, "report-final.pdf") {
		t.Errorf("said %q", out)
	}
}

func TestUncoveredIsQuietWhenEverythingIsCovered(t *testing.T) {
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)
	stateDir(t)

	out, err := withStdin(t, false, "", "uncovered", "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if !strings.Contains(out, "Every loose file is covered") {
		t.Errorf("said %q", out)
	}
}

func TestDoctorOnAHealthySetup(t *testing.T) {
	stateDir(t)
	folder := t.TempDir()
	makeFile(t, filepath.Join(folder, "report-final.pdf"), 1024)

	out, err := withStdin(t, false, "", "doctor", "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	for _, want := range []string{"configuration", "journal", "watched folder", "Ready"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor does not mention %q:\n%s", want, out)
		}
	}
}

func TestDoctorFailsWhenTheConfigurationIsMissing(t *testing.T) {
	stateDir(t)
	missing := filepath.Join(t.TempDir(), "nope.toml")
	out, err := withStdin(t, false, "", "doctor", "--config", missing)
	if code := exitCodeOf(err); code != exitcode.Error {
		t.Fatalf("exit code = %d, want %d\n%s", code, exitcode.Error, out)
	}
	if !strings.Contains(out, "FAIL") {
		t.Errorf("said %q", out)
	}
}

// configText is a small, commented configuration a test can edit.
func configText(folder string) string {
	return `# a configuration with a comment worth keeping
folders = ['` + folder + `']
keep = []
conflict = "rename"   # keep both files

[[rule]]
name = "PDFs"
when = [{ ext = ["pdf"] }]
move_to = "Documents/PDF"   # where PDFs go

[[rule]]
name = "Archives"
when = [{ category = "archive" }]
move_to = "Archives"
`
}

func TestRulesListShowsEveryRule(t *testing.T) {
	folder := t.TempDir()
	out, err := withStdin(t, false, "", "rules", "list", "--config", writeConfigText(t, configText(folder)))
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	for _, want := range []string{"PDFs", "Archives", "move to Documents/PDF", "on", "2 of 2 on"} {
		if !strings.Contains(out, want) {
			t.Errorf("rules list does not mention %q:\n%s", want, out)
		}
	}
}

func TestRulesShowOneRule(t *testing.T) {
	folder := t.TempDir()
	out, err := withStdin(t, false, "", "rules", "show", "PDFs", "--config", writeConfigText(t, configText(folder)))
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	for _, want := range []string{"PDFs", "ext is \"pdf\"", "move to Documents/PDF"} {
		if !strings.Contains(out, want) {
			t.Errorf("rules show does not mention %q:\n%s", want, out)
		}
	}
}

func TestRulesShowAnUnknownRuleIsAnError(t *testing.T) {
	folder := t.TempDir()
	_, err := withStdin(t, false, "", "rules", "show", "Nope", "--config", writeConfigText(t, configText(folder)))
	if code := exitCodeOf(err); code != exitcode.Error {
		t.Fatalf("exit code = %d, want %d", code, exitcode.Error)
	}
}

func TestRulesDisableEditsOnlyThatRuleAndKeepsComments(t *testing.T) {
	folder := t.TempDir()
	path := writeConfigText(t, configText(folder))

	out, err := withStdin(t, false, "", "rules", "disable", "PDFs", "--config", path)
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(after)
	if !strings.Contains(text, "# a configuration with a comment worth keeping") {
		t.Error("the leading comment was lost")
	}
	if !strings.Contains(text, `conflict = "rename"   # keep both files`) {
		t.Error("the trailing comment on conflict was lost")
	}
	if !strings.Contains(text, "move_to = \"Documents/PDF\"   # where PDFs go") {
		t.Error("the trailing comment on move_to was lost")
	}
	if !strings.Contains(text, "name = \"PDFs\"\nenabled = false\n") {
		t.Errorf("PDFs was not switched off:\n%s", text)
	}
	if strings.Contains(text, "name = \"Archives\"\nenabled") {
		t.Error("Archives was changed as well")
	}
	if !strings.Contains(out, "is off") {
		t.Errorf("said %q", out)
	}
}

func TestRulesDisableTwiceSaysNothingChanged(t *testing.T) {
	folder := t.TempDir()
	path := writeConfigText(t, configText(folder))
	if _, err := withStdin(t, false, "", "rules", "disable", "PDFs", "--config", path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	out, err := withStdin(t, false, "", "rules", "disable", "PDFs", "--config", path)
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if !strings.Contains(out, "already off") {
		t.Errorf("said %q", out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("the file was rewritten for no change")
	}
}

func TestRulesEnableTurnsAnOffRuleBackOn(t *testing.T) {
	folder := t.TempDir()
	path := writeConfigText(t, strings.Replace(configText(folder),
		"name = \"PDFs\"\n", "name = \"PDFs\"\nenabled = false\n", 1))
	if _, err := withStdin(t, false, "", "rules", "enable", "PDFs", "--config", path); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "name = \"PDFs\"\nenabled = true\n") {
		t.Errorf("PDFs was not switched on:\n%s", after)
	}
}

func TestFoldersAddAndRemove(t *testing.T) {
	watched := t.TempDir()
	extra := t.TempDir()
	path := writeConfigText(t, "folders = ['"+watched+"']\nconflict = \"rename\"\n")

	out, err := withStdin(t, false, "", "folders", "add", extra, "--config", path)
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Folders) != 2 || cfg.Folders[1] != extra {
		t.Fatalf("folders = %v", cfg.Folders)
	}

	out, err = withStdin(t, false, "", "folders", "list", "--config", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, extra) || !strings.Contains(out, watched) {
		t.Errorf("folders list said:\n%s", out)
	}

	if _, err := withStdin(t, false, "", "folders", "remove", extra, "--config", path); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Folders) != 1 || cfg.Folders[0] != watched {
		t.Errorf("folders = %v", cfg.Folders)
	}
}

func TestFoldersAddRefusesAFolderThatIsNotThere(t *testing.T) {
	watched := t.TempDir()
	path := writeConfigText(t, "folders = ['"+watched+"']\nconflict = \"rename\"\n")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = withStdin(t, false, "", "folders", "add", filepath.Join(t.TempDir(), "gone"), "--config", path)
	if code := exitCodeOf(err); code != exitcode.Usage {
		t.Fatalf("exit code = %d, want %d (%v)", code, exitcode.Usage, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("a folder that is not there was added anyway")
	}
}

func TestFoldersAddRefusesAFile(t *testing.T) {
	watched := t.TempDir()
	file := filepath.Join(watched, "a-file.pdf")
	makeFile(t, file, 10)
	path := writeConfigText(t, "folders = ['"+watched+"']\nconflict = \"rename\"\n")

	_, err := withStdin(t, false, "", "folders", "add", file, "--config", path)
	if code := exitCodeOf(err); code != exitcode.Usage {
		t.Fatalf("exit code = %d, want %d", code, exitcode.Usage)
	}
}

func TestFoldersAddIsIdempotent(t *testing.T) {
	watched := t.TempDir()
	path := writeConfigText(t, "folders = ['"+watched+"']\nconflict = \"rename\"\n")

	out, err := withStdin(t, false, "", "folders", "add", watched, "--config", path)
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if !strings.Contains(out, "already in folders") {
		t.Errorf("said %q", out)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Folders) != 1 {
		t.Errorf("folders = %v", cfg.Folders)
	}
}

func TestFoldersRemoveSomethingThatIsNotThereIsAnError(t *testing.T) {
	watched := t.TempDir()
	path := writeConfigText(t, "folders = ['"+watched+"']\nconflict = \"rename\"\n")
	_, err := withStdin(t, false, "", "folders", "remove", t.TempDir(), "--config", path)
	if code := exitCodeOf(err); code != exitcode.Error {
		t.Fatalf("exit code = %d, want %d", code, exitcode.Error)
	}
}

func TestKeepAddAndList(t *testing.T) {
	folder := t.TempDir()
	kept := t.TempDir()
	path := writeConfigText(t, "folders = ['"+folder+"']\nkeep = []\nconflict = \"rename\"\n")

	if _, err := withStdin(t, false, "", "keep", "add", kept, "--config", path); err != nil {
		t.Fatal(err)
	}
	out, err := withStdin(t, false, "", "keep", "list", "--config", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, kept) {
		t.Errorf("keep list said:\n%s", out)
	}

	// A kept folder really is left alone.
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	makeFile(t, filepath.Join(kept, "report.pdf"), 10)
	p, err := rules.Build([]scan.File{{
		Path: filepath.Join(kept, "report.pdf"), Name: "report.pdf", Extension: "pdf",
		Folder: kept, Modified: time.Now().Add(-time.Hour), Size: 10,
	}}, cfg, protect.New(cfg), rules.Env{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if p.Count() != 0 {
		t.Errorf("a kept file was planned: %+v", p.Folders)
	}
	if len(p.Folders) != 1 || len(p.Folders[0].Skipped) != 1 || p.Folders[0].Skipped[0].Reason != plan.SkipKept {
		t.Errorf("a kept file was not reported as kept: %+v", p.Folders)
	}
}

func TestKeepRemoveAllowsTidyingAgain(t *testing.T) {
	folder := t.TempDir()
	kept := filepath.Join(folder, "Taxes")
	if err := os.MkdirAll(kept, 0o755); err != nil {
		t.Fatal(err)
	}
	path := writeConfigText(t, "folders = ['"+folder+"']\nkeep = ['"+kept+"']\nconflict = \"rename\"\n")

	if _, err := withStdin(t, false, "", "keep", "remove", kept, "--config", path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Keep) != 0 {
		t.Errorf("keep = %v", cfg.Keep)
	}
}

// fakeEditor writes a stand-in editor that runs body with the file as its first
// argument. It is a shell script on Unix and a batch file on Windows: a test that
// only wrote .sh would be testing nothing at all on one of the three platforms.
func fakeEditor(t *testing.T, unixBody, windowsBody string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(t.TempDir(), "editor.bat")
		if err := os.WriteFile(path, []byte(windowsBody), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+unixBody), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRulesEditOpensAnEditorAndChecksTheResult(t *testing.T) {
	folder := t.TempDir()
	path := writeConfigText(t, configText(folder))

	// A stand-in editor that only adds a comment, so the file stays valid.
	editor := fakeEditor(t,
		"printf '\\n# edited by the test\\n' >> \"$1\"\n",
		"@echo off\r\n>>\"%1\" echo.\r\n>>\"%1\" echo # edited by the test\r\n",
	)
	t.Setenv("VISUAL", editor)

	out, err := withStdin(t, true, "", "rules", "edit", "--config", path)
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if !strings.Contains(out, "no problems found") {
		t.Errorf("said %q", out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "# edited by the test") {
		t.Error("the edit did not land")
	}
}

func TestRulesEditReportsWhatTheEditorBroke(t *testing.T) {
	folder := t.TempDir()
	path := writeConfigText(t, configText(folder))

	// Writes a configuration with an invalid value, which the next load refuses.
	editor := fakeEditor(t,
		"printf 'conflict = \"overwrite\"\\n' > \"$1\"\n",
		"@echo off\r\n>\"%1\" echo conflict = \"overwrite\"\r\n",
	)
	t.Setenv("VISUAL", editor)

	out, err := withStdin(t, true, "", "rules", "edit", "--config", path)
	if code := exitCodeOf(err); code != exitcode.Usage {
		t.Fatalf("exit code = %d, want %d (%v)\n%s", code, exitcode.Usage, err, out)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(after), "overwrite") {
		t.Error("the file was thrown away: a half-finished edit is still the user's")
	}
}

func TestRulesEditWithoutAnEditorOrATerminalRefuses(t *testing.T) {
	folder := t.TempDir()
	path := writeConfigText(t, configText(folder))
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	_, err := withStdin(t, false, "", "rules", "edit", "--config", path)
	if code := exitCodeOf(err); code != exitcode.Usage {
		t.Fatalf("exit code = %d, want %d", code, exitcode.Usage)
	}
	if !strings.Contains(err.Error(), "EDITOR") {
		t.Errorf("message = %q", err.Error())
	}
}

func TestExplainJSONHasTheDocumentedShape(t *testing.T) {
	folder := t.TempDir()
	file := filepath.Join(folder, "report-final.pdf")
	makeFile(t, file, 1024)

	out, err := withStdin(t, false, "", "explain", "--json", file, "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	var got struct {
		Path        string `json:"path"`
		Planned     bool   `json:"planned"`
		Rule        string `json:"rule"`
		Action      string `json:"action"`
		Destination string `json:"destination"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if !got.Planned || got.Rule != "PDFs" || got.Action != "move" {
		t.Errorf("got %+v", got)
	}
	if !strings.HasSuffix(got.Destination, filepath.Join("Documents", "PDF", "report-final.pdf")) {
		t.Errorf("destination = %q", got.Destination)
	}
}

func TestExplainJSONForAFileLeftAlone(t *testing.T) {
	folder := t.TempDir()
	file := filepath.Join(folder, "thing.weird")
	makeFile(t, file, 10)

	out, err := withStdin(t, false, "", "explain", "--json", file, "--config", writeConfig(t, folder))
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	var got struct {
		Planned bool `json:"planned"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Planned {
		t.Errorf("a file no rule covers was reported as planned:\n%s", out)
	}
}
