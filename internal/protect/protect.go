// Package protect says which files must not be touched.
//
// Contract (docs/safety-model.md, rule 10):
//   - Files in a code project are left alone: a folder holding .git, .hg, .svn,
//     or a project manifest such as go.mod, package.json, Cargo.toml,
//     pyproject.toml, setup.py, Gemfile, pom.xml, build.gradle, composer.json,
//     CMakeLists.txt, Package.swift, *.sln, *.csproj, *.xcodeproj.
//   - Files in a notes library are left alone: a folder holding .obsidian, or
//     logseq/config.edn.
//   - Everything in the user's `keep` list is left alone, with its children.
//   - Anything else may be touched; Guard returns "" for it.
//
// Where a rule says `subfolders = true` these checks are what stops a scan from
// walking into a project: the closest marker wins.
package protect

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/plan"
)

// Guard answers whether a path may be touched.
type Guard interface {
	// Protected returns the reason a path must be left alone, or "" if it may be touched.
	Protected(path string) plan.SkipReason
}

// codeFiles are names that make a folder a code project. Compared lower case.
var codeFiles = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"go.mod": true, "package.json": true, "cargo.toml": true,
	"pyproject.toml": true, "setup.py": true, "gemfile": true,
	"pom.xml": true, "build.gradle": true, "composer.json": true,
	"cmakelists.txt": true, "package.swift": true,
}

// codeSuffixes are the same, for names that vary per project.
var codeSuffixes = []string{".sln", ".csproj", ".xcodeproj"}

// notesMarkers are folders that make a folder a notes library.
const (
	obsidianMarker = ".obsidian"
	logseqFolder   = "logseq"
	logseqMarker   = "config.edn"
)

// guard is the concrete Guard. It is used by one goroutine at a time (a single
// run of the command), so the cache needs no lock.
type guard struct {
	keep  []string
	cache map[string]plan.SkipReason
}

// New returns a Guard for cfg: code projects, notes libraries, and everything
// in cfg.Keep.
func New(cfg config.Config) Guard {
	g := &guard{cache: make(map[string]plan.SkipReason, 64)}
	for _, k := range cfg.Keep {
		if strings.TrimSpace(k) == "" {
			continue
		}
		g.keep = append(g.keep, filepath.Clean(config.ExpandHome(k)))
	}
	return g
}

// Protected walks up from the path's folder. The closest marker wins: a notes
// vault inside a code repository is a notes library, because that is the more
// specific thing the user made there.
func (g *guard) Protected(path string) plan.SkipReason {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.Clean(abs)

	for _, keep := range g.keep {
		if abs == keep || strings.HasPrefix(abs, keep+string(filepath.Separator)) {
			return plan.SkipKept
		}
	}

	home := homeFolder()
	for dir := filepath.Dir(abs); ; {
		parent := filepath.Dir(dir)
		if dir == home || parent == dir {
			// The home folder and a volume root are never a project: one stray
			// package.json in ~ would otherwise protect every file below it and
			// the tool would quietly do nothing at all.
			return ""
		}
		if reason := g.judge(dir); reason != "" {
			return reason
		}
		dir = parent
	}
}

// homeFolder is the user's home folder, cleaned, or "" when it is unknown.
func homeFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Clean(home)
}

// judge answers for one folder, remembering the answer: every file in the same
// folder would otherwise read that folder again.
func (g *guard) judge(dir string) plan.SkipReason {
	if reason, ok := g.cache[dir]; ok {
		return reason
	}
	reason := classify(dir)
	g.cache[dir] = reason
	return reason
}

// classify looks at one folder. A folder that cannot be read protects nothing:
// we cannot claim a marker we cannot see, and reporting an error here would
// turn a permissions problem into a failed run rather than a skipped file.
func classify(dir string) plan.SkipReason {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	notes, code := false, false
	for _, entry := range entries {
		name := entry.Name()
		switch name {
		case obsidianMarker:
			notes = true
		case logseqFolder:
			if _, err := os.Lstat(filepath.Join(dir, logseqFolder, logseqMarker)); err == nil {
				notes = true
			}
		}
		lower := strings.ToLower(name)
		if codeFiles[lower] {
			code = true
			continue
		}
		for _, suffix := range codeSuffixes {
			if strings.HasSuffix(lower, suffix) {
				code = true
				break
			}
		}
	}
	if notes {
		return plan.SkipInNotesLibrary
	}
	if code {
		return plan.SkipInCodeProject
	}
	return ""
}
