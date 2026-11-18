package tomledit

import (
	"errors"
	"strings"
	"testing"
)

// builtIn is the shape of a real configuration: top-level keys, comments, and
// rules that each carry their own keys.
const builtIn = `# Owlsort — tidy the loose files in Downloads and Desktop.
# Edit this file by hand; it is yours.

folders = ["~/Downloads", "~/Desktop"]
keep = [
  "~/Downloads/Taxes",
]
conflict = "rename"      # "rename" or "skip"
language = ""            # "", "en" or "zh-Hans"

# Rules are tried in order.
[[rule]]
name = "Screenshots"
enabled = true
match = "all"
when = [
  { category = "image" },
  { name_contains = ["Screenshot"] },
]
move_to = "Pictures/Screenshots"

[[rule]]
name = "PDFs"
when = [{ ext = ["pdf"] }]
move_to = "Documents/PDF"   # keep PDFs together
`

func mustParse(t *testing.T, source string) *Doc {
	t.Helper()
	return Parse([]byte(source))
}

func TestParseKeepsTheFileByteForByte(t *testing.T) {
	d := mustParse(t, builtIn)
	if d.String() != builtIn {
		t.Error("parsing changed the text")
	}
}

func TestRuleBlocksAreFoundInOrder(t *testing.T) {
	d := mustParse(t, builtIn)
	blocks := d.RuleBlocks()
	if len(blocks) != 2 {
		t.Fatalf("found %d rules", len(blocks))
	}
	for i, want := range []string{"Screenshots", "PDFs"} {
		got, ok := d.StringValue(blocks[i], "name")
		if !ok || got != want {
			t.Errorf("rule %d: name = %q (%v), want %q", i+1, got, ok, want)
		}
		if blocks[i].Index() != i+1 {
			t.Errorf("rule %d: Index = %d", i+1, blocks[i].Index())
		}
	}
}

func TestAKeyAfterTheLastRuleDoesNotBelongToTheFile(t *testing.T) {
	// `language` is top-level; `move_to` is not. Reading one for the other would
	// put a value in the wrong table.
	source := "language = \"en\"\n[[rule]]\nname = \"a\"\nmove_to = \"x\"\n"
	d := mustParse(t, source)
	rule := d.RuleBlocks()[0]
	if _, ok := d.StringValue(rule, "language"); ok {
		t.Error("a top-level key was read as part of a rule")
	}
	if got, ok := d.StringValue(rule, "move_to"); !ok || got != "x" {
		t.Errorf("move_to = %q (%v)", got, ok)
	}

	top := d.Blocks()[0]
	if top.Kind != "" {
		t.Errorf("the first block should be the top level, got kind %q", top.Kind)
	}
	if got, ok := d.StringValue(top, "language"); !ok || got != "en" {
		t.Errorf("language = %q (%v)", got, ok)
	}
}

func TestFindRule(t *testing.T) {
	d := mustParse(t, builtIn)
	if _, err := d.FindRule("PDFs"); err != nil {
		t.Errorf("PDFs: %v", err)
	}
	if _, err := d.FindRule("Missing"); !errors.Is(err, ErrNotUnderstood) {
		t.Errorf("a missing rule gave %v", err)
	}
}

func TestFindRuleRefusesADuplicateName(t *testing.T) {
	d := mustParse(t, "[[rule]]\nname = \"a\"\nmove_to = \"x\"\n[[rule]]\nname = \"a\"\nmove_to = \"y\"\n")
	_, err := d.FindRule("a")
	if err == nil {
		t.Fatal("a duplicate name was accepted")
	}
	if !strings.Contains(err.Error(), "2 rules") {
		t.Errorf("message = %q", err)
	}
}

func TestSetBoolChangesOnlyTheValue(t *testing.T) {
	d := mustParse(t, builtIn)
	rule, err := d.FindRule("Screenshots")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetBool(rule, "enabled", false); err != nil {
		t.Fatal(err)
	}
	out := d.String()
	if !strings.Contains(out, "enabled = false") {
		t.Errorf("the value did not change:\n%s", out)
	}
	// Everything else is still there: comments, spacing, order.
	for _, want := range []string{
		"# Owlsort — tidy the loose files in Downloads and Desktop.",
		`conflict = "rename"      # "rename" or "skip"`,
		`move_to = "Documents/PDF"   # keep PDFs together`,
		`  { name_contains = ["Screenshot"] },`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lost %q", want)
		}
	}
	if strings.Index(out, "Screenshots") > strings.Index(out, "PDFs") {
		t.Error("the order of the rules changed")
	}
}

