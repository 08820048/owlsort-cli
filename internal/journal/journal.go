// Package journal is the operation record: an append-only JSON Lines file with
// one line per step. Undo is built on it; so is `history`.
//
// Rules (docs/architecture.md, "The journal"):
//   - Append only. An undo is a new line, never an edit of an old one.
//   - A line is written before the next step starts, so a crash loses at most
//     the step in flight, and `undo` can still put back everything before it.
//   - A line that cannot be read is skipped with a warning; it never stops
//     `undo` from reading the rest.
//   - The file holds the user's file paths. The README says so.
package journal

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

// EntryVersion is the version of a journal line. A reader accepts any version
// it understands and skips the rest with a warning.
const EntryVersion = 1

// Result of a step.
type Result string

const (
	OK      Result = "ok"
	Skipped Result = "skipped"
	Failed  Result = "failed"
)

// Op is what a line records. An undo is its own op, pointing from where the
// file is back to where it belongs.
const (
	OpMove   = "move"
	OpCopy   = "copy"
	OpRename = "rename"
	OpTrash  = "trash"
	OpUndo   = "undo"
)

// Entry is one line of the journal. The JSON keys are a stable format.
type Entry struct {
	Version int       `json:"v"`
	Batch   string    `json:"batch"` // e.g. "20261118-142503-a1b2"
	Time    time.Time `json:"time"`
	Op      string    `json:"op"` // move | copy | rename | trash | undo
	Source  string    `json:"src"`
	Dest    string    `json:"dst,omitempty"`
	Rule    string    `json:"rule,omitempty"`
	Result  Result    `json:"result"`
	Error   string    `json:"error,omitempty"`
	// Created is the topmost folder this step had to make. Undo takes it away
	// again when it is empty, so a run and its undo leave the folder as it was.
	Created string `json:"created,omitempty"`
}

// Journal reads and appends entries.
type Journal interface {
	Append(Entry) error
	// Batches returns batch ids, newest first.
	Batches() ([]string, error)
	Entries(batch string) ([]Entry, error)
}

// File is the journal on disk.
type File struct {
	path string
}

// DefaultPath is where the journal lives: $XDG_STATE_HOME/owlsort/journal.jsonl
// on macOS and Linux (~/.local/state/owlsort/ when the variable is unset), and
// %LOCALAPPDATA%\owlsort\journal.jsonl on Windows.
func DefaultPath() (string, error) {
	if runtime.GOOS == "windows" {
		dir := os.Getenv("LOCALAPPDATA")
		if dir == "" {
			return "", errors.New("journal: LOCALAPPDATA is not set")
		}
		return filepath.Join(dir, "owlsort", "journal.jsonl"), nil
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "owlsort", "journal.jsonl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("journal: %w", err)
	}
	return filepath.Join(home, ".local", "state", "owlsort", "journal.jsonl"), nil
}

// Open returns the journal at path. The file is created when the first entry is
// appended, not now, so `preview` never leaves anything behind.
func Open(path string) (*File, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("journal: %w", err)
	}
	return &File{path: abs}, nil
}

// Path is where the journal is written.
func (f *File) Path() string { return f.path }

