// Package journal is the operation record: an append-only JSON Lines file with
// one line per step (milestone M1). Undo is built on it; so is `history`.
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
	"errors"
	"time"
)

// ErrNotImplemented marks a milestone that has not been built yet.
var ErrNotImplemented = errors.New("not implemented yet")

// Result of a step.
type Result string

const (
	OK      Result = "ok"
	Skipped Result = "skipped"
	Failed  Result = "failed"
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
}

// Journal reads and appends entries.
type Journal interface {
	Append(Entry) error
	// Batches returns batch ids, newest first.
	Batches() ([]string, error)
	Entries(batch string) ([]Entry, error)
}
