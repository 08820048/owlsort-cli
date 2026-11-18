package config

import (
	"strings"
	"testing"

	"github.com/08820048/owlsort-cli/internal/i18n"
)

func TestDefaultIsValidInEveryLanguage(t *testing.T) {
	for _, lang := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		if err := Default(lang).Validate(); err != nil {
			t.Errorf("%s: %v", lang, err)
		}
	}
}

func TestDefaultOrderAndSwitches(t *testing.T) {
	c := Default(i18n.English)
	index := map[string]int{}
	for i, r := range c.Rules {
		index[r.Name] = i
	}
	if index["Desktop by date"] != 0 {
		t.Error("desktop by date must come first: whoever turns it on wants the desktop filed by day")
	}
	if index["Screenshots"] > index["Other pictures"] {
		t.Error("screenshots must come before other pictures")
	}
	if index["Installers"] > index["Archives"] {
		t.Error("installers must come before archives")
	}
	for _, name := range []string{"Desktop by date", "Office documents", "Other pictures", "Audio and video", "Screen recordings"} {
		if c.Rules[index[name]].IsEnabled() {
			t.Errorf("%s should start off", name)
		}
	}
	for _, name := range []string{"Screenshots", "PDFs", "Installers", "Archives"} {
		if !c.Rules[index[name]].IsEnabled() {
			t.Errorf("%s should start on", name)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	want := Default(i18n.SimplifiedChinese)
	data, err := Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	if len(got.Rules) != len(want.Rules) || got.Rules[1].MoveTo != want.Rules[1].MoveTo {
		t.Errorf("round trip changed the rules:\n%s", data)
	}
}

func TestParseExample(t *testing.T) {
	c, err := Parse([]byte(`
folders = ["~/Downloads"]
conflict = "skip"

[[rule]]
name = "Screenshots"
when = [
  { category = "image" },
  { name_contains = ["Screenshot", "截屏"] },
]
move_to = "Pictures/Screenshots"
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Conflict != ConflictSkip || len(c.Rules) != 1 || len(c.Rules[0].When) != 2 {
		t.Errorf("unexpected: %+v", c)
	}
}

func TestInvalid(t *testing.T) {
	cases := map[string]string{
		"two conditions in one":      "[[rule]]\nname='a'\nwhen=[{ext=['pdf'], category='image'}]\nmove_to='x'",
		"no conditions":              "[[rule]]\nname='a'\nmove_to='x'",
		"no action":                  "[[rule]]\nname='a'\nwhen=[{ext=['pdf']}]",
		"two actions":                "[[rule]]\nname='a'\nwhen=[{ext=['pdf']}]\nmove_to='x'\ntrash=true",
		"unknown category":           "[[rule]]\nname='a'\nwhen=[{category='spreadsheet'}]\nmove_to='x'",
		"bad regex":                  "[[rule]]\nname='a'\nwhen=[{name_regex=['(']}]\nmove_to='x'",
		"bad size":                   "[[rule]]\nname='a'\nwhen=[{larger_than='big'}]\nmove_to='x'",
		"destination climbs out":     "[[rule]]\nname='a'\nwhen=[{ext=['pdf']}]\nmove_to='../elsewhere'",
		"duplicate names":            "[[rule]]\nname='a'\nwhen=[{ext=['pdf']}]\nmove_to='x'\n[[rule]]\nname='a'\nwhen=[{ext=['zip']}]\nmove_to='y'",
		"unknown conflict":           "conflict='overwrite'",
		"unknown key is not ignored": "frobnicate = true",
	}
	for name, source := range cases {
		if _, err := Parse([]byte(source)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestEveryProblemIsReported(t *testing.T) {
	_, err := Parse([]byte("conflict='x'\n[[rule]]\nname='a'\nmove_to='../y'"))
	if err == nil {
		t.Fatal("accepted")
	}
	for _, want := range []string{"conflict", "no conditions", "climbs out"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in: %v", want, err)
		}
	}
}
