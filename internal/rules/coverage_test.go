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

func TestUncoveredGroupsByExtensionMostFirst(t *testing.T) {
	folder := "/w"
	old := now.Add(-48 * time.Hour)
	files := []scan.File{
		mk(folder, "a.pdf", 10, old),   // covered
		mk(folder, "b.weird", 10, old), // uncovered
		mk(folder, "c.weird", 10, old), // uncovered
		mk(folder, "d.weird", 10, old), // uncovered
		mk(folder, "README", 10, old),  // uncovered, no extension
	}
	groups, err := Uncovered(files, config.Default(i18n.English), nil, testEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("got %+v", groups)
	}
	if groups[0].Extension != "weird" || groups[0].Count != 3 {
		t.Errorf("first group = %+v, want weird x3", groups[0])
	}
	if groups[1].ExtensionLabel() != "no extension" || groups[1].Count != 1 {
		t.Errorf("second group = %+v", groups[1])
	}
}

func TestUncoveredKnowsWhyAFileIsLeftAlone(t *testing.T) {
	// A protected, hidden or downloading file is not "uncovered": the plan
	// already explains it, and telling the user to write a rule would be wrong.
	folder := "/w"
	old := now.Add(-48 * time.Hour)
	hidden := mk(folder, ".DS_Store", 10, old)
	hidden.Hidden = true
	downloading := mk(folder, "film.mp4.part", 10, old)
	downloading.Downloading = true
	kept := mk(folder, "taxes.weird", 10, old)

	guard := staticGuard{kept.Path: plan.SkipKept}
	groups, err := Uncovered([]scan.File{hidden, downloading, kept}, config.Default(i18n.English), guard, testEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Errorf("got %+v, want nothing reported", groups)
	}
}

func TestUncoveredIgnoresFilesBelowTheTopLevel(t *testing.T) {
	folder := "/w"
	deep := mk(folder, "x.weird", 10, now)
	deep.Path = filepath.Join(folder, "sub", "x.weird")

	groups, err := Uncovered([]scan.File{deep}, config.Default(i18n.English), nil, testEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Errorf("a file in a subfolder is the user's own arrangement: %+v", groups)
	}

	cfg := config.Default(i18n.English)
	cfg.Rules[0].Subfolders = true // some rule asks for subfolders
	groups, err = Uncovered([]scan.File{deep}, cfg, nil, testEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Errorf("with a subfolder rule it is worth reporting: %+v", groups)
	}
}

func TestUncoveredDoesNotReportAFileAlreadyFiled(t *testing.T) {
	folder := "/w"
	filed := mk(folder, "a.pdf", 10, now)
	filed.Path = filepath.Join(folder, "Documents", "PDF", "a.pdf")

	cfg := config.Config{
		Conflict: config.ConflictRename,
		Rules: []config.Rule{{
			Name:   "PDFs",
			When:   []config.Condition{{Ext: []string{"pdf"}}},
			MoveTo: "Documents/PDF",
		}},
	}
	groups, err := Uncovered([]scan.File{filed}, cfg, nil, testEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Errorf("a filed file matches a rule that already did its work: %+v", groups)
	}
}

func TestUncoveredRefusesAnInvalidConfiguration(t *testing.T) {
	cfg := config.Default(i18n.English)
	cfg.Rules[0].MoveTo = "../out"
	if _, err := Uncovered(nil, cfg, nil, testEnv()); err == nil {
		t.Error("an invalid configuration was accepted")
	}
}

func TestEveryLooseFileCoveredSaysNothing(t *testing.T) {
	folder := "/w"
	groups, err := Uncovered([]scan.File{mk(folder, "a.pdf", 10, now)}, config.Default(i18n.English), nil, testEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Errorf("got %+v", groups)
	}
}

func TestAGroupReadsAsOneLine(t *testing.T) {
	g := UncoveredGroup{Extension: "weird", Count: 2, Examples: []string{"a.weird", "b.weird"}}
	line := g.String()
	for _, want := range []string{"weird", "2 files", "a.weird"} {
		if !strings.Contains(line, want) {
			t.Errorf("%q does not mention %q", line, want)
		}
	}
	if one := (UncoveredGroup{Extension: "x", Count: 1}).String(); !strings.Contains(one, "1 file ") {
		t.Errorf("singular reads %q", one)
	}
}
