package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/08820048/owlsort-cli/internal/apply"
	"github.com/08820048/owlsort-cli/internal/exitcode"
	"github.com/08820048/owlsort-cli/internal/journal"
	"github.com/08820048/owlsort-cli/internal/output"
	"github.com/08820048/owlsort-cli/internal/plan"
)

// tidy shows the plan, asks, then carries it out. Nothing here is allowed to
// surprise the user: without a terminal and without --yes it only previews.
func tidy(cmd *cobra.Command, args []string, g *flags, yes bool) error {
	s, err := load(g)
	if err != nil {
		return err
	}
	p, err := planFor(s, args, false)
	if err != nil {
		return &ExitError{Code: exitcode.Error, Message: err.Error()}
	}

	out := cmd.OutOrStdout()
	if !g.Quiet {
		if err := output.Preview(out, p, g.Options); err != nil {
			return &ExitError{Code: exitcode.Error, Message: err.Error()}
		}
	}
	if p.Count() == 0 {
		return nil
	}

	// Safety rules 1 and 2: a preview, then a yes.
	if !yes {
		if !stdinIsTerminal() {
			fmt.Fprintln(out, "No terminal to ask in, so nothing was moved. Run `owlsort tidy --yes` to do it without asking.")
			return &ExitError{Code: exitcode.NotConfirmed}
		}
		ok, err := confirm(cmd, p.Count())
		if err != nil {
			return &ExitError{Code: exitcode.Error, Message: err.Error()}
		}
		if !ok {
			fmt.Fprintln(out, "Nothing moved.")
			return &ExitError{Code: exitcode.NotConfirmed}
		}
	}

	j, err := openJournal()
	if err != nil {
		return &ExitError{Code: exitcode.Error, Message: err.Error()}
	}
	res, err := apply.Run(p, j, apply.Options{Conflict: s.cfg.Conflict})
	if err != nil {
		return &ExitError{Code: exitcode.Error, Message: err.Error()}
	}

	if g.JSON {
		if err := output.WriteJSON(out, runJSONFrom(res)); err != nil {
			return &ExitError{Code: exitcode.Error, Message: err.Error()}
		}
	} else if !g.Quiet {
		printRun(out, res, "moved", forwardReason)
	}
	return afterRun(res)
}

// confirm asks the one question a tidy asks.
func confirm(cmd *cobra.Command, count int) (bool, error) {
	word := "files"
	if count == 1 {
		word = "file"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Tidy %d %s? [y/N] ", count, word)
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	switch line {
	case "y\n", "Y\n", "yes\n", "YES\n", "y", "Y", "yes", "YES":
		return true, nil
	}
	return false, nil
}

// undo puts a batch back.
func undo(cmd *cobra.Command, batch string, g *flags) error {
	j, err := openJournal()
	if err != nil {
		return &ExitError{Code: exitcode.Error, Message: err.Error()}
	}
	if batch != "" {
		batches, err := j.Batches()
		if err != nil {
			return &ExitError{Code: exitcode.Error, Message: err.Error()}
		}
		found := false
		for _, b := range batches {
			if b == batch {
				found = true
				break
			}
		}
		if !found {
			return &ExitError{
				Code:    exitcode.NothingToUndo,
				Message: fmt.Sprintf("there is no batch %q in %s", batch, j.Path()),
			}
		}
	}

	res, err := apply.Undo(batch, j, apply.Options{})
	if err != nil {
		return &ExitError{Code: exitcode.Error, Message: err.Error()}
	}
	out := cmd.OutOrStdout()
	if res.Batch == "" && res.Done == 0 {
		fmt.Fprintln(out, "There is nothing to undo.")
		return &ExitError{Code: exitcode.NothingToUndo}
	}

	if g.JSON {
		if err := output.WriteJSON(out, runJSONFrom(res)); err != nil {
			return &ExitError{Code: exitcode.Error, Message: err.Error()}
		}
	} else if !g.Quiet {
		printRun(out, res, "put back", undoReason)
	}
	return afterRun(res)
}

// history lists the runs, newest first.
func history(cmd *cobra.Command, limit int, g *flags) error {
	j, err := openJournal()
	if err != nil {
		return &ExitError{Code: exitcode.Error, Message: err.Error()}
	}
	runs, err := j.Runs()
	if err != nil {
		return &ExitError{Code: exitcode.Error, Message: err.Error()}
	}
	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}
	out := cmd.OutOrStdout()
	if g.JSON {
		if err := output.WriteJSON(out, runs); err != nil {
			return &ExitError{Code: exitcode.Error, Message: err.Error()}
		}
		return nil
	}
	if len(runs) == 0 {
		fmt.Fprintf(out, "No runs yet. %s does not exist.\n", j.Path())
		return nil
	}
	fmt.Fprintf(out, "%-22s  %-16s  %5s  %s\n", "BATCH", "WHEN", "FILES", "WHERE TO")
	for _, run := range runs {
		where := "-"
		if len(run.To) > 0 {
			where = displayPaths(run.To)
		}
		if run.Trashed > 0 {
			where = fmt.Sprintf("%s (trash: %d)", where, run.Trashed)
		}
		if run.Undone {
			where += " (undone)"
		}
		fmt.Fprintf(out, "%-22s  %-16s  %5d  %s\n", run.Batch, run.Time.Local().Format("2006-01-02 15:04"), run.Moved, where)
	}
	return nil
}

