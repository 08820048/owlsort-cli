// Package doctor answers "will this work on this machine?" for `owlsort doctor`.
//
// It checks only what the program needs to run: the configuration can be found
// and read, the watched folders are there, the journal can be written, and the
// platform's trash works. It changes nothing — no folder is created, no journal
// is touched — because a check that fixes things cannot be run to find out what
// is wrong.
package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/journal"
	"github.com/08820048/owlsort-cli/internal/trash"
)

// Check is one thing that was looked at.
type Check struct {
	Name    string // what was checked, in a few words
	Detail  string // the answer: a path, a count, or why it failed
	OK      bool
	Warning bool // usable, but worth knowing
}

// Report is everything doctor found.
type Report struct {
	ConfigPath  string
	JournalPath string
	Checks      []Check
}

// Failed reports whether anything makes the program unusable.
func (r Report) Failed() bool {
	for _, c := range r.Checks {
		if !c.OK && !c.Warning {
			return true
		}
	}
	return false
}

// Run checks the machine. configPath may be "" when the configuration could not
// even be located.
func Run(cfg *config.Config, configPath, configErr string) Report {
	r := Report{ConfigPath: configPath}

	switch {
	case configErr != "":
		r.Checks = append(r.Checks, Check{Name: "configuration", Detail: configErr})
	case cfg == nil:
		r.Checks = append(r.Checks, Check{Name: "configuration", Detail: "not read"})
	default:
		problems := cfg.Problems()
		if len(problems) > 0 {
			detail := fmt.Sprintf("%s has %d problem(s); run `owlsort rules check`", configPath, len(problems))
			r.Checks = append(r.Checks, Check{Name: "configuration", Detail: detail})
		} else {
			r.Checks = append(r.Checks, Check{
				Name: "configuration", OK: true,
				Detail: fmt.Sprintf("%s, %d rule(s)", configPath, len(cfg.Rules)),
			})
		}
	}

	if path, err := journal.DefaultPath(); err != nil {
		r.Checks = append(r.Checks, Check{Name: "journal", Detail: err.Error()})
	} else {
		r.JournalPath = path
		r.Checks = append(r.Checks, checkWritableFolder("journal", path))
	}

	// The trash only matters if some rule sends a file to it. Reporting a
	// failure for something the configuration never uses would send the user
	// chasing a problem they do not have.
	needsTrash := false
	if cfg != nil {
		for _, rule := range cfg.Rules {
			if rule.Trash && rule.IsEnabled() {
				needsTrash = true
				break
			}
		}
	}
	if err := trash.Available(); err != nil {
		check := Check{Name: "system trash", Detail: err.Error()}
		if !needsTrash {
			check.Warning = true
			check.Detail += " (no rule uses the trash, so this does not stop anything)"
		}
		r.Checks = append(r.Checks, check)
	} else {
		r.Checks = append(r.Checks, Check{Name: "system trash", OK: true, Detail: "usable"})
	}

	if cfg != nil {
		r.Checks = append(r.Checks, checkFolders(cfg.Folders)...)
	}
	return r
}

// checkWritableFolder reports whether the folder that would hold path exists or
// can be made, and whether an existing file there is usable. It never creates
// anything.
func checkWritableFolder(name, path string) Check {
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return Check{Name: name, Detail: fmt.Sprintf("%s is not a folder", dir)}
		}
		if existing, err := os.Stat(path); err == nil && !existing.Mode().IsRegular() {
			return Check{Name: name, Detail: fmt.Sprintf("%s is not a regular file", path)}
		}
		return Check{Name: name, OK: true, Detail: path}
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return Check{Name: name, Detail: err.Error()}
	}
	// Not there yet: fine as long as the nearest existing parent is a folder we
	// could write to.
	for d := dir; ; d = filepath.Dir(d) {
		if _, statErr := os.Stat(d); statErr == nil {
			return Check{Name: name, OK: true, Detail: path + " (will be created)"}
		}
		parent := filepath.Dir(d)
		if parent == d {
			return Check{Name: name, Detail: fmt.Sprintf("no folder to create %s in", dir)}
		}
	}
}

// checkFolders reports on each watched folder: it must exist, be a folder, and be
// readable.
func checkFolders(folders []string) []Check {
	if len(folders) == 0 {
		return []Check{{Name: "watched folders", Detail: "none configured; `owlsort preview` has nothing to look at"}}
	}
	checks := make([]Check, 0, len(folders))
	for _, folder := range folders {
		path := filepath.Clean(filepath.Join(config.ExpandHome(folder)))
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		info, err := os.Stat(path)
		switch {
		case err != nil:
			checks = append(checks, Check{Name: "watched folder", Detail: fmt.Sprintf("%s: %v", path, err)})
		case !info.IsDir():
			checks = append(checks, Check{Name: "watched folder", Detail: fmt.Sprintf("%s is not a folder", path)})
		default:
			// Can we actually read the names in it?
			if entries, err := os.ReadDir(path); err != nil {
				checks = append(checks, Check{Name: "watched folder", Detail: fmt.Sprintf("%s: %v", path, err)})
			} else {
				checks = append(checks, Check{
					Name: "watched folder", OK: true,
					Detail: fmt.Sprintf("%s (%d file(s) at the top level)", displayPath(path), countFiles(entries)),
				})
			}
		}
	}
	return checks
}

// countFiles counts the entries that are not folders.
func countFiles(entries []os.DirEntry) int {
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			n++
		}
	}
	return n
}

// displayPath shortens the home folder to "~", the way the rest of the program
// prints paths.
func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	if rel == "." {
		return "~"
	}
	return "~/" + filepath.ToSlash(rel)
}
