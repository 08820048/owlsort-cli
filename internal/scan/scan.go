// Package scan lists the files of a watched folder.
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
//
// TODO(windows): list OneDrive online-only files and hidden/system files by
// their attributes, not by their names. Windows needs golang.org/x/sys for that
// and the dependency is not in go.mod yet (milestone M1).
package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// settleTime is how long a file must have been quiet before it counts as
// finished. A download writes for seconds or minutes; a file that changed a
// moment ago may still be growing.
const settleTime = 5 * time.Second

// downloadingExtensions are the names a browser, a downloader or a text editor
// leaves on a file that is still being written.
var downloadingExtensions = map[string]bool{
	"crdownload": true, // Chrome, Edge
	"part":       true, // Firefox
	"partial":    true, // Edge
	"download":   true, // Safari
	"tmp":        true, // editors, downloads
	"!ut":        true, // uTorrent
}

// packageExtensions are folder names the OS shows as one thing. For us they are
// a single file: never opened, never walked into.
var packageExtensions = map[string]bool{
	"app": true, "bundle": true, "framework": true, "plugin": true,
	"kext": true, "prefpane": true, "qlgenerator": true,
}

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
	// Downloading is set for names like .crdownload / .part / .download, for
	// cloud placeholders, and for files that changed in the last few seconds.
	Downloading bool
	// Package is a folder the OS treats as one file (a macOS ".app"). Its Size
	// is unknown (0), so size conditions never match it.
	Package bool
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

// Folder lists the files of one watched folder.
func Folder(path string, opts Options) ([]File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan: %s is not a folder", path)
	}
	folder, err := filepath.Abs(path)
	if err != nil {
		folder = filepath.Clean(path)
	}
	files := []File{}
	if err := list(folder, folder, opts, time.Now(), &files); err != nil {
		return nil, err
	}
	return files, nil
}

// list appends the files of dir to out. folder is the watched folder the walk
// started from, which is what File.Folder reports.
func list(folder, dir string, opts Options, now time.Time, out *[]File) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)
		// Lstat, never Stat: a symbolic link is listed as itself and is never
		// followed, so a link cannot lead the scan out of the watched folder.
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		isPackage := info.IsDir() && packageExtensions[strings.ToLower(extension(name))]
		if info.IsDir() && !isPackage {
			if !opts.Subfolders {
				continue
			}
			if err := list(folder, path, opts, now, out); err != nil {
				return err
			}
			continue
		}
		*out = append(*out, File{
			Path:        path,
			Name:        name,
			Extension:   extension(name),
			Size:        size(info, isPackage),
			Modified:    info.ModTime(),
			Created:     created(info),
			Folder:      folder,
			Hidden:      strings.HasPrefix(name, "."),
			Downloading: downloading(name, info.ModTime(), now),
			Package:     isPackage,
		})
	}
	return nil
}

// extension is the suffix after the last dot, without the dot, as on disk.
// A name with no suffix ("README") and an all-dot name (".DS_Store") have none.
func extension(name string) string {
	ext := filepath.Ext(name)
	if len(ext) < 2 || ext == name {
		return ""
	}
	return ext[1:]
}

// size is the file's size. A package is a folder: its size is not computed,
// because walking into one is exactly what we promised not to do.
func size(info os.FileInfo, isPackage bool) int64 {
	if isPackage {
		return 0
	}
	return info.Size()
}

// downloading reports whether a file may still be being written.
func downloading(name string, modified, now time.Time) bool {
	if downloadingExtensions[strings.ToLower(extension(name))] {
		return true
	}
	// An iCloud placeholder has not been downloaded at all. It is never read:
	// reading one makes macOS fetch the file from the cloud.
	if strings.HasSuffix(strings.ToLower(name), ".icloud") {
		return true
	}
	return now.Sub(modified) < settleTime
}
