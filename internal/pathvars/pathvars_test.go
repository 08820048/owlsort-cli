package pathvars

import (
	"testing"
	"time"

	"github.com/08820048/owlsort-cli/internal/i18n"
)

var arrived = time.Date(2026, 1, 9, 23, 30, 0, 0, time.UTC)

func TestExpand(t *testing.T) {
	f := File{Extension: "PNG", Arrived: arrived}
	for template, want := range map[string]string{
		"Desktop Archive/{date}/{category}": "Desktop Archive/2026-01-09/Images",
		"{year}/{month}/{day}":              "2026/01/09",
		"{ext}":                             "png",
		"Pictures/Screenshots":              "Pictures/Screenshots",
	} {
		if got := Expand(template, f, i18n.English, time.UTC); got != want {
			t.Errorf("%q: got %q, want %q", template, got, want)
		}
	}
	if got := Expand("{category}", f, i18n.SimplifiedChinese, time.UTC); got != "图片" {
		t.Errorf("zh category: %q", got)
	}
}

func TestDatesFollowTheTimeZone(t *testing.T) {
	f := File{Arrived: arrived}
	tokyo := time.FixedZone("JST", 9*3600) // 23:30 UTC is already the 10th there
	if got := Expand("{date}", f, i18n.English, tokyo); got != "2026-01-10" {
		t.Errorf("got %q", got)
	}
}

func TestAValueNeverLeavesItsFolder(t *testing.T) {
	// An extension is user-controlled (a file called "x.a/b" has extension "a/b" to a sloppy
	// caller): whatever it holds, it stays one path segment.
	for ext, want := range map[string]string{"a/b": "a-b", "..": "-", "": "-"} {
		if got := Expand("x/{ext}", File{Extension: ext, Arrived: arrived}, i18n.English, time.UTC); got != "x/"+want {
			t.Errorf("%q: got %q", ext, got)
		}
	}
}

func TestFixedPrefix(t *testing.T) {
	for template, want := range map[string]string{
		"Archive/{date}/{category}": "Archive",
		"/data/Archive/{year}":      "/data/Archive",
		"{date}":                    "",
		"Pictures/Screenshots":      "Pictures/Screenshots",
	} {
		if got := FixedPrefix(template); got != want {
			t.Errorf("%q: got %q, want %q", template, got, want)
		}
	}
}
