package config

import (
	"fmt"
	"strings"

	"github.com/08820048/owlsort-cli/internal/i18n"
)

// Template is what `owlsort init` writes: the built-in rules, laid out and
// commented the way a person would write them.
//
// config.Marshal could write the same rules, but it writes every field in a
// machine's order, with the conditions as separate [[rule.when]] tables and not a
// comment in sight. This file is the first thing anyone sees, and the argument for
// TOML over YAML is partly that it stays pleasant to read and edit by hand.
//
// It is generated from Default, not typed out, so the two cannot drift: a test
// parses the template and compares it with Default.
func Template(lang i18n.Language, folders []string) []byte {
	cfg := Default(lang)
	if len(folders) > 0 {
		cfg.Folders = folders
	}

	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	w("# Owlsort — tidy the loose files in your Downloads and Desktop.")
	w("#")
	w("#   owlsort preview    what would be tidied, and where. Moves nothing.")
	w("#   owlsort tidy       the same, then it asks before moving anything.")
	w("#   owlsort undo       put the last run back.")
	w("#")
	w("# Rules are tried in order: the first one whose conditions hold takes the")
	w("# file. `owlsort rules list` shows them, `owlsort rules check` finds")
	w("# mistakes, `owlsort rules disable <name>` switches one off.")
	w("")
	w("folders = %s", array(cfg.Folders))
	w("keep = %s%s", array(cfg.Keep), comment("files and folders never touched"))
	w(`conflict = "rename"%s`, comment(`"rename" keeps both, "skip" leaves it`))
	w(`language = ""%s`, comment(`"", "en" or "zh-Hans"; empty follows LANG`))
	w("")
	w("# --- rules ---------------------------------------------------------------")
	w("#")
	w("# Conditions: ext, category, name_equals, name_contains, name_starts_with,")
	w("# name_ends_with, name_regex, larger_than, smaller_than, modified_within_days,")
	w("# modified_older_than_days, created_within_days, created_older_than_days,")
	w("# in_folder, path_contains. Each tests one thing; `not = true` inverts it.")
	w("#")
	w("# Exactly one action: move_to, copy_to, rename or trash. A destination may")
	w("# use {date} {year} {month} {day} {category} {ext}.")
	w("#")
	w("#   enabled = false    switch a rule off without deleting it")
	w("#   match = \"any\"      one condition is enough (default: all of them)")
	w("#   continue = true    let later rules have a go as well")
	w("#   subfolders = true  tidy below the top level too")
	w("#   background = true  may run unattended; `trash` needs this")

	for _, r := range cfg.Rules {
		w("")
		w("[[rule]]")
		w("name = %s", quote(r.Name))
		if r.Enabled != nil {
			w("enabled = %t", *r.Enabled)
		}
		if r.Match != "" {
			w("match = %s", quote(r.Match))
		}
		if r.Subfolders {
			w("subfolders = true")
		}
		if r.Background {
			w("background = true")
		}
		if r.Continue {
			w("continue = true")
		}
		w("when = %s", conditions(r.When))
		switch {
		case r.MoveTo != "":
			w("move_to = %s", quote(r.MoveTo))
		case r.CopyTo != "":
			w("copy_to = %s", quote(r.CopyTo))
		case r.Rename != "":
			w("rename = %s", quote(r.Rename))
		case r.Trash:
			w("trash = true")
		}
	}
	return []byte(b.String())
}

// conditions renders a rule's conditions: one per line when there are several,
// which is how a person writes a list they may want to add to.
func conditions(list []Condition) string {
	if len(list) == 1 {
		return "[" + condition(list[0]) + "]"
	}
	var b strings.Builder
	b.WriteString("[\n")
	for _, c := range list {
		fmt.Fprintf(&b, "  %s,\n", condition(c))
	}
	b.WriteString("]")
	return b.String()
}

// condition renders one condition as an inline table.
func condition(c Condition) string {
	var parts []string
	set := func(key, value string) { parts = append(parts, key+" = "+value) }
	if c.Not {
		set("not", "true")
	}
	switch {
	case len(c.Ext) > 0:
		set("ext", array(c.Ext))
	case c.Category != "":
		set("category", quote(c.Category))
	case len(c.NameEquals) > 0:
		set("name_equals", array(c.NameEquals))
	case len(c.NameContains) > 0:
		set("name_contains", array(c.NameContains))
	case len(c.NameStartsWith) > 0:
		set("name_starts_with", array(c.NameStartsWith))
	case len(c.NameEndsWith) > 0:
		set("name_ends_with", array(c.NameEndsWith))
	case len(c.NameRegex) > 0:
		set("name_regex", array(c.NameRegex))
	case c.LargerThan != "":
		set("larger_than", quote(c.LargerThan))
	case c.SmallerThan != "":
		set("smaller_than", quote(c.SmallerThan))
	case c.ModifiedWithinDays != nil:
		set("modified_within_days", fmt.Sprint(*c.ModifiedWithinDays))
	case c.ModifiedOlderThanDays != nil:
		set("modified_older_than_days", fmt.Sprint(*c.ModifiedOlderThanDays))
	case c.CreatedWithinDays != nil:
		set("created_within_days", fmt.Sprint(*c.CreatedWithinDays))
	case c.CreatedOlderThanDays != nil:
		set("created_older_than_days", fmt.Sprint(*c.CreatedOlderThanDays))
	case c.InFolder != "":
		set("in_folder", quote(c.InFolder))
	case len(c.PathContains) > 0:
		set("path_contains", array(c.PathContains))
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

// array writes a list of strings on one line.
func array(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = quote(v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// comment is a trailing remark on the line it follows.
func comment(text string) string { return "  # " + text }

// quote writes a TOML basic string.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
