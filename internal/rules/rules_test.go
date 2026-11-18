package rules

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/i18n"
	"github.com/08820048/owlsort-cli/internal/plan"
	"github.com/08820048/owlsort-cli/internal/scan"
)

// now is a fixed clock, so a plan does not depend on the day the test runs.
var now = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

func testEnv() Env {
	return Env{Lang: i18n.English, Location: time.UTC, Now: now}
}

func ext(name string) string {
	e := filepath.Ext(name)
	if len(e) < 2 || e == name {
		return ""
	}
	return e[1:]
}

// mk builds the file the scan would have described.
func mk(folder, name string, size int64, modified time.Time) scan.File {
	return scan.File{
		Path: filepath.Join(folder, name), Name: name, Extension: ext(name),
		Size: size, Modified: modified, Created: modified, Folder: folder,
	}
}

// cfg builds a configuration with the default conflict strategy, which every
// valid configuration needs.
func cfg(rules ...config.Rule) config.Config {
	return config.Config{Folders: []string{"/w"}, Conflict: config.ConflictRename, Rules: rules}
}

// staticGuard protects the paths it lists.
type staticGuard map[string]plan.SkipReason

func (g staticGuard) Protected(path string) plan.SkipReason { return g[path] }

func plan1(t *testing.T, files []scan.File, c config.Config, guard staticGuard, env Env) plan.Plan {
	t.Helper()
	p, err := Build(files, c, guard, env)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTheBuiltInRulesFileTheLooseFiles(t *testing.T) {
	folder := "/w"
	old := now.Add(-48 * time.Hour)
	files := []scan.File{
		mk(folder, "Screenshot 2026-11-18 at 09.14.png", 2<<20, old),
		mk(folder, "report-final.pdf", 300<<10, old),
		mk(folder, "Owlsort-0.2.11.dmg", 40<<20, old),
		mk(folder, "photos.zip", 5<<20, old),
	}
	p := plan1(t, files, config.Default(i18n.English), nil, testEnv())

	if p.Version != plan.Version {
		t.Errorf("version = %d, want %d", p.Version, plan.Version)
	}
	if p.Count() != 4 {
		t.Fatalf("planned %d steps: %+v", p.Count(), p.Folders)
	}
	want := map[string]string{
		"Screenshot 2026-11-18 at 09.14.png": "Pictures/Screenshots",
		"report-final.pdf":                   "Documents/PDF",
		"Owlsort-0.2.11.dmg":                 "Installers",
		"photos.zip":                         "Archives",
	}
	for _, item := range p.Folders[0].Items {
		dir, err := filepath.Rel(folder, filepath.Dir(item.Destination))
		if err != nil {
			t.Fatal(err)
		}
		if dir != want[filepath.Base(item.Source)] {
			t.Errorf("%s: went to %q, want %q", filepath.Base(item.Source), dir, want[filepath.Base(item.Source)])
		}
		if item.Action != plan.Move {
			t.Errorf("%s: action %q", item.Source, item.Action)
		}
	}
}

func TestContinueLetsALaterRuleTakeTheFile(t *testing.T) {
	c := cfg(
		config.Rule{Name: "first", When: []config.Condition{{Ext: []string{"pdf"}}}, MoveTo: "A", Continue: true},
		config.Rule{Name: "second", When: []config.Condition{{Category: "document"}}, MoveTo: "B"},
	)
	p := plan1(t, []scan.File{mk("/w", "a.pdf", 10, now)}, c, nil, testEnv())
	if len(p.Folders[0].Items) != 1 {
		t.Fatalf("got %+v", p.Folders[0])
	}
	item := p.Folders[0].Items[0]
	if item.Rule != "second" || filepath.Base(filepath.Dir(item.Destination)) != "B" {
		t.Errorf("got rule %q to %q; continue should let the later rule win", item.Rule, item.Destination)
	}
}

func TestAnyAndNotConditions(t *testing.T) {
	c := cfg(
		config.Rule{
			Name:   "any",
			Match:  "any",
			When:   []config.Condition{{Category: "image"}, {NameContains: []string{"report"}}},
			MoveTo: "Mixed",
		},
	)
	files := []scan.File{
		mk("/w", "holiday.png", 10, now),
		mk("/w", "report-final.pdf", 10, now),
		mk("/w", "notes.txt", 10, now),
	}
	p := plan1(t, files, c, nil, testEnv())
	if p.Count() != 2 {
		t.Errorf("match any: planned %d, want 2 (%+v)", p.Count(), p.Folders[0].Skipped)
	}

	negated := cfg(config.Rule{
		Name:   "notpdf",
		When:   []config.Condition{{Not: true, Ext: []string{"pdf"}}},
		MoveTo: "Rest",
	})
	p = plan1(t, []scan.File{mk("/w", "a.pdf", 10, now), mk("/w", "b.txt", 10, now)}, negated, nil, testEnv())
	if p.Count() != 1 || filepath.Base(p.Folders[0].Items[0].Source) != "b.txt" {
		t.Errorf("not: got %+v", p.Folders[0].Items)
	}
}

func TestNameConditions(t *testing.T) {
	day := "2026-11-18"
	cases := []struct {
		name string
		cond config.Condition
		file string
		want bool
	}{
		{"ext", config.Condition{Ext: []string{"png"}}, "a.png", true},
		{"ext upper", config.Condition{Ext: []string{"PNG"}}, "a.png", true},
		{"contains", config.Condition{NameContains: []string{"screen"}}, "Screenshot.png", true},
		{"equals", config.Condition{NameEquals: []string{"a.png"}}, "a.png", true},
		{"equals case", config.Condition{NameEquals: []string{"A.png"}}, "a.png", false},
		{"starts", config.Condition{NameStartsWith: []string{"owl"}}, "Owlsort.dmg", true},
		{"ends", config.Condition{NameEndsWith: []string{".dmg"}}, "Owlsort.dmg", true},
		{"regex", config.Condition{NameRegex: []string{`\d{4}-\d{2}-\d{2}`}}, "Screenshot " + day + ".png", true},
		{"category", config.Condition{Category: "archive"}, "photos.zip", true},
		{"larger", config.Condition{LargerThan: "1MB"}, "big.bin", true},
		{"smaller", config.Condition{SmallerThan: "1MB"}, "big.bin", false},
	}
	for _, c := range cases {
		f := mk("/w", c.file, 2<<20, now)
		rule := config.Rule{Name: c.name, When: []config.Condition{c.cond}, MoveTo: "x"}
		got := match(f, rule, testEnv())
		if got != c.want {
			t.Errorf("%s: %s -> %v, want %v", c.name, c.file, got, c.want)
		}
	}
}

func TestAgeConditionsUseTheClockThatWasPassedIn(t *testing.T) {
	old := now.Add(-10 * 24 * time.Hour)
	f := mk("/w", "a.pdf", 10, old)

	recent := config.Rule{Name: "recent", When: []config.Condition{{ModifiedWithinDays: intp(7)}}, MoveTo: "x"}
	if match(f, recent, testEnv()) {
		t.Error("ten days old is not within seven")
	}
	stale := config.Rule{Name: "stale", When: []config.Condition{{ModifiedOlderThanDays: intp(7)}}, MoveTo: "x"}
	if !match(f, stale, testEnv()) {
		t.Error("ten days old is older than seven")
	}
}

func TestOnlyTheTopLevelIsTidiedUnlessTheRuleAsks(t *testing.T) {
	file := mk("/w", "a.pdf", 10, now)
	file.Path = filepath.Join("/w", "sub", "a.pdf")

	flat := cfg(config.Rule{Name: "flat", When: []config.Condition{{Ext: []string{"pdf"}}}, MoveTo: "Docs"})
	if p := plan1(t, []scan.File{file}, flat, nil, testEnv()); p.Count() != 0 {
		t.Errorf("a file in a subfolder was planned by a rule that does not ask for subfolders")
	}

	deep := cfg(config.Rule{Name: "deep", When: []config.Condition{{Ext: []string{"pdf"}}}, MoveTo: "Docs", Subfolders: true})
	if p := plan1(t, []scan.File{file}, deep, nil, testEnv()); p.Count() != 1 {
		t.Errorf("a rule with subfolders did not reach the file")
	}
}

func TestFilesAlreadyFiledAreSettled(t *testing.T) {
	filed := mk("/w", "a.png", 10, now)
	filed.Path = filepath.Join("/w", "Pictures", "Screenshots", "a.png")

	byDate := mk("/w", "b.png", 10, now)
	byDate.Path = filepath.Join("/w", "Archive", "2026-11-18", "Images", "b.png")

	c := cfg(
		config.Rule{Name: "shots", When: []config.Condition{{Category: "image"}}, MoveTo: "Pictures/Screenshots"},
		config.Rule{Name: "dated", When: []config.Condition{{Category: "image"}}, MoveTo: "Archive/{date}/{category}"},
	)
	p := plan1(t, []scan.File{filed, byDate}, c, nil, testEnv())
	if p.Count() != 0 {
		t.Errorf("filed files were planned again: %+v", p.Folders[0].Items)
	}
}

func TestADestinationCannotClimbOut(t *testing.T) {
	bad := cfg(config.Rule{Name: "out", When: []config.Condition{{Ext: []string{"pdf"}}}, MoveTo: "../elsewhere"})
	if _, err := Build([]scan.File{mk("/w", "a.pdf", 10, now)}, bad, nil, testEnv()); err == nil {
		t.Error("a destination with .. was accepted")
	}
}

func TestAbsoluteDestinationsRunOnlyByHand(t *testing.T) {
	target := t.TempDir()
	c := cfg(config.Rule{Name: "abs", When: []config.Condition{{Ext: []string{"pdf"}}}, MoveTo: target})
	files := []scan.File{mk("/w", "a.pdf", 10, now)}

	unattended := testEnv()
	unattended.Unattended = true
	p := plan1(t, files, c, nil, unattended)
	if p.Count() != 0 || len(p.Folders[0].Skipped) != 1 || p.Folders[0].Skipped[0].Reason != plan.SkipNeedsForeground {
		t.Errorf("unattended: got %+v", p.Folders[0])
	}

	p = plan1(t, files, c, nil, testEnv())
	if p.Count() != 1 || p.Folders[0].Items[0].Destination != filepath.Join(target, "a.pdf") {
		t.Errorf("by hand: got %+v", p.Folders[0].Items)
	}
}

func TestTrashNeedsBackgroundWhenUnattended(t *testing.T) {
	c := cfg(config.Rule{Name: "junk", When: []config.Condition{{Ext: []string{"tmp"}}}, Trash: true})
	files := []scan.File{mk("/w", "old.tmp", 10, now)}
	unattended := testEnv()
	unattended.Unattended = true
	p := plan1(t, files, c, nil, unattended)
	if p.Count() != 0 {
		t.Error("a trash rule without background ran unattended")
	}

	c.Rules[0].Background = true
	p = plan1(t, files, c, nil, unattended)
	if p.Count() != 1 || p.Folders[0].Items[0].Action != plan.Trash {
		t.Errorf("background trash: got %+v", p.Folders[0])
	}
}

func TestSkipReasonsAreReported(t *testing.T) {
	folder := "/w"
	g := staticGuard{filepath.Join(folder, "kept.pdf"): plan.SkipKept}

	downloading := mk(folder, "film.mp4.crdownload", 10, now)
	downloading.Downloading = true
	hidden := mk(folder, ".DS_Store", 10, now)
	hidden.Hidden = true

	files := []scan.File{
		mk(folder, "kept.pdf", 10, now),
		downloading,
		hidden,
	}
	p := plan1(t, files, config.Default(i18n.English), g, testEnv())
	got := map[plan.SkipReason]int{}
	for _, s := range p.Folders[0].Skipped {
		got[s.Reason]++
	}
	for _, want := range []plan.SkipReason{plan.SkipKept, plan.SkipDownloading, plan.SkipHidden} {
		if got[want] != 1 {
			t.Errorf("%s: reported %d times, want 1 (%+v)", want, got[want], p.Folders[0].Skipped)
		}
	}
	if p.Count() != 0 {
		t.Errorf("skipped files were planned too: %+v", p.Folders[0].Items)
	}
}

func TestDateAndCategoryVariables(t *testing.T) {
	c := cfg(config.Rule{
		Name:   "dated",
		When:   []config.Condition{{Category: "document"}},
		MoveTo: "Archive/{date}/{category}",
	})
	p := plan1(t, []scan.File{mk("/w", "a.pdf", 10, now)}, c, nil, testEnv())
	want := filepath.Join("/w", "Archive", "2026-11-18", "Documents", "a.pdf")
	if got := p.Folders[0].Items[0].Destination; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExplainAgreesWithThePlan(t *testing.T) {
	c := config.Default(i18n.English)
	folder := "/w"
	files := []scan.File{
		mk(folder, "Screenshot 2026-11-18.png", 10, now),
		mk(folder, "report.pdf", 10, now),
		mk(folder, "unheard-of.weird", 10, now),
	}
	p, err := Build(files, c, nil, testEnv())
	if err != nil {
		t.Fatal(err)
	}
	planned := map[string]bool{}
	for _, item := range p.Folders[0].Items {
		planned[item.Source] = true
	}
	for _, f := range files {
		text, err := Explain(f, c, nil, testEnv())
		if err != nil {
			t.Fatal(err)
		}
		moves := strings.Contains(text, "moves to")
		if moves != planned[f.Path] {
			t.Errorf("%s: explain says %q, plan planned=%v", f.Name, text, planned[f.Path])
		}
	}
}

func intp(v int) *int { return &v }
