// Package scan lists the files of a watched folder (milestone M0).
//
// Contract (docs/architecture.md, "Scanning"):
//   - Only the top level, unless the caller asks for more (a rule with
//     `subfolders = true`).
//   - Folders are never files. A package that the OS shows as one thing
//     (a macOS ".app") counts as one file and is never opened.
//   - Not read at all: cloud placeholders (macOS ".icloud", a Windows
//     OneDrive online-only file); reading one would download it.
//   - Hidden files are listed but marked, so the plan can say why they were skipped.
//   - Never follows symbolic links out of the folder.
package scan

import (
	"errors"
	"time"
)

// ErrNotImplemented marks a milestone that has not been built yet.
var ErrNotImplemented = errors.New("not implemented yet")

// File is what the rest of the program knows about a file.
type File struct {
	Path      string
	Name      string
	Extension string // without the dot, as on disk
	Size      int64
	Modified  time.Time
	Created   time.Time // birth time where the OS keeps it, else zero
	Folder    string    // the watched folder it was found under (not its own directory)
	Hidden    bool
	// Downloading is set for names like .crdownload / .part / .download and for
	// files that changed in the last few seconds.
	Downloading bool
}

// Arrived is the day the file landed where it is: its birth time, else its
// modification time. (macOS has an exact "date added"; the other systems do
// not, so the CLI does not pretend to.)
func (f File) Arrived() time.Time {
	if !f.Created.IsZero() {
		return f.Created
	}
	return f.Modified
}

// Options say how deep to look.
type Options struct {
	Subfolders bool
}

// Folder lists the files of one watched folder. Milestone M0.
func Folder(path string, opts Options) ([]File, error) {
	return nil, ErrNotImplemented
}