func TestSetBoolAddsTheKeyWhenItIsMissing(t *testing.T) {
	d := mustParse(t, builtIn)
	rule, err := d.FindRule("PDFs")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.BoolValue(rule, "enabled"); ok {
		t.Fatal("PDFs already has an enabled key")
	}
	if err := d.SetBool(rule, "enabled", false); err != nil {
		t.Fatal(err)
	}
	out := d.String()
	if !strings.Contains(out, "name = \"PDFs\"\nenabled = false\nwhen =") {
		t.Errorf("the key was not added next to the name:\n%s", out)
	}
	// It went inside the right rule, not the previous one.
	if on, ok := d.BoolValue(d.RuleBlocks()[0], "enabled"); !ok || !on {
		t.Errorf("Screenshots' enabled changed (%v, %v)", on, ok)
	}
}

func TestSetBoolKeepsTheLineEndingAndATrailingComment(t *testing.T) {
	d := mustParse(t, "[[rule]]\nname = \"a\"\nenabled = false    # on by default\nmove_to = \"x\"\n")
	if err := d.SetBool(d.RuleBlocks()[0], "enabled", true); err != nil {
		t.Fatal(err)
	}
	want := "enabled = true    # on by default\nmove_to"
	if !strings.Contains(d.String(), want) {
		t.Errorf("want %q in:\n%s", want, d.String())
	}
	if strings.Contains(d.String(), "truewhen") {
		t.Errorf("the newline was swallowed:\n%s", d.String())
	}
}

func TestSetBoolKeepsIndentation(t *testing.T) {
	d := mustParse(t, "[[rule]]\n    name = \"a\"\n    move_to = \"x\"\n")
	rule := d.RuleBlocks()[0]
	if err := d.SetBool(rule, "enabled", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.String(), "    enabled = false") {
		t.Errorf("indentation lost:\n%s", d.String())
	}
}

func TestElements(t *testing.T) {
	d := mustParse(t, builtIn)
	folders, ok, err := d.Elements("folders")
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	want := []string{"~/Downloads", "~/Desktop"}
	if len(folders) != len(want) {
		t.Fatalf("folders = %v", folders)
	}
	for i := range want {
		if folders[i] != want[i] {
			t.Errorf("folder %d = %q, want %q", i, folders[i], want[i])
		}
	}

	keep, ok, err := d.Elements("keep")
	if err != nil || !ok || len(keep) != 1 || keep[0] != "~/Downloads/Taxes" {
		t.Errorf("keep = %v (%v, %v)", keep, ok, err)
	}

	if _, ok, err := d.Elements("language"); err != nil || ok {
		t.Error("a string is not an array")
	}
}

func TestAppendToOneLineArray(t *testing.T) {
	d := mustParse(t, builtIn)
	if err := d.Append("folders", "/Volumes/Backup"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.String(), `folders = ["~/Downloads", "~/Desktop", "/Volumes/Backup"]`) {
		t.Errorf("got:\n%s", d.String())
	}
}

func TestAppendToOnePerLineArray(t *testing.T) {
	d := mustParse(t, builtIn)
	if err := d.Append("keep", "~/Documents/Scan"); err != nil {
		t.Fatal(err)
	}
	want := "keep = [\n  \"~/Downloads/Taxes\",\n  \"~/Documents/Scan\",\n]"
	if !strings.Contains(d.String(), want) {
		t.Errorf("want\n%s\ngot\n%s", want, d.String())
	}
}

func TestAppendAddsACommaWhenTheLastElementHasNone(t *testing.T) {
	d := mustParse(t, "keep = [\n  \"~/a\",\n  \"~/b\"\n]\n")
	if err := d.Append("keep", "~/c"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.String(), "\"~/b\",\n  \"~/c\",\n]") {
		t.Errorf("got:\n%s", d.String())
	}
}

func TestAppendCreatesTheKeyBeforeTheFirstRule(t *testing.T) {
	// A top-level key written after `[[rule]]` would belong to that rule.
	d := mustParse(t, "[[rule]]\nname = \"a\"\nmove_to = \"x\"\n")
	if err := d.Append("keep", "~/Taxes"); err != nil {
		t.Fatal(err)
	}
	out := d.String()
	if strings.Index(out, "keep =") > strings.Index(out, "[[rule]]") {
		t.Errorf("keep was added after the first rule:\n%s", out)
	}
}

