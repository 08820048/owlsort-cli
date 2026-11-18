package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/08820048/owlsort-cli/internal/atomicfile"
	"github.com/08820048/owlsort-cli/internal/tomledit"
)

// Edit changes the configuration file as text.
//
// fn makes the change; then the result is parsed again before anything is
// written. A command can therefore never leave a file that will not load, which
// matters more than any feature here: the configuration is what the program
// obeys, and a broken one stops it entirely (docs/safety-model.md, rule 12).
//
// The write goes to a temporary file in the same folder and is renamed over the
// target, so the file is never half-written. A symlink is followed rather than
// replaced, because a configuration kept in dotfiles is usually a link and
// replacing it would quietly break the setup it points at. Its permissions are
// kept; Windows has no such bits, so there the file is whatever Windows makes it.
//
// changed is false when fn made no edit, so a caller can say so.
func Edit(path string, fn func(*tomledit.Doc) error) (changed bool, err error) {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}

	info, err := os.Stat(target)
	if err != nil {
		return false, fmt.Errorf("config: %w", err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("config: %s is not a regular file", target)
	}

	src, err := os.ReadFile(target)
	if err != nil {
		return false, fmt.Errorf("config: %w", err)
	}
	doc := tomledit.Parse(src)
	if err := fn(doc); err != nil {
		return false, err
	}
	out := doc.String()
	if out == string(src) {
		return false, nil
	}

	if err := checkStillUsable(src, []byte(out)); err != nil {
		return false, err
	}
	if err := atomicfile.Write(target, []byte(out), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

// checkStillUsable is the guard: the file after the edit must still be a
// configuration, and it must still hold the same rules. A text edit that lost a
// rule would be a silent disaster — the user's files would stop being tidied and
// nothing would say why.
func checkStillUsable(before, after []byte) error {
	was, err := ParseLoose(before)
	if err != nil {
		return fmt.Errorf("config: %s is not a valid configuration to begin with: %w", "the file", err)
	}
	now, err := ParseLoose(after)
	if err != nil {
		return fmt.Errorf("config: the edit would leave a file that does not parse, so nothing was written: %w", err)
	}

	if len(was.Rules) != len(now.Rules) {
		return fmt.Errorf(
			"config: the edit would change the number of rules from %d to %d, so nothing was written",
			len(was.Rules), len(now.Rules))
	}
	for i := range was.Rules {
		if was.Rules[i].Name != now.Rules[i].Name {
			return fmt.Errorf(
				"config: the edit would rename rule %d from %q to %q, so nothing was written",
				i+1, was.Rules[i].Name, now.Rules[i].Name)
		}
	}
	return nil
}

// SamePath reports whether two paths mean the same file, for comparing a folder
// the user typed with one already in the list. It expands "~", makes the path
// absolute and cleans it; a path that does not exist yet still compares as
// itself.
func SamePath(a, b string) bool {
	return normalizePath(a) == normalizePath(b)
}

func normalizePath(p string) string {
	p = filepath.Clean(ExpandHome(strings.TrimSpace(p)))
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	// A symlink and its target are the same folder as far as watching goes.
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return p
}
