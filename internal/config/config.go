// Package config is the configuration file: its model, parsing, validation
// and the built-in rules written by `owlsort init`. The file format is
// documented in docs/config.md; this package is its source of truth.
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/08820048/owlsort-cli/internal/category"
)

// Conflict says what to do when the destination already has a file of that name.
type Conflict string

const (
	ConflictRename Conflict = "rename" // keep both: "name 2.ext" (the default)
	ConflictSkip   Conflict = "skip"   // leave the file where it is
)

// Config is the whole file.
type Config struct {
	// Folders are the watched folders; only their top level is tidied unless a
	// rule says `subfolders = true`.
	Folders []string `toml:"folders"`
	// Keep lists files and folders that no rule ever touches.
	Keep []string `toml:"keep"`
	// Conflict is the default for every rule.
	Conflict Conflict `toml:"conflict"`
	// Language is "", "en" or "zh-Hans"; empty follows the environment.
	Language string `toml:"language"`
	// Rules are tried in order; the first match wins unless it says `continue`.
	Rules []Rule `toml:"rule"`
}

// Rule is conditions plus one action.
type Rule struct {
	Name    string      `toml:"name"`
	Enabled *bool       `toml:"enabled"` // nil means true
	Match   string      `toml:"match"`   // "all" (default) or "any"
	When    []Condition `toml:"when"`

	// Exactly one of the four actions.
	MoveTo string `toml:"move_to"`
	CopyTo string `toml:"copy_to"`
	Rename string `toml:"rename"` // a template; the folder stays
	Trash  bool   `toml:"trash"`  // to the system trash, never a permanent delete

	Subfolders bool `toml:"subfolders"` // also tidy files below the top level
	Background bool `toml:"background"` // may run unattended in `watch` (trash needs this)
	Continue   bool `toml:"continue"`   // go on matching later rules after this one
}

// IsEnabled reports whether the rule is on.
func (r Rule) IsEnabled() bool { return r.Enabled == nil || *r.Enabled }

// Condition is one test on a file. Exactly one field besides Not is set.
type Condition struct {
	Not bool `toml:"not"`

	Ext      []string `toml:"ext"`      // extensions, without dots
	Category string   `toml:"category"` // see package category

	NameEquals     []string `toml:"name_equals"`
	NameContains   []string `toml:"name_contains"`
	NameStartsWith []string `toml:"name_starts_with"`
	NameEndsWith   []string `toml:"name_ends_with"`
	NameRegex      []string `toml:"name_regex"`

	LargerThan  string `toml:"larger_than"`  // "10MB"
	SmallerThan string `toml:"smaller_than"` // "500KB"

	ModifiedWithinDays    *int `toml:"modified_within_days"`
	ModifiedOlderThanDays *int `toml:"modified_older_than_days"`
	CreatedWithinDays     *int `toml:"created_within_days"`
	CreatedOlderThanDays  *int `toml:"created_older_than_days"`

	InFolder     string   `toml:"in_folder"`     // the file is directly in this folder
	PathContains []string `toml:"path_contains"` // the full path contains one of these
}

