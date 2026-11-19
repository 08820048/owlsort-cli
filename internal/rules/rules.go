// Package rules turns files and a configuration into a plan.
//
// Contract (docs/architecture.md, "The engine"):
//   - Rules are tried in order; the first enabled rule that matches wins,
//     unless it says `continue`.
//   - A file that is protected (internal/protect) or kept is never planned;
//     it is reported as skipped with the reason.
//   - A file already inside the fixed part of a rule's destination
//     (pathvars.FixedPrefix) is settled: not planned again.
//   - A destination is expanded with pathvars.Expand per file, and must stay
//     inside the watched folder unless it is absolute (absolute destinations
//     are allowed by hand, never unattended).
//   - Pure: no disk access. Same inputs, same plan.
//
// The clock and the time zone come in through Env, so a test can pin them and
// get the same plan every time.
package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/08820048/owlsort-cli/internal/category"
	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/i18n"
	"github.com/08820048/owlsort-cli/internal/pathvars"
	"github.com/08820048/owlsort-cli/internal/plan"
	"github.com/08820048/owlsort-cli/internal/protect"
	"github.com/08820048/owlsort-cli/internal/scan"
)

// Env is everything the engine needs from outside itself. It is passed in so
// that deciding what to do stays pure: the same files, config and Env give the
// same plan.
type Env struct {
	// Lang names the folders a {category} expands to.
	Lang i18n.Language
	// Location is the time zone dates are written in.
	Location *time.Location
	// Now is the clock the "modified within N days" conditions are measured
	// against. A caller that leaves it zero gets time.Now, which only tests do.
	Now time.Time
	// Unattended is set by `watch`: absolute destinations and trash rules
	// without `background = true` may not run then (safety rules 9 and
	// docs/config.md).
	Unattended bool
}

func (e Env) withDefaults() Env {
	if e.Lang == "" {
		e.Lang = i18n.English
	}
	if e.Location == nil {
		e.Location = time.Local
	}
	if e.Now.IsZero() {
		e.Now = time.Now()
	}
	return e
}

// state is what happened to one file.
type state int

const (
	// stateNone: nothing to plan. Either no rule covers the file, or it is
	// already filed where its rule would put it.
	stateNone state = iota
	statePlanned
	// stateNeedsForeground: a rule matched, but may not do this unattended.
	stateNeedsForeground
)

// Build decides what happens to each file.
func Build(files []scan.File, cfg config.Config, guard protect.Guard, env Env) (plan.Plan, error) {
	if err := cfg.Validate(); err != nil {
		// A bad configuration stops everything: the program never falls back to
		// the built-in rules (docs/safety-model.md, rule 12).
		return plan.Plan{}, err
	}
	env = env.withDefaults()

	p := plan.Plan{Version: plan.Version, Folders: []plan.Folder{}}
	index := make(map[string]int, len(files))
	for _, f := range files {
		i, ok := index[f.Folder]
		if !ok {
			i = len(p.Folders)
			index[f.Folder] = i
			p.Folders = append(p.Folders, plan.Folder{Path: f.Folder})
		}

		if reason := skipReason(f, guard); reason != "" {
			p.Folders[i].Skipped = append(p.Folders[i].Skipped, plan.Skipped{Path: f.Path, Reason: reason})
			continue
		}

		item, st, err := decide(f, cfg, env)
		if err != nil {
			return plan.Plan{}, err
		}
		switch st {
		case statePlanned:
			p.Folders[i].Items = append(p.Folders[i].Items, item)
		case stateNeedsForeground:
			p.Folders[i].Skipped = append(p.Folders[i].Skipped, plan.Skipped{
				Path: f.Path, Reason: plan.SkipNeedsForeground, Rule: item.Rule,
			})
		}
	}
	return p, nil
}

// skipReason is what stops a file before any rule is looked at. Protection
// comes first: it is about where the file is, and it outranks what the file is.
func skipReason(f scan.File, guard protect.Guard) plan.SkipReason {
	if guard != nil {
		if reason := guard.Protected(f.Path); reason != "" {
			return reason
		}
	}
	// A dot file is listed, so the plan can say why it was not planned. This
	// comes before "downloading": a .DS_Store that was just written is hidden,
	// and "hidden" is the reason a person can act on.
	if f.Hidden {
		return plan.SkipHidden
	}
	// A file still being written is left alone (rule 13). A cloud placeholder
	// counts as downloading: its contents are not here yet.
	if f.Downloading {
		return plan.SkipDownloading
	}
	return ""
}

