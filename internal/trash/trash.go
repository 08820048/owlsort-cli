// Package trash puts a file in the system trash (milestone M1). It is the
// heaviest thing the program can do to a file: nothing here deletes for good.
//
// Per platform (docs/architecture.md, "Platforms"):
//   - macOS:   move into ~/.Trash, renaming on a clash. The Finder's "Put Back"
//     is not available; `undo` works from the journal.
//   - Linux:   the freedesktop.org trash spec: ~/.local/share/Trash/files plus
//     an .trashinfo file, so the desktop's "Restore" works.
//   - Windows: the Shell recycle bin (SHFileOperationW with FOF_ALLOWUNDO), so
//     Explorer can restore it.
package trash

import "errors"

// ErrNotImplemented marks a milestone that has not been built yet.
var ErrNotImplemented = errors.New("not implemented yet")

// Move puts path in the system trash and returns where it went.
// Implemented per platform in trash_<os>.go.
func Move(path string) (string, error) { return move(path) }
