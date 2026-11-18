package output

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/08820048/owlsort-cli/internal/plan"
)

// folder is a watched folder and a file inside it, built the way the platform
// would spell it.
func fixture() (string, plan.Item) {
	folder := filepath.Join(string(filepath.Separator), "w")
	return folder, plan.Item{
		Source:      filepath.Join(folder, "report.pdf"),
		Destination: filepath.Join(folder, "Documents", "PDF", "report.pdf"),
		Action:      plan.Move,
		Rule:        "PDFs",
	}
}

func TestPathsAreShownWithForwardSlashesEverywhere(t *testing.T) {
	folder, item := fixture()
	// The configuration writes "Documents/PDF"; the output must read the same on
	// every platform, so a test and a screenshot do not change with the OS.
	if got := destination(folder, item); got != "Documents/PDF/" {
		t.Errorf("destination = %q, want %q", got, "Documents/PDF/")
	}
	if got := sourceName(folder, item); got != "report.pdf" {
		t.Errorf("source = %q, want %q", got, "report.pdf")
	}
}

func TestATrashAndARenameSayWhatTheyDo(t *testing.T) {
	folder, item := fixture()
	item.Action = plan.Trash
	item.Destination = ""
	if got := destination(folder, item); got != "the trash" {
		t.Errorf("trash destination = %q", got)
	}
	item.Action = plan.Rename
	item.Destination = filepath.Join(folder, "renamed.pdf")
	if got := destination(folder, item); got != "renamed.pdf" {
		t.Errorf("rename destination = %q", got)
	}
}

func TestPreviewShowsThePlanAndTheSkips(t *testing.T) {
	folder, item := fixture()
	p := plan.Plan{
		Version: plan.Version,
		Folders: []plan.Folder{{
			Path:    folder,
			Items:   []plan.Item{item},
			Skipped: []plan.Skipped{{Path: filepath.Join(folder, "x.crdownload"), Reason: plan.SkipDownloading}},
		}},
	}
	var out bytes.Buffer
	if err := Preview(&out, p, Options{NoColor: true}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"1 file can be tidied", "report.pdf", "Documents/PDF/", "Skipped: 1 (still downloading: 1)", "owlsort undo"} {
		if !strings.Contains(text, want) {
			t.Errorf("preview does not mention %q:\n%s", want, text)
		}
	}
}

func TestPreviewWithNothingToDoSaysSo(t *testing.T) {
	var out bytes.Buffer
	p := plan.Plan{Version: plan.Version, Folders: []plan.Folder{{Path: "/w"}}}
	if err := Preview(&out, p, Options{NoColor: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Nothing to tidy") {
		t.Errorf("said %q", out.String())
	}
}
