package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/08820048/owlsort-cli/internal/exitcode"
)

func run(args ...string) (string, error) {
	root := New(BuildInfo{Version: "test", Commit: "abc", Date: "today"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestEveryCommandIsListed(t *testing.T) {
	out, err := run("--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"init", "preview", "tidy", "undo", "history", "explain", "uncovered", "rules", "folders", "keep", "watch", "service", "doctor"} {
		if !strings.Contains(out, name) {
			t.Errorf("--help does not list %q", name)
		}
	}
}

func TestVersion(t *testing.T) {
	out, err := run("--version")
	if err != nil || !strings.Contains(out, "test") {
		t.Errorf("%q, %v", out, err)
	}
}

func TestUnbuiltCommandsSayWhenTheyComeAndExitWithAnError(t *testing.T) {
	_, err := run("preview")
	var coded *ExitError
	if !errors.As(err, &coded) {
		t.Fatalf("got %v", err)
	}
	if coded.Code != exitcode.Error || !strings.Contains(coded.Message, "M0") {
		t.Errorf("%+v", coded)
	}
}

func TestBadUsageIsAnError(t *testing.T) {
	if _, err := run("explain"); err == nil {
		t.Error("explain without a file was accepted")
	}
}
