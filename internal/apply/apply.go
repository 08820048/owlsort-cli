// Package apply carries a plan out (milestone M1).
//
// Contract (docs/safety-model.md):
//   - Every step is re-checked right before it runs: the source is still
//     there and unchanged since the preview, the destination folder is
//     creatable. A step that fails the check is skipped with a reason; the
//     rest go on.
//   - Never overwrite. A clash is resolved by the rule's conflict strategy
//     (rename "name 2.ext", or skip).
//   - A step is written to the journal before the next one starts.
//   - Moves across volumes copy, verify the size, then remove the source.
//   - Undo walks a batch backwards and, for each step, puts the file back only
//     if it is still at the destination and the original place is free.
package apply

import (
	"errors"

	"github.com/08820048/owlsort-cli/internal/journal"
	"github.com/08820048/owlsort-cli/internal/plan"
)

// ErrNotImplemented marks a milestone that has not been built yet.
var ErrNotImplemented = errors.New("not implemented yet")

// Result of a run.
type Result struct {
	Batch   string
	Done    int
	Skipped []plan.Skipped
	Failed  int
}

// Run carries out p and records it in j. Milestone M1.
func Run(p plan.Plan, j journal.Journal) (Result, error) {
	return Result{}, ErrNotImplemented
}

// Undo puts back a batch ("" is the newest). Milestone M1.
func Undo(batch string, j journal.Journal) (Result, error) {
	return Result{}, ErrNotImplemented
}
