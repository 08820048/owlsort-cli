// Package output prints results: for people (aligned text, color when the
// output is a terminal) and for scripts (--json). Every command goes through
// here so the two forms stay in step.
//
// Colors are off when stdout is not a terminal, when NO_COLOR is set, or with
// --no-color.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/08820048/owlsort-cli/internal/plan"
)

// Options are the global flags that shape output.
type Options struct {
	JSON    bool
	Quiet   bool
	NoColor bool
}

// JSON writes v as indented JSON, followed by a newline.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Preview prints a plan for a person: what would move where, and what was left
// alone. It prints only; nothing here touches a file.
func Preview(w io.Writer, p plan.Plan, opts Options) error {
	color := !opts.NoColor && os.Getenv("NO_COLOR") == "" && isTerminal(w)
	bold := func(s string) string { return paint(color, "1", s) }
	dim := func(s string) string { return paint(color, "2", s) }

	if p.Count() == 0 {
		fmt.Fprintln(w, "Nothing to tidy.")
	}

	for _, folder := range p.Folders {
		if len(folder.Items) == 0 {
			continue
		}
		word := "files"
		if len(folder.Items) == 1 {
			word = "file"
		}
		fmt.Fprintf(w, "%s · %d %s can be tidied\n", bold(displayFolder(folder.Path)), len(folder.Items), word)
		width := 0
		for _, item := range folder.Items {
			if n := runeLen(sourceName(folder.Path, item)); n > width {
				width = n
			}
		}
		for _, item := range folder.Items {
			name := sourceName(folder.Path, item)
			pad := strings.Repeat(" ", width-runeLen(name))
			fmt.Fprintf(w, "  %s%s %s %s\n", name, pad, dim("→"), destination(folder.Path, item))
		}
		if line := skippedLine(folder); line != "" {
			fmt.Fprintln(w, "  "+dim(line))
		}
	}

	if p.Count() > 0 {
		fmt.Fprintln(w, "Run `owlsort tidy` to move them. Every run can be undone with `owlsort undo`.")
	}
	return nil
}

// runeLen counts characters, not bytes: a Chinese file name is one column wide
// per character, and byte lengths would leave the arrows ragged.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// sourceName is the file's path as shown in a preview: inside its folder.
func sourceName(folder string, item plan.Item) string {
	if rel, err := filepath.Rel(folder, item.Source); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return item.Source
}

// destination is where the step puts the file, relative to the folder when it
// stays inside it. A rename shows the new name only.
func destination(folder string, item plan.Item) string {
	switch item.Action {
	case plan.Trash:
		return "the trash"
	case plan.Rename:
		return filepath.Base(item.Destination)
	}
	dir := filepath.Dir(item.Destination)
	rel, err := filepath.Rel(folder, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return dir
	}
	if rel == "." {
		return "./"
	}
	return rel + "/"
}

// skippedLine summarises the files a folder left alone, the way the README
// shows it: "Skipped: 2 (in a code project: 1, still downloading: 1)".
func skippedLine(folder plan.Folder) string {
	if len(folder.Skipped) == 0 {
		return ""
	}
	counts := map[plan.SkipReason]int{}
	for _, s := range folder.Skipped {
		counts[s.Reason]++
	}
	reasons := make([]string, 0, len(counts))
	keys := make([]plan.SkipReason, 0, len(counts))
	for reason := range counts {
		keys = append(keys, reason)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return string(keys[i]) < string(keys[j])
	})
	for _, reason := range keys {
		reasons = append(reasons, fmt.Sprintf("%s: %d", reason.Text(), counts[reason]))
	}
	return fmt.Sprintf("Skipped: %d (%s)", len(folder.Skipped), strings.Join(reasons, ", "))
}

// displayFolder shortens a long path for a heading: the home folder becomes "~".
func displayFolder(path string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, path); err == nil && rel != ".." && !strings.HasPrefix(rel, "..") {
			if rel == "." {
				return "~"
			}
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return path
}

// isTerminal reports whether w is a terminal, so color is not written into a
// pipe or a file.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// paint wraps s in an ANSI escape when color is on.
func paint(on bool, code, s string) string {
	if !on {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}
