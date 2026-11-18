package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/08820048/owlsort-cli/internal/i18n"
)

// The template is generated, so the test that matters is that it says exactly
// what Default says: a template that drifts from the built-in rules would give
// every new user a different starting point from the one the tests cover.
func TestTemplateIsExactlyTheBuiltInRules(t *testing.T) {
	for _, lang := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		data := Template(lang, nil)
		got, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: the template does not parse: %v\n%s", lang, err, data)
		}
		want := Default(lang)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the template and the built-in rules differ\n--- template ---\n%+v\n--- built-in ---\n%+v", lang, got, want)
		}
	}
}

func TestTemplateTakesTheFoldersItIsGiven(t *testing.T) {
	got, err := Parse(Template(i18n.English, []string{"/tmp/a", "/tmp/b"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Folders) != 2 || got.Folders[0] != "/tmp/a" || got.Folders[1] != "/tmp/b" {
		t.Errorf("folders = %v", got.Folders)
	}
	if len(got.Rules) != len(Default(i18n.English).Rules) {
		t.Error("the rules changed when the folders did")
	}
}

func TestTemplateIsReadable(t *testing.T) {
	data := string(Template(i18n.English, nil))
	for _, want := range []string{
		"# Owlsort",
		"owlsort preview",
		`conflict = "rename"  #`,
		`  { category = "image" },`,
		`move_to = "Pictures/Screenshots"`,
		"enabled = false",
	} {
		if !strings.Contains(data, want) {
			t.Errorf("the template does not contain %q", want)
		}
	}
	// The whole point: no machine-shaped [[rule.when]] tables, and no fields
	// written out just because they have a zero value.
	for _, unwanted := range []string{"[[rule.when]]", "trash = false", "continue = false", `match = ""`} {
		if strings.Contains(data, unwanted) {
			t.Errorf("the template contains %q", unwanted)
		}
	}
}
