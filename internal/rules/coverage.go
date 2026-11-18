package rules

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/protect"
	"github.com/08820048/owlsort-cli/internal/scan"
)

// UncoveredGroup is one extension and the files no rule takes.
type UncoveredGroup struct {
	// Extension is the extension without the dot, lower case; "" for files that
	// have none. It is what the user would add to a rule.
	Extension string
	// Count is how many loose files have it.
	Count int
	// Examples are a few of the names, so the user can see what they are.
	Examples []string
}

// Uncovered lists the files no rule takes, grouped by extension, most first. A
// file that is protected, hidden or still downloading is not "uncovered": it was
// left alone for a reason the plan already reports, and telling the user to
// write a rule for it would be wrong advice.
//
// It runs the same skipReason and decide code as Build, so it cannot disagree
// with `preview` about whether a file is planned.
func Uncovered(files []scan.File, cfg config.Config, guard protect.Guard, env Env) ([]UncoveredGroup, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	env = env.withDefaults()

	// Only the top level of a watched folder is looked at, unless a rule asks
	// for subfolders: a file deep inside a folder the user arranged is not
	// something to nag about.
	subfolders := false
	for _, r := range cfg.Rules {
		if r.Subfolders {
			subfolders = true
			break
		}
	}

	groups := map[string]*UncoveredGroup{}
	for _, f := range files {
		if !subfolders && !isTopLevel(f) {
			continue
		}
		if skipReason(f, guard) != "" {
			continue
		}
		_, st, err := decide(f, cfg, env)
		if err != nil {
			return nil, err
		}
		if st != stateNone {
			continue
		}
		// decide reports stateNone both for "no rule" and for "already filed".
		// Only the first is uncovered: a filed file matches a rule that has
		// already done its work.
		if covered(f, cfg, env) {
			continue
		}

		key := strings.ToLower(f.Extension)
		group, ok := groups[key]
		if !ok {
			group = &UncoveredGroup{Extension: key}
			groups[key] = group
		}
		group.Count++
		if len(group.Examples) < 3 {
			group.Examples = append(group.Examples, f.Name)
		}
	}

	out := make([]UncoveredGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Extension < out[j].Extension
	})
	return out, nil
}

// covered reports whether some rule's conditions hold for the file, whatever it
// would do with it.
func covered(f scan.File, cfg config.Config, env Env) bool {
	for _, r := range cfg.Rules {
		if r.IsEnabled() && match(f, r, env) {
			return true
		}
	}
	return false
}

// isTopLevel reports whether a file sits directly in its watched folder.
func isTopLevel(f scan.File) bool {
	return filepath.Dir(f.Path) == filepath.Clean(f.Folder)
}

// ExtensionLabel is how a group's extension is shown to a person: "no extension"
// for a file like README.
func (g UncoveredGroup) ExtensionLabel() string {
	if g.Extension == "" {
		return "no extension"
	}
	return g.Extension
}

// String is one line for a person.
func (g UncoveredGroup) String() string {
	word := "files"
	if g.Count == 1 {
		word = "file"
	}
	return fmt.Sprintf("%-14s %3d %s  e.g. %s", g.ExtensionLabel(), g.Count, word, strings.Join(g.Examples, ", "))
}
