// Package check answers "is anything wrong with these rules?" for `rules check`.
//
// Two kinds of problem come out of it:
//
//   - **A mistake**: something the configuration may not say — a condition that
//     tests two things, a bad regex, a destination that climbs out. These come
//     from config.Problems, so this package cannot disagree with the loader.
//   - **A warning**: a rule that can never take a file, because an earlier rule
//     already takes every file it would. This is not an error — the
//     configuration is legal and the program runs — but the rule is dead weight
//     and the user almost never meant it.
//
// The analysis is *deliberately* narrow (docs/decisions.md, D11). It reports a
// shadowed rule only when it can prove the earlier rule takes every file the
// later one would. A checker that cries wolf is worse than no checker: the
// second time it is wrong, nobody reads it again. Every case it does not
// understand is silence, not a guess.
package check

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/08820048/owlsort-cli/internal/config"
)

// Severity says how much a problem matters.
type Severity string

const (
	// Error means the configuration is not valid and will not be loaded.
	Error Severity = "error"
	// Warning means the configuration is valid but probably not what was meant.
	Warning Severity = "warning"
)

// Problem is one finding.
type Problem struct {
	Severity Severity
	Rule     string // the rule it is about, or "" for the file's own
	Message  string
}

// Rules checks a configuration. It touches nothing: no disk, no clock.
func Rules(cfg config.Config) []Problem {
	var problems []Problem
	for _, p := range cfg.Problems() {
		problems = append(problems, Problem{Severity: Error, Rule: p.Rule, Message: p.Message})
	}
	problems = append(problems, shadowed(cfg)...)
	return problems
}

// HasErrors reports whether anything would stop the program from running.
func HasErrors(problems []Problem) bool {
	for _, p := range problems {
		if p.Severity == Error {
			return true
		}
	}
	return false
}

// shadowed finds rules no file can ever reach. Only an enabled rule can shadow;
// a rule that has been switched off takes nothing.
func shadowed(cfg config.Config) []Problem {
	var problems []Problem
	for j, later := range cfg.Rules {
		if !later.IsEnabled() || len(later.When) == 0 {
			continue
		}
		for i := 0; i < j; i++ {
			earlier := cfg.Rules[i]
			if !earlier.IsEnabled() || earlier.Continue || len(earlier.When) == 0 {
				continue
			}
			if covers(earlier, later) {
				problems = append(problems, Problem{
					Severity: Warning,
					Rule:     later.Name,
					Message: fmt.Sprintf(
						"%s can never take a file: %s above it takes every file it would. Move %s above %s, or set `continue = true` on %s.",
						ruleName(later, j), ruleName(earlier, i), plainName(later, j), plainName(earlier, i), plainName(earlier, i)),
				})
				break // one shadowing rule is enough to explain it
			}
		}
	}
	return problems
}

// ruleName is how a rule is called in a message. A rule with no name cannot pass
// validation, but the analysis still has to say something.
func ruleName(r config.Rule, index int) string {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Sprintf("rule %d", index+1)
	}
	return fmt.Sprintf("rule %q", r.Name)
}

// plainName is a rule's name with nothing around it, for the middle of a
// sentence.
func plainName(r config.Rule, index int) string {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Sprintf("rule %d", index+1)
	}
	return fmt.Sprintf("%q", r.Name)
}

// covers reports whether every file earlier would take, later would take too.
func covers(earlier, later config.Rule) bool {
	if later.Match == "any" {
		// The later rule needs only one of its conditions to hold, so the
		// earlier rule covers it when it guarantees one of them.
		for _, c := range later.When {
			if impliesCondition(earlier, c) {
				return true
			}
		}
		return false
	}
	// match = "all": the earlier rule has to guarantee every condition.
	for _, c := range later.When {
		if !impliesCondition(earlier, c) {
			return false
		}
	}
	return true
}

// impliesCondition reports whether a rule matching guarantees that one condition
// holds.
func impliesCondition(rule config.Rule, want config.Condition) bool {
	if rule.Match == "any" {
		// Any one of the rule's conditions may take a file, so the rule only
		// guarantees `want` when every one of them does.
		for _, has := range rule.When {
			if !implies(has, want) {
				return false
			}
		}
		return len(rule.When) > 0
	}
	// match = "all": one condition that guarantees `want` is enough.
	for _, has := range rule.When {
		if implies(has, want) {
			return true
		}
	}
	return false
}

