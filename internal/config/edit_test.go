package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/08820048/owlsort-cli/internal/i18n"
	"github.com/08820048/owlsort-cli/internal/tomledit"
)

// errNoSuchRule stands in for a change that cannot be made.
var errNoSuchRule = errors.New("no such rule")

// writeConfig puts a configuration somewhere throwaway and returns its path.
func writeConfigFile(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEditChangesTheFileAndKeepsTheComments(t *testing.T) {
	source := string(Template(i18n.English, nil))
	path := writeConfigFile(t, source)

	changed, err := Edit(path, func(doc *tomledit.Doc) error {
		block, err := doc.FindRule("PDFs")
		if err != nil {
			return err
		}
		return doc.SetBool(block, "enabled", false)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("Edit reported no change")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "# Owlsort") || !strings.Contains(string(after), "may run unattended") {
		t.Error("the comments are gone")
	}
	if !strings.Contains(string(after), "name = \"PDFs\"\nenabled = false\n") {
		t.Errorf("the rule was not switched off:\n%s", after)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("the file no longer loads: %v", err)
	}
	for _, r := range cfg.Rules {
		if r.Name == "PDFs" && r.IsEnabled() {
			t.Error("PDFs is still on")
		}
	}
}

func TestEditWritesNothingWhenThereIsNoChange(t *testing.T) {
	path := writeConfigFile(t, "folders = []\n")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	changed, err := Edit(path, func(*tomledit.Doc) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("Edit reported a change it did not make")
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("the file was rewritten for nothing")
	}
}

func TestTheGuardRefusesAnEditThatWouldBreakTheFile(t *testing.T) {
	// The guard is what makes a text edit safe, so it is tested on its own: an
	// edit that loses a rule, renames one, or leaves text that does not parse must
	// all be refused, because each would silently change what the program does.
	source := []byte(string(Template(i18n.English, nil)))

	cases := map[string]struct {
		after []byte
		want  string
	}{
		"a rule lost": {
			after: []byte(strings.Replace(string(source),
				"[[rule]]\nname = \"PDFs\"\nwhen = [{ ext = [\"pdf\"] }]\nmove_to = \"Documents/PDF\"\n", "", 1)),
			want: "number of rules",
		},
		"a rule renamed": {
			after: []byte(strings.Replace(string(source), `name = "PDFs"`, `name = "PDF files"`, 1)),
			want:  "rename rule",
		},
		"the file no longer parses": {
			after: []byte(strings.Replace(string(source), `conflict = "rename"`, `conflict = rename`, 1)),
			want:  "does not parse",
		},
	}
	for name, c := range cases {
		err := checkStillUsable(source, c.after)
		if err == nil {
			t.Errorf("%s: the guard allowed it", name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: message = %q, want it to mention %q", name, err, c.want)
		}
	}
}

func TestTheGuardAllowsARealEdit(t *testing.T) {
	source := string(Template(i18n.English, nil))
	after := strings.Replace(source, `name = "PDFs"`, "name = \"PDFs\"\nenabled = false", 1)
	if err := checkStillUsable([]byte(source), []byte(after)); err != nil {
		t.Errorf("a legitimate edit was refused: %v", err)
	}
}

func TestEditWritesNothingWhenTheChangeFails(t *testing.T) {
	source := string(Template(i18n.English, nil))
	path := writeConfigFile(t, source)

	sentinel := strings.NewReader("").Len // any error will do
	_ = sentinel
	_, err := Edit(path, func(*tomledit.Doc) error { return errNoSuchRule })
	if err == nil {
		t.Fatal("the error was swallowed")
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != source {
		t.Error("the file was changed even though the change failed")
	}
}

func TestEditKeepsTheFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no file permission bits to keep")
	}
	path := writeConfigFile(t, "folders = []\n")
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Edit(path, func(doc *tomledit.Doc) error { return doc.Append("folders", "~/a") }); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Errorf("mode = %o, want 640", got)
	}
}

func TestEditFollowsASymlink(t *testing.T) {
	real := writeConfigFile(t, "folders = []\n")
	link := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}

	if _, err := Edit(link, func(doc *tomledit.Doc) error { return doc.Append("folders", "~/a") }); err != nil {
		t.Fatal(err)
	}
	// The link is still a link, and the file it points at changed. A config kept
	// in dotfiles is usually a link, and replacing it would silently break the
	// setup it points at.
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced by a regular file")
	}
	content, err := os.ReadFile(real)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"~/a"`) {
		t.Errorf("the target was not changed: %s", content)
	}
}

func TestEditRefusesAConfigurationThatIsNotOne(t *testing.T) {
	path := writeConfigFile(t, "this is not toml = = =\n")
	if _, err := Edit(path, func(doc *tomledit.Doc) error { return doc.Append("folders", "~/a") }); err == nil {
		t.Error("editing something that is not a configuration was allowed")
	}
}

func TestSamePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home folder")
	}
	dir := t.TempDir()
	if !SamePath("~/Downloads", filepath.Join(home, "Downloads")) {
		t.Error("~ and the home folder should be the same path")
	}
	if !SamePath(dir+"/", dir) {
		t.Error("a trailing separator should not matter")
	}
	if SamePath("/a/b", "/a/c") {
		t.Error("different folders compared equal")
	}
}

func TestEditRefusesAFileThatIsNotRegular(t *testing.T) {
	dir := t.TempDir()
	if _, err := Edit(dir, func(*tomledit.Doc) error { return nil }); err == nil {
		t.Error("a folder was accepted as a configuration file")
	}
}