// Append writes one line and flushes it. A half-written last line is the most a
// crash can leave behind; the reader skips it.
func (f *File) Append(e Entry) error {
	if e.Version == 0 {
		e.Version = EntryVersion
	}
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	file, err := os.OpenFile(f.path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	// A crash can leave a half-written last line with no newline. Closing that
	// line off first keeps this entry readable, and confines the damage to the
	// one line that was already lost.
	prefix := []byte{}
	if !endsWithNewline(file) {
		prefix = []byte{'\n'}
	}
	if _, err := file.Write(append(prefix, append(line, '\n')...)); err != nil {
		file.Close()
		return fmt.Errorf("journal: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	return nil
}

// endsWithNewline reports whether the file's last byte is a newline. An empty
// file counts as yes: there is nothing to close off.
func endsWithNewline(f *os.File) bool {
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return true
	}
	buf := make([]byte, 1)
	if _, err := f.ReadAt(buf, info.Size()-1); err != nil {
		return true
	}
	return buf[0] == '\n'
}

// Read returns every entry, oldest first, and one message per line that could
// not be read. A damaged line never stops the rest from being read: an undo
// that refuses to run because of one bad line would be worse than useless.
func (f *File) Read() ([]Entry, []string, error) {
	file, err := os.Open(f.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("journal: %w", err)
	}
	defer file.Close()

	var (
		entries []Entry
		bad     []string
		line    int
	)
	scanner := bufio.NewScanner(file)
	// A line holds two paths, so the default 64 KB is not always enough.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line++
		text := scanner.Bytes()
		if len(text) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(text, &e); err != nil {
			bad = append(bad, fmt.Sprintf("line %d: %v", line, err))
			continue
		}
		if e.Version > EntryVersion {
			bad = append(bad, fmt.Sprintf("line %d: version %d is newer than this build understands", line, e.Version))
			continue
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return entries, bad, fmt.Errorf("journal: %w", err)
	}
	return entries, bad, nil
}

// Batches returns batch ids, newest first.
func (f *File) Batches() ([]string, error) {
	runs, err := f.Runs()
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(runs))
	for i, run := range runs {
		ids[i] = run.Batch
	}
	return ids, nil
}

// Entries returns one batch in the order the steps were written, so an undo can
// walk it backwards.
func (f *File) Entries(batch string) ([]Entry, error) {
	all, _, err := f.Read()
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, e := range all {
		if e.Batch == batch {
			out = append(out, e)
		}
	}
	return out, nil
}

// Run is one batch, summarised for `history`.
type Run struct {
	Batch    string    `json:"batch"`
	Time     time.Time `json:"time"`              // when the first step was written
	Moved    int       `json:"moved"`             // steps that went through
	Trashed  int       `json:"trashed,omitempty"` // of which went to the trash
	Skipped  int       `json:"skipped,omitempty"`
	Failed   int       `json:"failed,omitempty"`
	To       []string  `json:"to,omitempty"`        // destination folders, without duplicates
	Undone   bool      `json:"undone,omitempty"`    // an undo of this batch has been written
	UndoDone int       `json:"undo_done,omitempty"` // steps the undo put back
}

// Runs summarises every batch, newest first.
func (f *File) Runs() ([]Run, error) {
	all, _, err := f.Read()
	if err != nil {
		return nil, err
	}
	order := []string{}
	byBatch := map[string]*Run{}
	toSets := map[string]map[string]bool{}
	for _, e := range all {
		run, ok := byBatch[e.Batch]
		if !ok {
			run = &Run{Batch: e.Batch, Time: e.Time}
			byBatch[e.Batch] = run
			toSets[e.Batch] = map[string]bool{}
			order = append(order, e.Batch)
		}
		if e.Time.Before(run.Time) {
			run.Time = e.Time
		}
		switch {
		case e.Op == OpUndo:
			run.Undone = true
			if e.Result == OK {
				run.UndoDone++
			}
		case e.Result == OK:
			run.Moved++
			if e.Op == OpTrash {
				run.Trashed++
			} else if e.Dest != "" {
				toSets[e.Batch][filepath.Dir(e.Dest)] = true
			}
		case e.Result == Skipped:
			run.Skipped++
		case e.Result == Failed:
			run.Failed++
		}
	}

	runs := make([]Run, 0, len(order))
	for _, batch := range order {
		run := byBatch[batch]
		for dir := range toSets[batch] {
			run.To = append(run.To, dir)
		}
		sort.Strings(run.To)
		runs = append(runs, *run)
	}
	// Newest first: by time, then by id so two runs in the same second are stable.
	sort.SliceStable(runs, func(i, j int) bool {
		if !runs[i].Time.Equal(runs[j].Time) {
			return runs[i].Time.After(runs[j].Time)
		}
		return runs[i].Batch > runs[j].Batch
	})
	return runs, nil
}

// NewBatchID is the id of one run: "20261118-142503-a1b2" — the day, the time,
// and four hexadecimal digits so two runs in the same second stay apart.
func NewBatchID(t time.Time) string {
	return t.Format("20060102-150405") + "-" + randomHex(2)
}

// randomHex returns n random bytes as hex. A failure is not worth stopping a run
// for: the timestamp alone is still a usable id.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "0000"
	}
	return hex.EncodeToString(b)
}
