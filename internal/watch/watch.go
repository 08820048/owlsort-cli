// Package watch is the foreground monitor behind `owlsort watch` (milestone M3).
//
// Contract (docs/safety-model.md, rule 9, and docs/architecture.md, "Watching"):
//   - Only the top level of each watched folder is watched.
//   - Events are coalesced per path; a file is tidied only after it has stopped
//     changing for a few seconds (downloads write for a long time).
//   - Unattended means conservative: only rules with `background = true` may
//     trash; moves, copies and renames are allowed.
//   - It does not start if the desktop app is tidying the same folder in the
//     background, unless forced.
package watch

import "errors"

// ErrNotImplemented marks a milestone that has not been built yet.
var ErrNotImplemented = errors.New("not implemented yet")

// Run watches folders until the context ends. Milestone M3.
func Run() error { return ErrNotImplemented }