func TestAppendToAnEmptyArray(t *testing.T) {
	d := mustParse(t, "folders = []\n")
	if err := d.Append("folders", "~/Downloads"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.String(), `folders = ["~/Downloads"]`) {
		t.Errorf("got %q", d.String())
	}
}

func TestRemoveTakesTheWholeLine(t *testing.T) {
	d := mustParse(t, builtIn)
	ok, err := d.Remove("keep", "~/Downloads/Taxes")
	if err != nil || !ok {
		t.Fatalf("remove: %v %v", ok, err)
	}
	want := "keep = [\n]\n"
	if !strings.Contains(d.String(), want) {
		t.Errorf("want a clean empty array, got:\n%s", d.String())
	}
}

func TestRemoveFromAOneLineArray(t *testing.T) {
	cases := map[string]string{
		`folders = ["a", "b", "c"]`: `folders = ["a", "c"]`,
		`folders = ["a", "b"]`:      `folders = ["a"]`,
		`folders = ["b", "c"]`:      `folders = ["c"]`,
		`folders = ["b"]`:           `folders = []`,
	}
	for source, want := range cases {
		d := mustParse(t, source+"\n")
		ok, err := d.Remove("folders", "b")
		if err != nil || !ok {
			t.Fatalf("%s: %v %v", source, ok, err)
		}
		got := strings.TrimSpace(d.String())
		if got != want {
			t.Errorf("%s -> %q, want %q", source, got, want)
		}
	}
}

func TestRemoveReportsWhenTheValueIsNotThere(t *testing.T) {
	d := mustParse(t, builtIn)
	ok, err := d.Remove("folders", "/nowhere")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("removing something that is not there reported a change")
	}
	if d.String() != builtIn {
		t.Error("the file changed")
	}
}

func TestLiteralStringsAreReadAndWrittenAsWritten(t *testing.T) {
	d := mustParse(t, "folders = ['/Users/me/My Files']\n")
	got, ok, err := d.Elements("folders")
	if err != nil || !ok || len(got) != 1 || got[0] != "/Users/me/My Files" {
		t.Fatalf("got %v (%v, %v)", got, ok, err)
	}
	// Appending keeps the user's quoting for what was there and uses its own for
	// the new value: a single quote is a legal path character.
	if err := d.Append("folders", "/two"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.String(), `'/Users/me/My Files', "/two"`) {
		t.Errorf("got %q", d.String())
	}
}

func TestABracketInAStringIsNotATable(t *testing.T) {
	d := mustParse(t, "keep = [\"/tmp/[draft]\"]\n[[rule]]\nname = \"a\"\nmove_to = \"x\"\n")
	if got := len(d.RuleBlocks()); got != 1 {
		t.Errorf("found %d rules", got)
	}
	if got, ok, _ := d.Elements("keep"); !ok || got[0] != "/tmp/[draft]" {
		t.Errorf("keep = %v", got)
	}
}

func TestAHashInAStringIsNotAComment(t *testing.T) {
	d := mustParse(t, "folders = [\"/tmp/a#b\"]\n")
	got, ok, err := d.Elements("folders")
	if err != nil || !ok || got[0] != "/tmp/a#b" {
		t.Errorf("got %v (%v, %v)", got, ok, err)
	}
}

func TestQuotedKeyNamesWork(t *testing.T) {
	d := mustParse(t, "[[rule]]\n\"name\" = \"a\"\nmove_to = \"x\"\n")
	rule := d.RuleBlocks()[0]
	if got, ok := d.StringValue(rule, "name"); !ok || got != "a" {
		t.Errorf("name = %q (%v)", got, ok)
	}
}

func TestAColorfulPathSurvivesAWrite(t *testing.T) {
	d := mustParse(t, "folders = []\n")
	awful := `/Volumes/O'Brien/dir "with" quotes`
	if err := d.Append("folders", awful); err != nil {
		t.Fatal(err)
	}
	got, ok, err := d.Elements("folders")
	if err != nil || !ok || len(got) != 1 || got[0] != awful {
		t.Errorf("got %q (%v, %v); file:\n%s", got, ok, err, d.String())
	}
}