// afterRun maps a result onto the documented exit codes.
func afterRun(res apply.Result) error {
	if len(res.Failures) > 0 {
		// Something went wrong that the user may need to look at. The steps that
		// did work are recorded, so this is not silent.
		return &ExitError{Code: exitcode.Partial}
	}
	if len(res.Skipped) > 0 {
		return &ExitError{Code: exitcode.Partial}
	}
	return nil
}

// printRun is the sentence a person gets after a run. reasonText words the
// reasons the way that run means them: "changed" is a different thing before a
// move than after it.
func printRun(w io.Writer, res apply.Result, verb string, reasonText func(plan.SkipReason) string) {
	switch {
	case res.Done == 0:
		fmt.Fprintln(w, "Nothing was "+verb+".")
	case res.Done == 1:
		fmt.Fprintf(w, "1 file %s.\n", verb)
	default:
		fmt.Fprintf(w, "%d files %s.\n", res.Done, verb)
	}
	if len(res.Skipped) > 0 {
		reasons := map[plan.SkipReason]int{}
		for _, s := range res.Skipped {
			reasons[s.Reason]++
		}
		parts := make([]string, 0, len(reasons))
		for reason, n := range reasons {
			parts = append(parts, fmt.Sprintf("%s: %d", reasonText(reason), n))
		}
		fmt.Fprintf(w, "Left alone: %d (%s).\n", len(res.Skipped), joinSorted(parts))
	}
	for _, f := range res.Failures {
		fmt.Fprintf(w, "Failed: %s (%v).\n", f.Path, f.Err)
	}
	if verb == "moved" && res.Done > 0 {
		fmt.Fprintf(w, "Undo with `owlsort undo --batch %s`.\n", res.Batch)
	}
}

// forwardReason words a skip the way a tidy means it.
func forwardReason(reason plan.SkipReason) string { return reason.Text() }

// undoReason words a skip the way an undo means it: a file the run moved that is
// no longer where the run left it was not "changed since the preview".
func undoReason(reason plan.SkipReason) string {
	switch reason {
	case plan.SkipChanged:
		return "no longer where the run left it"
	case plan.SkipConflict:
		return "the original place is taken"
	case plan.SkipNeedsForeground:
		return "cannot be put back from here"
	}
	return reason.Text()
}

// runJSON is the --json shape of tidy and undo. It is a draft; docs/commands.md
// is where the final one is written down.
type runJSON struct {
	Batch    string         `json:"batch"`
	Done     int            `json:"done"`
	Skipped  []plan.Skipped `json:"skipped"`
	Failed   int            `json:"failed"`
	Failures []failureJSON  `json:"failures,omitempty"`
}

type failureJSON struct {
	Path  string `json:"path"`
	Rule  string `json:"rule,omitempty"`
	Error string `json:"error"`
}

func runJSONFrom(res apply.Result) runJSON {
	out := runJSON{Batch: res.Batch, Done: res.Done, Skipped: res.Skipped, Failed: res.Failed}
	if out.Skipped == nil {
		out.Skipped = []plan.Skipped{}
	}
	for _, f := range res.Failures {
		out.Failures = append(out.Failures, failureJSON{Path: f.Path, Rule: f.Rule, Error: f.Err.Error()})
	}
	return out
}

// joinSorted joins summary fragments after sorting them, so the same counts
// always read the same way.
func joinSorted(parts []string) string {
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// openJournal is the journal at its platform location. Nothing is created until
// the first step is written.
func openJournal() (*journal.File, error) {
	path, err := journal.DefaultPath()
	if err != nil {
		return nil, err
	}
	return journal.Open(path)
}

// stdinIsTerminal reports whether there is a person to ask. It is a variable so
// a test can stand in for a terminal; nothing else in the program writes to it.
var stdinIsTerminal = func() bool { return isTerminalFile(os.Stdin) }

// displayPaths shortens long paths and caps the list, so a history line stays a
// line.
func displayPaths(paths []string) string {
	const max = 3
	home, _ := os.UserHomeDir()
	short := make([]string, 0, len(paths))
	for i, p := range paths {
		if i == max {
			short = append(short, fmt.Sprintf("+%d more", len(paths)-max))
			break
		}
		if home != "" {
			if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
				if rel == "." {
					p = "~"
				} else {
					p = "~/" + filepath.ToSlash(rel)
				}
			}
		}
		short = append(short, p)
	}
	return joinSorted(short)
}