// decide runs the rules over one file.
func decide(f scan.File, cfg config.Config, env Env) (plan.Item, state, error) {
	var (
		chosen plan.Item
		got    state
		have   bool
	)
	for _, r := range cfg.Rules {
		if !r.IsEnabled() || !match(f, r, env) {
			continue
		}
		item, st, err := perform(f, r, env)
		if err != nil {
			return plan.Item{}, stateNone, err
		}
		if st == stateNone && item.Destination != "" {
			// Already filed: it is where this rule would put it.
			return item, stateNone, nil
		}
		if st == stateNone && item.Rule == "" {
			// The rule does not apply to this file after all (it is not at the
			// top level, and the rule did not ask for subfolders).
			continue
		}
		chosen, got, have = item, st, true
		if st == stateNeedsForeground {
			// A rule that may not run unattended leaves the file where it is;
			// a later rule still may take it.
			continue
		}
		if !r.Continue {
			break
		}
	}
	if !have {
		return plan.Item{}, stateNone, nil
	}
	return chosen, got, nil
}

// perform turns one matching rule into one step, and says whether the file is
// already where the rule would put it.
func perform(f scan.File, r config.Rule, env Env) (plan.Item, state, error) {
	if !r.Subfolders && filepath.Dir(f.Path) != filepath.Clean(f.Folder) {
		// Only the top level of a watched folder is tidied unless a rule asks
		// for more: subfolders are the user's own finished work (rule 8).
		return plan.Item{}, stateNone, nil
	}

	item := plan.Item{
		Source: f.Path, Rule: r.Name, Size: f.Size, Modified: f.Modified,
		Conflict: string(r.Conflict),
	}
	switch {
	case r.Trash:
		item.Action = plan.Trash
		if env.Unattended && !r.Background {
			return item, stateNeedsForeground, nil
		}
		return item, statePlanned, nil

	case r.Rename != "":
		name := strings.TrimSpace(pathvars.Expand(r.Rename, vars(f), env.Lang, env.Location))
		if name == "" || name == "." || name == ".." {
			return plan.Item{}, stateNone, fmt.Errorf("rule %q: rename template %q gives an empty name for %s", r.Name, r.Rename, f.Path)
		}
		if strings.ContainsAny(name, `/\`) {
			return plan.Item{}, stateNone, fmt.Errorf("rule %q: a rename template must give a name, not a path (%q)", r.Name, r.Rename)
		}
		item.Action = plan.Rename
		item.Destination = filepath.Join(filepath.Dir(f.Path), name)
		if samePath(item.Destination, f.Path) {
			return item, stateNone, nil
		}
		return item, statePlanned, nil

	case r.MoveTo != "" || r.CopyTo != "":
		template := r.MoveTo
		item.Action = plan.Move
		if r.CopyTo != "" {
			template = r.CopyTo
			item.Action = plan.Copy
		}
		destination, st, err := destination(f, template, env)
		if err != nil || st != statePlanned {
			return plan.Item{Destination: destination, Rule: r.Name, Size: f.Size, Modified: f.Modified}, st, err
		}
		item.Destination = destination
		return item, statePlanned, nil
	}
	return plan.Item{}, stateNone, fmt.Errorf("rule %q has no action", r.Name)
}

// destination expands a move/copy destination for one file and checks it stays
// where it may. It reports stateNone when the file is already filed there.
func destination(f scan.File, template string, env Env) (string, state, error) {
	expanded := pathvars.Expand(template, vars(f), env.Lang, env.Location)
	if hasParentStep(expanded) {
		// config rejects this too; planning checks again because a variable's
		// value could in principle change between the two (rule 14).
		return "", stateNone, fmt.Errorf("rule destination %q climbs out of the watched folder", template)
	}
	if strings.TrimSpace(expanded) == "" {
		return "", stateNone, fmt.Errorf("rule destination %q is empty", template)
	}

	folder := filepath.Clean(f.Folder)

	// A file already inside the fixed part of this destination was filed by
	// this rule before: it is settled, not planned again.
	if prefix := pathvars.FixedPrefix(template); prefix != "" {
		base := config.ExpandHome(prefix)
		if !filepath.IsAbs(base) {
			base = filepath.Join(folder, base)
		}
		if inside(f.Path, base) {
			return f.Path, stateNone, nil
		}
	}

	var target string
	if filepath.IsAbs(expanded) {
		if env.Unattended {
			// An absolute destination is allowed when run by hand and never
			// unattended (docs/config.md).
			return "", stateNeedsForeground, nil
		}
		target = filepath.Join(filepath.Clean(expanded), f.Name)
	} else {
		base := filepath.Join(folder, expanded)
		rel, err := filepath.Rel(folder, base)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", stateNone, fmt.Errorf("rule destination %q climbs out of %s", template, folder)
		}
		if samePath(base, folder) {
			return "", stateNone, fmt.Errorf("rule destination %q is the watched folder itself", template)
		}
		target = filepath.Join(base, f.Name)
	}

	if samePath(target, f.Path) {
		return target, stateNone, nil
	}
	return target, statePlanned, nil
}

// match reports whether a file satisfies a rule's conditions.
func match(f scan.File, r config.Rule, env Env) bool {
	if len(r.When) == 0 {
		return false
	}
	if r.Match == "any" {
		for _, c := range r.When {
			if matchOne(f, c, env) {
				return true
			}
		}
		return false
	}
	for _, c := range r.When {
		if !matchOne(f, c, env) {
			return false
		}
	}
	return true
}

// matchOne tests one condition, then inverts it if it says `not`.
func matchOne(f scan.File, c config.Condition, env Env) bool {
	var ok bool
	switch {
	case len(c.Ext) > 0:
		ok = containsFold(c.Ext, f.Extension)
	case c.Category != "":
		ok = category.ForExtension(f.Extension) == category.Category(c.Category)
	case len(c.NameEquals) > 0:
		ok = containsExact(c.NameEquals, f.Name)
	case len(c.NameContains) > 0:
		ok = containsFold(c.NameContains, f.Name)
	case len(c.NameStartsWith) > 0:
		ok = anyFold(c.NameStartsWith, func(s string) bool {
			return strings.HasPrefix(strings.ToLower(f.Name), strings.ToLower(s))
		})
	case len(c.NameEndsWith) > 0:
		ok = anyFold(c.NameEndsWith, func(s string) bool {
			return strings.HasSuffix(strings.ToLower(f.Name), strings.ToLower(s))
		})
	case len(c.NameRegex) > 0:
		ok = anyRegex(c.NameRegex, f.Name)
	case c.LargerThan != "":
		// A package's size is unknown, so a size condition never matches one.
		ok = !f.Package && sizeAtLeast(f.Size, c.LargerThan, false)
	case c.SmallerThan != "":
		ok = !f.Package && sizeAtLeast(f.Size, c.SmallerThan, true)
	case c.ModifiedWithinDays != nil:
		ok = withinDays(env.Now, f.Modified, *c.ModifiedWithinDays)
	case c.ModifiedOlderThanDays != nil:
		ok = !withinDays(env.Now, f.Modified, *c.ModifiedOlderThanDays)
	case c.CreatedWithinDays != nil:
		// A file whose birth time the OS does not keep cannot be said to be
		// recent: the condition does not hold.
		ok = !f.Created.IsZero() && withinDays(env.Now, f.Created, *c.CreatedWithinDays)
	case c.CreatedOlderThanDays != nil:
		ok = !f.Created.IsZero() && !withinDays(env.Now, f.Created, *c.CreatedOlderThanDays)
	case c.InFolder != "":
		ok = samePath(filepath.Dir(f.Path), config.ExpandHome(c.InFolder))
	case len(c.PathContains) > 0:
		for _, s := range c.PathContains {
			if strings.Contains(f.Path, s) {
				ok = true
				break
			}
		}
	}
	if c.Not {
		return !ok
	}
	return ok
}

// vars is what pathvars needs to know about a file.
func vars(f scan.File) pathvars.File {
	return pathvars.File{Extension: f.Extension, Arrived: f.Arrived()}
}

// withinDays reports whether t is no more than days ago.
func withinDays(now, t time.Time, days int) bool {
	if t.IsZero() {
		return false
	}
	return now.Sub(t) <= time.Duration(days)*24*time.Hour
}

var sizePattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(B|KB|MB|GB|TB)$`)

// sizeAtLeast compares a size with a spec like "10MB". smaller asks whether the
// file is the smaller one.
func sizeAtLeast(size int64, spec string, smaller bool) bool {
	limit, ok := bytesOf(spec)
	if !ok {
		return false
	}
	if smaller {
		return size < limit
	}
	return size > limit
}

// bytesOf turns "10MB" into bytes. Sizes are powers of 1024, as the file
// managers of all three systems show them.
func bytesOf(spec string) (int64, bool) {
	m := sizePattern.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(spec)))
	if m == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	units := map[string]float64{"B": 1, "KB": 1 << 10, "MB": 1 << 20, "GB": 1 << 30, "TB": 1 << 40}
	return int64(value * units[m[2]]), true
}

