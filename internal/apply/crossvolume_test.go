//go:build !windows

package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/journal"
)

// These tests exercise the one code path in the program that can lose a file if it
// is wrong: a move between two volumes, where the file has to be copied, checked
// and only then removed from where it was (docs/safety-model.md, rule 11).
//
// They need a second volume, which a CI runner does not have, so they skip unless
// OWLSORT_SECOND_VOLUME points at a directory on another volume:
//
//	make cross-volume-test
//
// does the whole thing on macOS: creates a small disk image, mounts it, runs these
// tests, unmounts and deletes it. On Linux, mount a tmpfs or a loopback filesystem
// and set the variable by hand.

// secondVolume is a directory on a different volume, or "" to skip.
func secondVolume(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("OWLSORT_SECOND_VOLUME")
	if dir == "" {
		t.Skip("set OWLSORT_SECOND_VOLUME to a directory on another volume (make cross-volume-test)")
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("OWLSORT_SECOND_VOLUME=%s: %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("OWLSORT_SECOND_VOLUME=%s is not a folder", dir)
	}
	// Being a different volume is the whole point; if the two paths are on the same
	// one, the test would silently exercise os.Rename and prove nothing.
	if sameVolume(t, dir) {
		t.Fatalf("OWLSORT_SECOND_VOLUME=%s is on the same volume as the temporary folder; this would test nothing", dir)
	}
	return dir
}

// sameVolume reports whether dir and the system temporary folder are one volume.
func sameVolume(t *testing.T, dir string) bool {
	t.Helper()
	a, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.Stat(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return deviceOf(a) == deviceOf(b)
}

// writeOn puts a file in dir and returns its path.
func writeOn(t *testing.T, dir, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	return path
}

// A move off the volume must leave the file, whole, at the destination and gone
// from where it was.
func TestMovingAcrossVolumesCopiesThenRemoves(t *testing.T) {
	other := secondVolume(t)
	src := writeOn(t, other, "report.pdf", "the contents survive the crossing")
	dst := filepath.Join(t.TempDir(), "Documents", "PDF", "report.pdf")

	// A rename would keep the inode. Comparing it afterwards is what proves this
	// really went through the copy path, rather than the test quietly passing on a
	// rename and never touching the code it is named after.
	before, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}

	f := newFixture(t)
	res, err := Run(f.plan(f.item(t, src, dst)), f.j, Options{Conflict: config.ConflictRename, Now: f.now})
	if err != nil {
		t.Fatal(err)
	}
	if res.Done != 1 {
		t.Fatalf("got %+v", res)
	}

	after, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("the file is not at the destination: %v", err)
	}
	if inodeOf(before) == inodeOf(after) {
		t.Error("the inode is unchanged: this was a rename, so the copy path was never run")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("the file is not at the destination: %v", err)
	}
	if string(got) != "the contents survive the crossing" {
		t.Errorf("the copy is wrong: %q", got)
	}
	if _, err := os.Lstat(src); !os.IsNotExist(err) {
		t.Error("the source is still on the other volume")
	}
}

// A file bigger than the hash threshold is verified by size instead, and must still
// arrive whole: the branch that skips hashing is a branch.
func TestMovingABigFileAcrossVolumes(t *testing.T) {
	other := secondVolume(t)
	content := strings.Repeat("0123456789abcdef", 128*1024) // 2 MiB, over hashLimit
	src := writeOn(t, other, "film.mov", content)
	dst := filepath.Join(t.TempDir(), "Media", "film.mov")

	f := newFixture(t)
	item := f.item(t, src, dst)
	if item.Size <= hashLimit {
		t.Fatalf("the file is %d bytes; the test is meant to be over %d", item.Size, hashLimit)
	}
	res, err := Run(f.plan(item), f.j, Options{Conflict: config.ConflictRename, Now: f.now})
	if err != nil {
		t.Fatal(err)
	}
	if res.Done != 1 {
		t.Fatalf("got %+v", res)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(content)) {
		t.Errorf("size at the destination is %d, want %d", info.Size(), len(content))
	}
	if _, err := os.Lstat(src); !os.IsNotExist(err) {
		t.Error("the source is still there")
	}
}

// The property that matters most: when the copy cannot be made, the source is
// untouched. A file tidier that removes first and copies second loses files on a
// full disk, and that is the failure this test exists to rule out.
func TestAFailedCrossVolumeMoveLeavesTheSourceAlone(t *testing.T) {
	other := secondVolume(t)
	src := writeOn(t, other, "report.pdf", "still here afterwards")

	// A destination folder that cannot be written to. The move fails here before
	// anything is copied, which is the earliest point it can fail; what matters is
	// that the source is still whole afterwards.
	blocked := filepath.Join(t.TempDir(), "Blocked")
	if err := os.MkdirAll(blocked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })
	if err := os.Chmod(blocked, 0o500); err != nil {
		t.Fatal(err)
	}

	f := newFixture(t)
	item := f.item(t, src, filepath.Join(blocked, "report.pdf"))
	res, err := Run(f.plan(item), f.j, Options{Conflict: config.ConflictRename, Now: f.now})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 || res.Done != 0 {
		t.Fatalf("got %+v", res)
	}
	if got, err := os.ReadFile(src); err != nil || string(got) != "still here afterwards" {
		t.Fatalf("the source was damaged by a failed move: %q %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(blocked, "report.pdf")); !os.IsNotExist(err) {
		t.Error("a half-written copy was left behind")
	}
}

// Undo has to bring a file back across the volume, which is the same code path in
// reverse — and it has to check first, as always.
func TestUndoAcrossVolumes(t *testing.T) {
	other := secondVolume(t)
	src := writeOn(t, other, "report.pdf", "and back again")
	dst := filepath.Join(t.TempDir(), "Documents", "PDF", "report.pdf")

	f := newFixture(t)
	res, err := Run(f.plan(f.item(t, src, dst)), f.j, Options{Conflict: config.ConflictRename, Now: f.now})
	if err != nil {
		t.Fatal(err)
	}
	if res.Done != 1 {
		t.Fatalf("got %+v", res)
	}

	undoRes, err := Undo(res.Batch, f.j, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if undoRes.Done != 1 {
		t.Fatalf("undo: got %+v", undoRes)
	}
	got, err := os.ReadFile(src)
	if err != nil || string(got) != "and back again" {
		t.Fatalf("the file did not come back: %q %v", got, err)
	}
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Error("the file is still at the destination")
	}
}

// A journal written by a move across volumes must record the destination, because
// that is the only thing undo knows to look at.
func TestACrossVolumeMoveIsJournaled(t *testing.T) {
	other := secondVolume(t)
	src := writeOn(t, other, "report.pdf", "x")
	dst := filepath.Join(t.TempDir(), "Documents", "report.pdf")

	f := newFixture(t)
	res, err := Run(f.plan(f.item(t, src, dst)), f.j, Options{Conflict: config.ConflictRename, Now: f.now})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := f.j.Entries(res.Batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %+v", entries)
	}
	if entries[0].Result != journal.OK || entries[0].Dest != dst {
		t.Errorf("entry = %+v", entries[0])
	}
	if entries[0].Source != src {
		t.Errorf("source = %q, want %q", entries[0].Source, src)
	}
}
