package apply

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/journal"
	"github.com/08820048/owlsort-cli/internal/plan"
)

// fixture is a watched folder, a journal and a clock.
type fixture struct {
	dir string
	j   *journal.File
	now time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	j, err := journal.Open(filepath.Join(t.TempDir(), "journal.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{dir: dir, j: j, now: time.Date(2026, 11, 18, 14, 25, 3, 0, time.UTC)}
}

// write makes a file and returns its path.
func (f *fixture) write(t *testing.T, name string, size int) string {
	t.Helper()
	path := filepath.Join(f.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// item describes a move of an existing file.
func (f *fixture) item(t *testing.T, src, dst string) plan.Item {
	t.Helper()
	info, err := os.Lstat(src)
	if err != nil {
		t.Fatal(err)
	}
	return plan.Item{
		Source: src, Destination: dst, Action: plan.Move, Rule: "test",
		Size: info.Size(), Modified: info.ModTime(),
	}
}

func (f *fixture) plan(items ...plan.Item) plan.Plan {
	return plan.Plan{Version: plan.Version, Folders: []plan.Folder{{Path: f.dir, Items: items}}}
}

func (f *fixture) run(t *testing.T, p plan.Plan, conflict config.Conflict) Result {
	t.Helper()
	res, err := Run(p, f.j, Options{Conflict: conflict, Now: f.now})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// tree is every path below dir with its content, for "the folder is unchanged".
func tree(t *testing.T, dir string) map[string]string {
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
		t.Fatalf("the folder changed: had %d entries, has %d", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("%s: was %q, is %q", path, content, after[path])
		}
	}
}

func TestAMovePutsTheFileWhereThePlanSaid(t *testing.T) {
	f := newFixture(t)
	src := f.write(t, "report.pdf", 10)
	dst := filepath.Join(f.dir, "Documents", "PDF", "report.pdf")

	res := f.run(t, f.plan(f.item(t, src, dst)), config.ConflictRename)
	if res.Done != 1 || len(res.Skipped) != 0 || res.Failed != 0 {
		t.Fatalf("got %+v", res)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("the file is not at %s: %v", dst, err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("the source is still there")
	}

	entries, err := f.j.Entries(res.Batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Result != journal.OK || entries[0].Dest != dst {
		t.Errorf("journal = %+v", entries)
	}
}

func TestALeapFrogIsRenamedAndBothFilesSurvive(t *testing.T) {
	f := newFixture(t)
	src := f.write(t, "report.pdf", 10)
	dst := filepath.Join(f.dir, "Documents", "report.pdf")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("the one that was already there"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := f.run(t, f.plan(f.item(t, src, dst)), config.ConflictRename)
	if res.Done != 1 {
		t.Fatalf("got %+v", res)
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != "the one that was already there" {
		t.Errorf("the existing file was overwritten: %q %v", got, err)
	}
	moved := filepath.Join(f.dir, "Documents", "report 2.pdf")
	if got, err := os.ReadFile(moved); err != nil || string(got) != strings.Repeat("x", 10) {
		t.Errorf("the moved file is not at %s: %q %v", moved, got, err)
	}
}

func TestSkipLeavesTheFileWhereItIs(t *testing.T) {
	f := newFixture(t)
	src := f.write(t, "report.pdf", 10)
	dst := filepath.Join(f.dir, "Documents", "report.pdf")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := tree(t, f.dir)

	res := f.run(t, f.plan(f.item(t, src, dst)), config.ConflictSkip)
	if res.Done != 0 || len(res.Skipped) != 1 || res.Skipped[0].Reason != plan.SkipConflict {
		t.Fatalf("got %+v", res)
	}
	sameTree(t, before, tree(t, f.dir))
}

func TestAFileThatChangedSinceThePreviewIsLeftAlone(t *testing.T) {
	f := newFixture(t)
	src := f.write(t, "report.pdf", 10)
	dst := filepath.Join(f.dir, "Documents", "report.pdf")
	item := f.item(t, src, dst)

	// The user edits the file between the preview and the run.
	if err := os.WriteFile(src, []byte("a different, longer file"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := f.run(t, f.plan(item), config.ConflictRename)
	if res.Done != 0 || len(res.Skipped) != 1 || res.Skipped[0].Reason != plan.SkipChanged {
		t.Fatalf("got %+v", res)
	}
	if _, err := os.Stat(src); err != nil {
		t.Error("the changed file was moved anyway")
	}
}

func TestAFileThatDisappearedSinceThePreviewIsReported(t *testing.T) {
	f := newFixture(t)
	src := f.write(t, "report.pdf", 10)
	item := f.item(t, src, filepath.Join(f.dir, "Documents", "report.pdf"))
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}

	res := f.run(t, f.plan(item), config.ConflictRename)
	if res.Done != 0 || len(res.Skipped) != 1 || res.Skipped[0].Reason != plan.SkipChanged {
		t.Fatalf("got %+v", res)
	}
}

func TestTrashThenUndoPutsTheFileBack(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the trash test writes under $HOME/.Trash; other platforms are covered by CI")
	}
	// A throwaway home, so nothing real is touched.
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".Trash"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	f := newFixture(t)
	src := f.write(t, "junk.tmp", 10)
	info, err := os.Lstat(src)
	if err != nil {
		t.Fatal(err)
	}
	item := plan.Item{Source: src, Action: plan.Trash, Rule: "junk", Size: info.Size(), Modified: info.ModTime()}

	res := f.run(t, f.plan(item), config.ConflictRename)
	if res.Done != 1 {
		t.Fatalf("got %+v", res)
	}
	if _, err := os.Stat(filepath.Join(home, ".Trash", "junk.tmp")); err != nil {
		t.Fatalf("the file is not in the trash: %v", err)
	}

	undoRes, err := Undo(res.Batch, f.j, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if undoRes.Done != 1 {
		t.Fatalf("undo: got %+v", undoRes)
	}
	if got, err := os.ReadFile(src); err != nil || string(got) != strings.Repeat("x", 10) {
		t.Errorf("the file did not come back: %q %v", got, err)
	}
}

func TestUndoRestoresTheFolderExactly(t *testing.T) {
	f := newFixture(t)
	a := f.write(t, "report.pdf", 10)
	b := f.write(t, "photos.zip", 20)
	c := f.write(t, "Screenshot 1.png", 30)
	before := tree(t, f.dir)

	items := []plan.Item{
		f.item(t, a, filepath.Join(f.dir, "Documents", "PDF", "report.pdf")),
		f.item(t, b, filepath.Join(f.dir, "Archives", "photos.zip")),
		f.item(t, c, filepath.Join(f.dir, "Pictures", "Screenshots", "Screenshot 1.png")),
	}
	res := f.run(t, f.plan(items...), config.ConflictRename)
	if res.Done != 3 {
		t.Fatalf("got %+v", res)
	}
	if len(tree(t, f.dir)) == len(before) {
		t.Fatal("nothing moved, so the undo proves nothing")
	}

	undoRes, err := Undo(res.Batch, f.j, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if undoRes.Done != 3 {
		t.Fatalf("undo: got %+v", undoRes)
	}
	sameTree(t, before, tree(t, f.dir))
}

func TestUndoWalksTheBatchBackwards(t *testing.T) {
	f := newFixture(t)
	// Two renames of the same file in one batch: only the reverse order puts it
	// back where it started.
	src := f.write(t, "a.txt", 5)
	first := f.item(t, src, filepath.Join(f.dir, "b.txt"))
	res := f.run(t, f.plan(first), config.ConflictRename)
	if res.Done != 1 {
		t.Fatalf("got %+v", res)
	}

	entries, err := f.j.Entries(res.Batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Source != src {
		t.Fatalf("journal = %+v", entries)
	}

	undoRes, err := Undo(res.Batch, f.j, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if undoRes.Done != 1 {
		t.Fatalf("undo: got %+v", undoRes)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("the file is not back at %s", src)
	}
}

func TestUndoSkippedWhenTheOriginalPlaceIsTaken(t *testing.T) {
	f := newFixture(t)
	src := f.write(t, "report.pdf", 10)
	dst := filepath.Join(f.dir, "Documents", "report.pdf")
	res := f.run(t, f.plan(f.item(t, src, dst)), config.ConflictRename)
	if res.Done != 1 {
		t.Fatalf("got %+v", res)
	}

	// The user puts a new file where the old one was.
	if err := os.WriteFile(src, []byte("a new file the user made"), 0o644); err != nil {
		t.Fatal(err)
	}

	undoRes, err := Undo(res.Batch, f.j, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if undoRes.Done != 0 || len(undoRes.Skipped) != 1 || undoRes.Skipped[0].Reason != plan.SkipConflict {
		t.Fatalf("undo: got %+v", undoRes)
	}
	if got, err := os.ReadFile(src); err != nil || string(got) != "a new file the user made" {
		t.Errorf("undo overwrote the user's file: %q %v", got, err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Error("the moved file should still be where the run put it")
	}
}

func TestUndoSkippedWhenTheFileHasMovedOn(t *testing.T) {
	f := newFixture(t)
	src := f.write(t, "report.pdf", 10)
	dst := filepath.Join(f.dir, "Documents", "report.pdf")
	res := f.run(t, f.plan(f.item(t, src, dst)), config.ConflictRename)
	if res.Done != 1 {
		t.Fatalf("got %+v", res)
	}
	if err := os.Remove(dst); err != nil {
		t.Fatal(err)
	}

	undoRes, err := Undo(res.Batch, f.j, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if undoRes.Done != 0 || len(undoRes.Skipped) != 1 || undoRes.Skipped[0].Reason != plan.SkipChanged {
		t.Fatalf("undo: got %+v", undoRes)
	}
}

func TestUndoneStepsAreNotUndoneTwice(t *testing.T) {
	f := newFixture(t)
	src := f.write(t, "report.pdf", 10)
	res := f.run(t, f.plan(f.item(t, src, filepath.Join(f.dir, "Documents", "report.pdf"))), config.ConflictRename)
	if res.Done != 1 {
		t.Fatalf("got %+v", res)
	}
	if _, err := Undo(res.Batch, f.j, Options{}); err != nil {
		t.Fatal(err)
	}

	second, err := Undo(res.Batch, f.j, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if second.Done != 0 {
		t.Errorf("a second undo moved %d files; an undone step must not be undone again", second.Done)
	}
	if _, err := os.Stat(src); err != nil {
		t.Error("the second undo disturbed the restored file")
	}
}

func TestUndoWithNothingToUndo(t *testing.T) {
	f := newFixture(t)
	res, err := Undo("", f.j, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Batch != "" || res.Done != 0 {
		t.Errorf("got %+v", res)
	}
}

func TestCopyLeavesTheOriginalAndUndoRemovesTheCopy(t *testing.T) {
	f := newFixture(t)
	src := f.write(t, "report.pdf", 10)
	item := f.item(t, src, filepath.Join(f.dir, "Backup", "report.pdf"))
	item.Action = plan.Copy

	res := f.run(t, f.plan(item), config.ConflictRename)
	if res.Done != 1 {
		t.Fatalf("got %+v", res)
	}
	if _, err := os.Stat(src); err != nil {
		t.Error("a copy must leave the original in place")
	}
	copied := filepath.Join(f.dir, "Backup", "report.pdf")
	if got, err := os.ReadFile(copied); err != nil || string(got) != strings.Repeat("x", 10) {
		t.Errorf("the copy is wrong: %q %v", got, err)
	}

	undoRes, err := Undo(res.Batch, f.j, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if undoRes.Done != 1 {
		t.Fatalf("undo: got %+v", undoRes)
	}
	if _, err := os.Stat(copied); !os.IsNotExist(err) {
		t.Error("the copy is still there")
	}
	if _, err := os.Stat(src); err != nil {
		t.Error("undo removed the original as well")
	}
}

// Rule 11, the part that can be tested on one volume: the copy is exact, and a
// destination that already exists is refused rather than overwritten.
func TestCopyVerifyIsExactAndNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "film.mov")
	if err := os.WriteFile(src, []byte("frames"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "copy.mov")
	if err := copyVerify(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "frames" {
		t.Fatalf("copy = %q %v", got, err)
	}

	if err := copyVerify(src, dst); err == nil {
		t.Error("copying onto an existing file was allowed")
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != "frames" {
		t.Errorf("the refused copy changed the destination: %q %v", got, err)
	}
}

func TestAFailedStepIsRecordedAndTheRestGoOn(t *testing.T) {
	f := newFixture(t)
	good := f.write(t, "report.pdf", 10)
	// A destination folder that cannot be created: a file where a folder has to
	// be.
	blocked := filepath.Join(f.dir, "Blocked")
	if err := os.WriteFile(blocked, []byte("in the way"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := f.item(t, good, filepath.Join(blocked, "sub", "report.pdf"))
	other := f.write(t, "photos.zip", 10)
	items := []plan.Item{bad, f.item(t, other, filepath.Join(f.dir, "Archives", "photos.zip"))}

	res := f.run(t, f.plan(items...), config.ConflictRename)
	if res.Done != 1 || res.Failed != 1 {
		t.Fatalf("got %+v", res)
	}
	if len(res.Failures) != 1 || res.Failures[0].Path != good {
		t.Errorf("failures = %+v", res.Failures)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "Archives", "photos.zip")); err != nil {
		t.Error("the step after the failure did not run")
	}
}
