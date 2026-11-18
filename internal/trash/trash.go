// Package trash puts a file in the system trash. It is the heaviest thing the
// program can do to a file: nothing here deletes for good.
//
// Per platform (docs/architecture.md, "Platforms"):
//   - macOS:   move into ~/.Trash, renaming on a clash. The Finder's "Put Back"
//     is not available; `undo` works from the journal.
//   - Linux:   the freedesktop.org trash spec: ~/.local/share/Trash/files plus
//     an .trashinfo file, so the desktop's "Restore" works.
//   - Windows: the Shell recycle bin (SHFileOperationW with FOF_ALLOWUNDO), so
//     Explorer can restore it.
//
// A move to the trash across volumes is refused with a clear error rather than
// silently doing something else: `undo` puts a file back from the path this
// package returns, and a copy would leave the original in place.
package trash

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/08820048/owlsort-cli/internal/naming"
)

// Move puts path in the system trash and returns where it went.
// Implemented per platform in trash_<os>.go.
func Move(path string) (string, error) { return move(path) }

// freePath is the first name in dir that is not taken, in the same "name 2.ext"
// form a move uses.
func freePath(dir, name string) string {
	free := naming.Free(name, func(candidate string) bool {
		_, err := os.Lstat(filepath.Join(dir, candidate))
		return err == nil
	})
	return filepath.Join(dir, free)
}

// renameInto moves path into dir under a free name. It never overwrites.
func renameInto(path, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("trash: %w", err)
	}
	target := freePath(dir, filepath.Base(path))
	if err := os.Rename(path, target); err != nil {
		if isCrossDevice(err) {
			return "", fmt.Errorf("trash: %s is on another volume than %s; move it there by hand first", path, dir)
		}
		return "", fmt.Errorf("trash: %w", err)
	}
	return target, nil
}

// isCrossDevice reports whether an error means the file is on another volume.
func isCrossDevice(err error) bool {
	return errors.Is(err, errCrossDevice)
}
