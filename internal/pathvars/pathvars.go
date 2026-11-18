// Package pathvars expands the variables a destination or rename template may
// contain: {date} {year} {month} {day} {category} {ext}.
//
// The date is the day the file arrived (see File.Arrived). Every filled-in
// value is a single path segment: it never carries a separator or a parent
// step, so a variable can never move a file out of its folder.
package pathvars

import (
	"fmt"
	"strings"
	"time"

	"github.com/08820048/owlsort-cli/internal/category"
	"github.com/08820048/owlsort-cli/internal/i18n"
)

// Names lists the variables.
var Names = []string{"{date}", "{year}", "{month}", "{day}", "{category}", "{ext}"}

// File is what expansion needs to know about a file.
type File struct {
	Extension string    // without the dot, as on disk
	Arrived   time.Time // the day it landed in its folder (birth time, else modification time)
}

// Contains reports whether a template uses any variable.
func Contains(template string) bool {
	for _, name := range Names {
		if strings.Contains(template, name) {
			return true
		}
	}
	return false
}

// Expand fills the variables in template for f, with dates in loc.
func Expand(template string, f File, lang i18n.Language, loc *time.Location) string {
	if !Contains(template) {
		return template
	}
	if loc == nil {
		loc = time.Local
	}
	d := f.Arrived.In(loc)
	cat := category.ForExtension(f.Extension)
	r := strings.NewReplacer(
		"{date}", fmt.Sprintf("%04d-%02d-%02d", d.Year(), int(d.Month()), d.Day()),
		"{year}", fmt.Sprintf("%04d", d.Year()),
		"{month}", fmt.Sprintf("%02d", int(d.Month())),
		"{day}", fmt.Sprintf("%02d", d.Day()),
		"{category}", segment(i18n.CategoryName(lang, cat)),
		"{ext}", segment(strings.ToLower(f.Extension)),
	)
	return r.Replace(template)
}

// segment makes a value safe as one path segment.
func segment(s string) string {
	s = strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(s)
	if s == "" || s == "." || s == ".." {
		return "-"
	}
	return s
}

// FixedPrefix is the part of a template before the first folder that holds a
// variable: "Archive/{date}/{category}" gives "Archive". Files anywhere under
// that folder were already filed by the rule, so later scans leave them alone.
func FixedPrefix(template string) string {
	if !Contains(template) {
		return template
	}
	var fixed []string
	for _, part := range strings.FieldsFunc(template, func(r rune) bool { return r == '/' || r == '\\' }) {
		if Contains(part) {
			break
		}
		fixed = append(fixed, part)
	}
	prefix := strings.Join(fixed, "/")
	if strings.HasPrefix(template, "/") {
		return "/" + prefix
	}
	return prefix
}