// containsFold, containsExact and anyFold are the name comparisons.
func containsFold(list []string, value string) bool {
	lower := strings.ToLower(value)
	for _, s := range list {
		if strings.Contains(lower, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

func containsExact(list []string, value string) bool {
	for _, s := range list {
		if s == value {
			return true
		}
	}
	return false
}

func anyFold(list []string, test func(string) bool) bool {
	for _, s := range list {
		if test(s) {
			return true
		}
	}
	return false
}

func anyRegex(expressions []string, value string) bool {
	for _, expr := range expressions {
		re, err := regexp.Compile(expr)
		if err != nil {
			// config rejects a bad expression; a plan never guesses at one.
			continue
		}
		if re.MatchString(value) {
			return true
		}
	}
	return false
}

// inside reports whether path is dir itself or below it.
func inside(path, dir string) bool {
	path, dir = filepath.Clean(path), filepath.Clean(dir)
	return path == dir || strings.HasPrefix(path, dir+string(filepath.Separator))
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

// hasParentStep reports whether a relative destination climbs out with "..".
func hasParentStep(dest string) bool {
	for _, part := range strings.FieldsFunc(dest, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

// Explained is the answer `owlsort explain` gives about one file: what would
// happen to it, and why. It is a value rather than a sentence so the text and
// `--json` cannot say different things; the words live in internal/output, which
// is where every other thing the program says to a person lives.
type Explained struct {
	Path        string      `json:"path"`
	Planned     bool        `json:"planned"`
	Rule        string      `json:"rule,omitempty"`
	Action      plan.Action `json:"action,omitempty"`
	Destination string      `json:"destination,omitempty"`
	// Skip is why the file is left alone, when something decided that before any
	// rule was tried (protection, kept, hidden, still downloading).
	Skip plan.SkipReason `json:"skip,omitempty"`
	// Filed means a rule covers the file and the file is already where that rule
	// would put it, so there is nothing to do.
	Filed bool `json:"filed,omitempty"`
	// NeedsForeground means a rule matched, but may not do this unattended.
	NeedsForeground bool `json:"needs_foreground,omitempty"`
}

// Explain says why a file is, or is not, going to be tidied. It runs the same
// skipReason and decide code as Build, so it cannot disagree with `preview`.
func Explain(f scan.File, cfg config.Config, guard protect.Guard, env Env) (Explained, error) {
	out := Explained{Path: f.Path}
	if err := cfg.Validate(); err != nil {
		return Explained{}, err
	}
	env = env.withDefaults()

	if reason := skipReason(f, guard); reason != "" {
		out.Skip = reason
		return out, nil
	}

	item, st, err := decide(f, cfg, env)
	if err != nil {
		return Explained{}, err
	}
	switch st {
	case stateNeedsForeground:
		out.Rule, out.NeedsForeground = item.Rule, true
	case stateNone:
		if item.Rule != "" {
			out.Rule, out.Filed = item.Rule, true
		}
	case statePlanned:
		out.Planned = true
		out.Rule, out.Action, out.Destination = item.Rule, item.Action, item.Destination
	}
	return out, nil
}
