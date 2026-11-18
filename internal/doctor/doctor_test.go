package doctor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/08820048/owlsort-cli/internal/config"
)

// stateDir points the journal somewhere throwaway, on every platform.
func stateDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
}

// configIn writes a configuration and returns its path.
func configIn(t *testing.T, folder string) string {
	t.Helper()
	c := config.Default("en")
	c.Folders = []string{folder}
	data, err := config.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// find is the check with that name.
func find(t *testing.T, r Report, name string) Check {
	t.Helper()
	for _, c := range r.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no check called %q in %+v", name, r.Checks)
	return Check{}
}

func TestAHealthySetupIsReady(t *testing.T) {
	stateDir(t)
	folder := t.TempDir()
	if err := os.WriteFile(filepath.Join(folder, "report.pdf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configIn(t, folder))
	if err != nil {
		t.Fatal(err)
	}

	r := Run(&cfg, "somewhere.toml", "")
	if r.Failed() {
		t.Fatalf("a healthy setup failed: %+v", r.Checks)
	}
	if c := find(t, r, "configuration"); !c.OK {
		t.Errorf("configuration: %+v", c)
	}
	if c := find(t, r, "journal"); !c.OK {
		t.Errorf("journal: %+v", c)
	}
	if c := find(t, r, "watched folder"); !c.OK {
		t.Errorf("watched folder: %+v", c)
	}
	if !strings.Contains(find(t, r, "watched folder").Detail, "1 file") {
		t.Errorf("the folder check does not count the files: %q", find(t, r, "watched folder").Detail)
	}
}

func TestAMissingWatchFolderFails(t *testing.T) {
	stateDir(t)
	folder := t.TempDir()
	cfg, err := config.Load(configIn(t, folder))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Folders = []string{filepath.Join(folder, "not-there")}

	r := Run(&cfg, "somewhere.toml", "")
	if !r.Failed() {
		t.Fatalf("a missing folder passed: %+v", r.Checks)
	}
	if c := find(t, r, "watched folder"); c.OK {
		t.Errorf("watched folder: %+v", c)
	}
}

func TestAFileWhereTheFolderShouldBeFails(t *testing.T) {
	stateDir(t)
	folder := t.TempDir()
	cfg, err := config.Load(configIn(t, folder))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(folder, "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Folders = []string{file}

	if !Run(&cfg, "somewhere.toml", "").Failed() {
		t.Error("a file as a watched folder passed")
	}
}

func TestNoWatchedFoldersFails(t *testing.T) {
	stateDir(t)
	cfg := config.Default("en")
	cfg.Folders = nil
	if !Run(&cfg, "somewhere.toml", "").Failed() {
		t.Error("no watched folders passed")
	}
}

func TestAnInvalidConfigurationIsReportedWithItsProblems(t *testing.T) {
	stateDir(t)
	folder := t.TempDir()
	cfg := config.Default("en")
	cfg.Folders = []string{folder}
	cfg.Rules[0].MoveTo = "../out"

	r := Run(&cfg, "somewhere.toml", "")
	c := find(t, r, "configuration")
	if c.OK {
		t.Errorf("an invalid configuration passed: %+v", c)
	}
	if !strings.Contains(c.Detail, "rules check") {
		t.Errorf("the check does not point at `rules check`: %q", c.Detail)
	}
}

func TestAConfigurationThatCouldNotBeReadIsReported(t *testing.T) {
	stateDir(t)
	r := Run(nil, "/nope/config.toml", "open /nope/config.toml: no such file or directory")
	c := find(t, r, "configuration")
	if c.OK || !strings.Contains(c.Detail, "no such file") {
		t.Errorf("configuration: %+v", c)
	}
	if r.Failed() != true {
		t.Error("it should fail")
	}
}

func TestTheTrashIsOnlyAFailureWhenARuleUsesIt(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("this builds a throwaway home with no trash folder, which only macOS checks for")
	}
	stateDir(t)
	home := t.TempDir() // deliberately without .Trash
	t.Setenv("HOME", home)
	folder := t.TempDir()
	cfg, err := config.Load(configIn(t, folder))
	if err != nil {
		t.Fatal(err)
	}

	r := Run(&cfg, "somewhere.toml", "")
	c := find(t, r, "system trash")
	if !c.Warning || c.OK {
		t.Errorf("with no rule using the trash it should only warn: %+v", c)
	}
	if r.Failed() {
		t.Error("a warning must not fail the run")
	}

	cfg.Rules = append(cfg.Rules, config.Rule{
		Name:       "junk",
		When:       []config.Condition{{Ext: []string{"tmp"}}},
		Trash:      true,
		Background: true,
	})
	r = Run(&cfg, "somewhere.toml", "")
	c = find(t, r, "system trash")
	if c.OK || c.Warning {
		t.Errorf("with a trash rule it must be a failure: %+v", c)
	}
	if !r.Failed() {
		t.Error("it should fail")
	}
}
