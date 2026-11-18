package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/08820048/owlsort-cli/internal/config"
)

// Rules prints the rules of a configuration, one paragraph each, so a person can
// see what is on and what it does without opening the file.
func Rules(w io.Writer, cfg config.Config, path string, opts Options) error {
	color := !opts.NoColor && isTerminal(w)
	bold := func(s string) string { return paint(color, "1", s) }
	dim := func(s string) string { return paint(color, "2", s) }

	on := 0
	for _, r := range cfg.Rules {
		if r.IsEnabled() {
			on++
		}
	}
	fmt.Fprintf(w, "%s · %d of %d on · %s\n\n", bold("Rules"), on, len(cfg.Rules), displayFolder(path))

	if len(cfg.Rules) == 0 {
		fmt.Fprintln(w, "No rules. `owlsort init` writes the built-in set.")
		return nil
	}

	width := 0
	for _, r := range cfg.Rules {
		if n := runeLen(r.Name); n > width {
			width = n
		}
	}
	for _, r := range cfg.Rules {
		mark := "on "
		if !r.IsEnabled() {
			mark = "off"
		}
		pad := strings.Repeat(" ", width-runeLen(r.Name))
		fmt.Fprintf(w, "  %s  %s%s  %s\n", mark, r.Name, pad, dim(ActionText(r)))
		for _, line := range conditionLines(r) {
			fmt.Fprintf(w, "       %s\n", dim(line))
		}
	}
	fmt.Fprintln(w, "\nTurn one off with `owlsort rules disable <name>`; see one with `owlsort rules show <name>`.")
	return nil
}

// Rule prints one rule in full.
func Rule(w io.Writer, r config.Rule, path string, opts Options) error {
	color := !opts.NoColor && isTerminal(w)
	bold := func(s string) string { return paint(color, "1", s) }

	state := "on"
	if !r.IsEnabled() {
		state = "off"
	}
	fmt.Fprintf(w, "%s · %s\n\n", bold(r.Name), state)
	fmt.Fprintf(w, "  does      %s\n", ActionText(r))
	match := r.Match
	if match == "" {
		match = "all"
	}
	fmt.Fprintf(w, "  conditions  %s of these:\n", match)
	for _, line := range conditionParts(r) {
		fmt.Fprintf(w, "              %s\n", line)
	}
	flags := flagsOf(r)
	if len(flags) == 0 {
		flags = []string{"none"}
	}
	fmt.Fprintf(w, "  flags     %s\n", strings.Join(flags, ", "))
	fmt.Fprintf(w, "  in        %s\n", displayFolder(path))
	return nil
}

// Strings prints a list such as folders or keep, with a note when it is empty.
func Strings(w io.Writer, title string, path string, values []string, empty string, opts Options) error {
	color := !opts.NoColor && isTerminal(w)
	bold := func(s string) string { return paint(color, "1", s) }

	fmt.Fprintf(w, "%s · %d · %s\n", bold(title), len(values), displayFolder(path))
	if len(values) == 0 {
		fmt.Fprintf(w, "  %s\n", empty)
		return nil
	}
	for _, v := range values {
		fmt.Fprintf(w, "  %s\n", v)
	}
	return nil
}

// ActionOf is what a rule does, as the word for it ("move", "copy", "rename",
// "trash") and the destination it names. `rules list` prints it and `rules list
// --json` reports it, so the two cannot say different things.
func ActionOf(r config.Rule) (action, to string) {
	switch {
	case r.MoveTo != "":
		return "move", r.MoveTo
	case r.CopyTo != "":
		return "copy", r.CopyTo
	case r.Rename != "":
		return "rename", r.Rename
	case r.Trash:
		return "trash", ""
	}
	return "none", ""
}

// ActionText is ActionOf as one phrase for a line of output.
func ActionText(r config.Rule) string {
	action, to := ActionOf(r)
	switch action {
	case "trash":
		return "send to the trash"
	case "move":
		return "move to " + to
	case "copy":
		return "copy to " + to
	case "rename":
		return "rename to " + to
	}
	return "do nothing"
}

// conditionLines says what a rule looks at, one condition per line.
func conditionLines(r config.Rule) []string {
	parts := conditionParts(r)
	if len(parts) == 0 {
		return []string{"no conditions"}
	}
	prefix := "when"
	if r.Match == "any" {
		prefix = "when any of"
	}
	out := make([]string, 0, len(parts)+1)
	out = append(out, prefix+": "+parts[0])
	for _, p := range parts[1:] {
		out = append(out, strings.Repeat(" ", len(prefix)+2)+p)
	}
	return out
}

// conditionParts is each condition as one line of text.
func conditionParts(r config.Rule) []string {
	out := make([]string, 0, len(r.When))
	for _, c := range r.When {
		out = append(out, ConditionText(c))
	}
	return out
}

// ConditionText says one condition the way the configuration file spells it.
func ConditionText(c config.Condition) string {
	var text string
	switch {
	case len(c.Ext) > 0:
		text = "ext is " + list(c.Ext)
	case c.Category != "":
		text = "category is " + c.Category
	case len(c.NameEquals) > 0:
		text = "name is " + list(c.NameEquals)
	case len(c.NameContains) > 0:
		text = "name contains " + list(c.NameContains)
	case len(c.NameStartsWith) > 0:
		text = "name starts with " + list(c.NameStartsWith)
	case len(c.NameEndsWith) > 0:
		text = "name ends with " + list(c.NameEndsWith)
	case len(c.NameRegex) > 0:
		text = "name matches " + list(c.NameRegex)
	case c.LargerThan != "":
		text = "bigger than " + c.LargerThan
	case c.SmallerThan != "":
		text = "smaller than " + c.SmallerThan
	case c.ModifiedWithinDays != nil:
		text = fmt.Sprintf("changed in the last %d days", *c.ModifiedWithinDays)
	case c.ModifiedOlderThanDays != nil:
		text = fmt.Sprintf("unchanged for %d days", *c.ModifiedOlderThanDays)
	case c.CreatedWithinDays != nil:
		text = fmt.Sprintf("created in the last %d days", *c.CreatedWithinDays)
	case c.CreatedOlderThanDays != nil:
		text = fmt.Sprintf("created more than %d days ago", *c.CreatedOlderThanDays)
	case c.InFolder != "":
		text = "in folder " + displayFolder(c.InFolder)
	case len(c.PathContains) > 0:
		text = "path contains " + list(c.PathContains)
	default:
		text = "nothing"
	}
	if c.Not {
		if c.InFolder != "" {
			return "not in folder " + displayFolder(c.InFolder)
		}
		return "not (" + text + ")"
	}
	return text
}

// list joins values the way the file writes them: quoted, comma separated.
func list(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = `"` + v + `"`
	}
	if len(quoted) > 6 {
		return strings.Join(quoted[:6], ", ") + fmt.Sprintf(" and %d more", len(quoted)-6)
	}
	return strings.Join(quoted, ", ")
}

// flagsOf says what the switches on a rule are.
func flagsOf(r config.Rule) []string {
	var out []string
	if r.Continue {
		out = append(out, "continue")
	}
	if r.Subfolders {
		out = append(out, "subfolders")
	}
	if r.Background {
		out = append(out, "background")
	}
	return out
}