// Parse reads and validates a configuration.
func Parse(data []byte) (Config, error) {
	c, err := ParseLoose(data)
	if err != nil {
		return Config{}, err
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// ParseLoose reads a configuration without checking what it says. It still
// refuses what cannot be read at all (bad TOML, an unknown key), because there is
// no configuration to report on then.
//
// It exists for `rules check` and `doctor`, which have to list every problem a
// configuration has. Loading it the strict way would stop at the first mistake
// and hide the rest, which is the one thing those commands must not do.
func ParseLoose(data []byte) (Config, error) {
	var c Config
	meta, err := toml.Decode(string(data), &c)
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if unknown := meta.Undecoded(); len(unknown) > 0 {
		keys := make([]string, len(unknown))
		for i, k := range unknown {
			keys[i] = k.String()
		}
		return Config{}, fmt.Errorf("config: unknown keys: %s", strings.Join(keys, ", "))
	}
	if c.Conflict == "" {
		c.Conflict = ConflictRename
	}
	return c, nil
}

// Load reads a configuration file. A file that cannot be read or is invalid is
// an error: the program never falls back to the built-in rules to tidy with,
// because that could move files the user's own rules would have left alone.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return Parse(data)
}

// Marshal writes a configuration as TOML.
func Marshal(c Config) ([]byte, error) {
	var b strings.Builder
	if err := toml.NewEncoder(&b).Encode(c); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

var sizePattern = regexp.MustCompile(`^\d+(\.\d+)?\s*(B|KB|MB|GB|TB)$`)

// Problem is one thing wrong with a configuration.
type Problem struct {
	// Rule is the name of the rule it belongs to, or "" when the problem is
	// the file's own. A rule with no name is reported as "rule 3".
	Rule    string
	Message string // one sentence, ready to print
}

// Validate checks a configuration and reports every problem it finds.
func (c Config) Validate() error {
	problems := c.Problems()
	if len(problems) == 0 {
		return nil
	}
	messages := make([]string, len(problems))
	for i, p := range problems {
		messages[i] = p.Message
	}
	return errors.New("config: " + strings.Join(messages, "; "))
}

// Problems checks a configuration and returns every problem, one at a time, so
// `rules check` can say which rule each one belongs to instead of one long line.
func (c Config) Problems() []Problem {
	var problems []Problem
	add := func(rule, format string, args ...any) {
		problems = append(problems, Problem{Rule: rule, Message: fmt.Sprintf(format, args...)})
	}

	switch c.Conflict {
	case ConflictRename, ConflictSkip:
	default:
		add("", "conflict must be %q or %q, not %q", ConflictRename, ConflictSkip, c.Conflict)
	}

	seen := map[string]bool{}
	for i, r := range c.Rules {
		label := fmt.Sprintf("rule %d", i+1)
		if r.Name != "" {
			label = fmt.Sprintf("rule %q", r.Name)
		}
		if strings.TrimSpace(r.Name) == "" {
			add(label, "%s has no name", label)
		} else if seen[r.Name] {
			add(r.Name, "%s: the name is used twice", label)
		}
		seen[r.Name] = true

		if r.Match != "" && r.Match != "all" && r.Match != "any" {
			add(r.Name, "%s: match must be \"all\" or \"any\", not %q", label, r.Match)
		}
		if len(r.When) == 0 {
			add(r.Name, "%s has no conditions (when)", label)
		}
		for j, cond := range r.When {
			if err := cond.validate(); err != nil {
				add(r.Name, "%s, condition %d: %v", label, j+1, err)
			}
		}

		actions := 0
		for _, set := range []bool{r.MoveTo != "", r.CopyTo != "", r.Rename != "", r.Trash} {
			if set {
				actions++
			}
		}
		if actions != 1 {
			add(r.Name, "%s must have exactly one of move_to, copy_to, rename, trash (it has %d)", label, actions)
		}
		for _, dest := range []string{r.MoveTo, r.CopyTo} {
			if hasParentStep(dest) {
				add(r.Name, "%s: %q climbs out of the watched folder with \"..\"", label, dest)
			}
		}
	}
	return problems
}

func (c Condition) validate() error {
	set := 0
	count := func(b bool) {
		if b {
			set++
		}
	}
	count(len(c.Ext) > 0)
	count(c.Category != "")
	count(len(c.NameEquals) > 0)
	count(len(c.NameContains) > 0)
	count(len(c.NameStartsWith) > 0)
	count(len(c.NameEndsWith) > 0)
	count(len(c.NameRegex) > 0)
	count(c.LargerThan != "")
	count(c.SmallerThan != "")
	count(c.ModifiedWithinDays != nil)
	count(c.ModifiedOlderThanDays != nil)
	count(c.CreatedWithinDays != nil)
	count(c.CreatedOlderThanDays != nil)
	count(c.InFolder != "")
	count(len(c.PathContains) > 0)
	if set != 1 {
		return fmt.Errorf("a condition must test exactly one thing (it tests %d)", set)
	}

	if c.Category != "" && !category.Valid(c.Category) {
		return fmt.Errorf("unknown category %q", c.Category)
	}
	for _, expr := range c.NameRegex {
		if _, err := regexp.Compile(expr); err != nil {
			return fmt.Errorf("name_regex %q: %v", expr, err)
		}
	}
	for _, size := range []string{c.LargerThan, c.SmallerThan} {
		if size != "" && !sizePattern.MatchString(strings.ToUpper(size)) {
			return fmt.Errorf("size %q is not like \"10MB\"", size)
		}
	}
	for _, days := range []*int{c.ModifiedWithinDays, c.ModifiedOlderThanDays, c.CreatedWithinDays, c.CreatedOlderThanDays} {
		if days != nil && *days < 0 {
			return fmt.Errorf("a number of days cannot be negative")
		}
	}
	return nil
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
