package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openTemp(t *testing.T) *File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state", "owlsort", "journal.jsonl")
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func entry(batch, op, src, dst string) Entry {
	return Entry{
		Batch: batch, Time: time.Date(2026, 11, 18, 14, 25, 3, 0, time.UTC),
		Op: op, Source: src, Dest: dst, Result: OK,
	}
}

func TestAppendThenReadBack(t *testing.T) {
	f := openTemp(t)
	if _, err := os.Stat(f.Path()); !os.IsNotExist(err) {
		t.Fatal("Open must not create the file")
	}
	if err := f.Append(entry("b1", OpMove, "/w/a.pdf", "/w/PDF/a.pdf")); err != nil {
		t.Fatal(err)
	}
	entries, bad, err := f.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 0 {
		t.Errorf("bad lines: %v", bad)
	}
	if len(entries) != 1 || entries[0].Source != "/w/a.pdf" {
		t.Fatalf("got %+v", entries)
	}
	if entries[0].Version != EntryVersion {
		t.Errorf("version = %d, want %d", entries[0].Version, EntryVersion)
	}
}

func TestADamagedLineDoesNotStopTheRest(t *testing.T) {
	f := openTemp(t)
	if err := f.Append(entry("b1", OpMove, "/w/a.pdf", "/w/PDF/a.pdf")); err != nil {
		t.Fatal(err)
	}
	// A half-written line is what a crash leaves behind.
	file, err := os.OpenFile(f.Path(), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"batch":"b1","src":"/w/tr`); err != nil {
		t.Fatal(err)
	}
	file.Close()
	if err := f.Append(entry("b2", OpMove, "/w/b.pdf", "/w/PDF/b.pdf")); err != nil {
		t.Fatal(err)
	}

	entries, bad, err := f.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 1 {
		t.Errorf("bad = %v, want one damaged line reported", bad)
	}
	if len(entries) != 2 {
		t.Errorf("read %d entries, want both good ones", len(entries))
	}
}

func TestBatchesAreNewestFirst(t *testing.T) {
	f := openTemp(t)
	first := entry("b1", OpMove, "/w/a.pdf", "/w/PDF/a.pdf")
	second := entry("b2", OpMove, "/w/b.pdf", "/w/PDF/b.pdf")
	second.Time = first.Time.Add(time.Minute)
	for _, e := range []Entry{first, second} {
		if err := f.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	batches, err := f.Batches()
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || batches[0] != "b2" || batches[1] != "b1" {
		t.Errorf("got %v, want [b2 b1]", batches)
	}
}

func TestEntriesAreReturnedInTheOrderWritten(t *testing.T) {
	f := openTemp(t)
	for _, name := range []string{"a", "b", "c"} {
		if err := f.Append(entry("b1", OpMove, "/w/"+name, "/w/x/"+name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Append(entry("b2", OpMove, "/w/z", "/w/x/z")); err != nil {
		t.Fatal(err)
	}
	entries, err := f.Entries("b1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries", len(entries))
	}
	for i, want := range []string{"/w/a", "/w/b", "/w/c"} {
		if entries[i].Source != want {
			t.Errorf("entry %d = %q, want %q", i, entries[i].Source, want)
		}
	}
}

func TestAMissingJournalIsEmptyNotAnError(t *testing.T) {
	f := openTemp(t)
	entries, bad, err := f.Read()
	if err != nil || len(entries) != 0 || len(bad) != 0 {
		t.Errorf("got %+v %v %v", entries, bad, err)
	}
}

func TestRunSummarisesABatch(t *testing.T) {
	f := openTemp(t)
	steps := []Entry{
		entry("b1", OpMove, "/w/a.pdf", "/w/Documents/PDF/a.pdf"),
		entry("b1", OpMove, "/w/b.zip", "/w/Archives/b.zip"),
		entry("b1", OpTrash, "/w/junk.tmp", "/home/.Trash/junk.tmp"),
	}
	skipped := entry("b1", OpMove, "/w/c.pdf", "/w/Documents/PDF/c.pdf")
	skipped.Result = Skipped
	for _, e := range append(steps, skipped) {
		if err := f.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := f.Runs()
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("got %d runs", len(runs))
	}
	run := runs[0]
	if run.Moved != 3 || run.Trashed != 1 || run.Skipped != 1 {
		t.Errorf("got %+v", run)
	}
	if len(run.To) != 2 {
		t.Errorf("destinations = %v, want two folders", run.To)
	}
	if run.Undone {
		t.Error("nothing has been undone yet")
	}
}

func TestBatchIDsDifferWithinTheSameSecond(t *testing.T) {
	at := time.Date(2026, 11, 18, 14, 25, 3, 0, time.UTC)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id := NewBatchID(at)
		if !strings.HasPrefix(id, "20261118-142503-") {
			t.Fatalf("id = %q", id)
		}
		seen[id] = true
	}
	// Four hex digits: a collision in fifty draws is possible but should not
	// happen every time. Anything less would be a broken generator.
	if len(seen) < 2 {
		t.Error("every id in the same second was identical")
	}
}