// implies reports whether a file that satisfies has must satisfy want. It only
// answers yes when it is sure; an unrecognised pair is no.
func implies(has, want config.Condition) bool {
	if has.Not != want.Not {
		return false // "not pdf" says nothing about "pdf"
	}
	switch {
	case len(has.Ext) > 0 && len(want.Ext) > 0:
		return subsetFold(has.Ext, want.Ext)
	case has.Category != "" && want.Category != "":
		return has.Category == want.Category
	case len(has.NameEquals) > 0 && len(want.NameEquals) > 0:
		return subsetExact(has.NameEquals, want.NameEquals)
	case len(has.NameContains) > 0 && len(want.NameContains) > 0:
		return subsetFold(has.NameContains, want.NameContains)
	case len(has.NameStartsWith) > 0 && len(want.NameStartsWith) > 0:
		return subsetFold(has.NameStartsWith, want.NameStartsWith)
	case len(has.NameEndsWith) > 0 && len(want.NameEndsWith) > 0:
		return subsetFold(has.NameEndsWith, want.NameEndsWith)
	case len(has.NameRegex) > 0 && len(want.NameRegex) > 0:
		return sameSet(has.NameRegex, want.NameRegex)
	case has.LargerThan != "" && want.LargerThan != "":
		// "bigger than 10MB" also satisfies "bigger than 1MB".
		hasBytes, ok1 := bytesOf(has.LargerThan)
		wantBytes, ok2 := bytesOf(want.LargerThan)
		return ok1 && ok2 && hasBytes >= wantBytes
	case has.SmallerThan != "" && want.SmallerThan != "":
		hasBytes, ok1 := bytesOf(has.SmallerThan)
		wantBytes, ok2 := bytesOf(want.SmallerThan)
		return ok1 && ok2 && hasBytes <= wantBytes
	case has.ModifiedWithinDays != nil && want.ModifiedWithinDays != nil:
		// "within 7 days" also satisfies "within 30 days".
		return *has.ModifiedWithinDays <= *want.ModifiedWithinDays
	case has.ModifiedOlderThanDays != nil && want.ModifiedOlderThanDays != nil:
		return *has.ModifiedOlderThanDays >= *want.ModifiedOlderThanDays
	case has.CreatedWithinDays != nil && want.CreatedWithinDays != nil:
		return *has.CreatedWithinDays <= *want.CreatedWithinDays
	case has.CreatedOlderThanDays != nil && want.CreatedOlderThanDays != nil:
		return *has.CreatedOlderThanDays >= *want.CreatedOlderThanDays
	case has.InFolder != "" && want.InFolder != "":
		return filepath.Clean(has.InFolder) == filepath.Clean(want.InFolder)
	case len(has.PathContains) > 0 && len(want.PathContains) > 0:
		return subsetExact(has.PathContains, want.PathContains)
	}
	return false
}

// subsetFold reports whether every entry of a appears in b, ignoring case.
func subsetFold(a, b []string) bool {
	if len(a) > len(b) || len(a) == 0 {
		return false // a wider list matches more, so it cannot narrow b
	}
	set := make(map[string]bool, len(b))
	for _, s := range b {
		set[strings.ToLower(s)] = true
	}
	for _, s := range a {
		if !set[strings.ToLower(s)] {
			return false
		}
	}
	return true
}

// subsetExact is subsetFold without the case folding, for values where case is
// part of the meaning (an exact name, a path fragment).
func subsetExact(a, b []string) bool {
	if len(a) > len(b) || len(a) == 0 {
		return false
	}
	set := make(map[string]bool, len(b))
	for _, s := range b {
		set[s] = true
	}
	for _, s := range a {
		if !set[s] {
			return false
		}
	}
	return true
}

// sameSet reports whether two lists hold the same strings. A regex says what it
// says: this analysis does not try to reason about one expression containing
// another, and it will not guess.
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	return subsetExact(a, b)
}

// bytesOf turns "10MB" into bytes, or 0 and false when it is not a size.
func bytesOf(spec string) (int64, bool) {
	upper := strings.ToUpper(strings.TrimSpace(spec))
	digits := upper
	unit := ""
	for _, suffix := range []string{"TB", "GB", "MB", "KB", "B"} {
		if strings.HasSuffix(upper, suffix) {
			digits = strings.TrimSpace(strings.TrimSuffix(upper, suffix))
			unit = suffix
			break
		}
	}
	if unit == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(digits, 64)
	if err != nil {
		return 0, false
	}
	units := map[string]float64{"B": 1, "KB": 1 << 10, "MB": 1 << 20, "GB": 1 << 30, "TB": 1 << 40}
	return int64(value * units[unit]), true
}
