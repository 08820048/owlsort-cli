package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/08820048/owlsort-cli/internal/check"
	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/doctor"
	"github.com/08820048/owlsort-cli/internal/exitcode"
	"github.com/08820048/owlsort-cli/internal/output"
	"github.com/08820048/owlsort-cli/internal/protect"
	"github.com/08820048/owlsort-cli/internal/rules"
)

// rulesCheck reads the configuration and reports what is wrong with it. It is
// the one command that reads an invalid configuration on purpose: telling the
// user what is broken is its whole job.
func rulesCheck(cmd *cobra.Command, g *flags) error {
	path, err := configPath(g)
	if err != nil {
		return &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}
	// ParseLoose, not Load: the command has to report every problem, and the
	// strict loader stops at the first one.
	cfg, parseErr := loadLoose(path)
	var problems []check.Problem
	switch {
	case parseErr != nil:
		// The file cannot be read at all, so that is the one thing to report.
		problems = []check.Problem{{Severity: check.Error, Message: parseErr.Error()}}
	default:
		problems = check.Rules(cfg)
	}

	out := cmd.OutOrStdout()
	if g.JSON {
		if err := output.WriteJSON(out, problemsJSON(problems)); err != nil {
			return &ExitError{Code: exitcode.Error, Message: err.Error()}
		}
	} else {
		printProblems(out, path, problems)
	}

	if check.HasErrors(problems) || parseErr != nil {
		// A check that finds a mistake fails, the way `go vet` does: a script
		// asks "is this configuration usable?" and the exit code is the answer.
		return &ExitError{Code: exitcode.Error}
	}
	return nil
}

// loadLoose reads a configuration without stopping on what it says, so a check
// can list every problem rather than the first.
func loadLoose(path string) (config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return config.Config{}, fmt.Errorf("no configuration at %s; run `owlsort init` first", path)
	}
	return config.ParseLoose(data)
}

// printProblems writes the findings, one per line, with a summary.
func printProblems(w io.Writer, path string, problems []check.Problem) {
	if len(problems) == 0 {
		fmt.Fprintf(w, "%s: no problems found.\n", path)
		return
	}
	errors, warnings := 0, 0
	for _, p := range problems {
		label := "error"
		if p.Severity == check.Warning {
			label = "warning"
			warnings++
		} else {
			errors++
		}
		fmt.Fprintf(w, "%s: %s\n", label, p.Message)
	}
	fmt.Fprintf(w, "\n%s: %s.\n", path, summarise(errors, warnings))
	if warnings > 0 && errors == 0 {
		fmt.Fprintln(w, "A warning does not stop the program: the configuration is valid.")
	}
}

// summarise is the tail of a check.
func summarise(errors, warnings int) string {
	if errors == 0 && warnings == 0 {
		return "no problems found"
	}
	parts := make([]string, 0, 2)
	if errors > 0 {
		parts = append(parts, plural(errors, "error"))
	}
	if warnings > 0 {
		parts = append(parts, plural(warnings, "warning"))
	}
	return joinSorted(parts)
}

// problemJSON is one finding in --json.
type problemJSON struct {
	Severity string `json:"severity"`
	Rule     string `json:"rule,omitempty"`
	Message  string `json:"message"`
}

func problemsJSON(problems []check.Problem) []problemJSON {
	out := make([]problemJSON, 0, len(problems))
	for _, p := range problems {
		out = append(out, problemJSON{Severity: string(p.Severity), Rule: p.Rule, Message: p.Message})
	}
	return out
}

// uncovered lists the loose files no rule takes, grouped by extension.
func uncovered(cmd *cobra.Command, args []string, g *flags) error {
	s, err := load(g)
	if err != nil {
		return err
	}
	files, err := scanFor(s, args)
	if err != nil {
		return &ExitError{Code: exitcode.Error, Message: err.Error()}
	}
	groups, err := rules.Uncovered(files, s.cfg, protect.New(s.cfg), s.env)
	if err != nil {
		return &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}
	out := cmd.OutOrStdout()
	if g.JSON {
		if err := output.WriteJSON(out, groups); err != nil {
			return &ExitError{Code: exitcode.Error, Message: err.Error()}
		}
		return nil
	}
	if len(groups) == 0 {
		fmt.Fprintln(out, "Every loose file is covered by a rule.")
		return nil
	}
	total := 0
	for _, group := range groups {
		total += group.Count
	}
	fmt.Fprintf(out, "%d loose file(s) no rule covers, by extension:\n\n", total)
	for _, group := range groups {
		fmt.Fprintln(out, "  "+group.String())
	}
	fmt.Fprintln(out, "\nAdd a rule to a copy of the built-in set, or turn on one of the rules that ship off.")
	return &ExitError{Code: exitcode.Partial}
}

// doctorRun checks the machine and says what it found.
func doctorRun(cmd *cobra.Command, g *flags) error {
	path, pathErr := configPath(g)
	var (
		cfg    *config.Config
		detail string
	)
	if pathErr != nil {
		detail = pathErr.Error()
	} else if loaded, err := loadLoose(path); err != nil {
		detail = err.Error()
	} else {
		// A configuration with mistakes is still worth checking against: the
		// watched folders can be examined, and the problems get reported.
		cfg = &loaded
	}

	report := doctor.Run(cfg, path, detail)
	out := cmd.OutOrStdout()
	if g.JSON {
		if err := output.WriteJSON(out, report); err != nil {
			return &ExitError{Code: exitcode.Error, Message: err.Error()}
		}
	} else {
		printReport(out, report)
	}
	if report.Failed() {
		return &ExitError{Code: exitcode.Error}
	}
	return nil
}

// printReport writes one line per check.
func printReport(w io.Writer, report doctor.Report) {
	for _, c := range report.Checks {
		mark := "ok  "
		switch {
		case !c.OK && !c.Warning:
			mark = "FAIL"
		case c.Warning:
			mark = "warn"
		}
		fmt.Fprintf(w, "%-4s %-16s %s\n", mark, c.Name, c.Detail)
	}
	if report.Failed() {
		fmt.Fprintln(w, "\nSomething above has to be fixed before tidying will work.")
	} else {
		fmt.Fprintf(w, "\nReady. `owlsort preview` shows what would be tidied.\n")
	}
}

// plural is "1 error" / "2 errors".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
