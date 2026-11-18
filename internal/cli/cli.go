// Package cli is the command line: the cobra commands and their flags. A
// command parses its arguments, calls into the packages that do the work, and
// prints through internal/output. It holds no logic of its own.
//
// The command list, flags and exit codes are in docs/commands.md. Commands
// whose work is not built yet return notImplemented, naming the milestone in
// docs/roadmap.md, so the surface can be reviewed before the engine exists.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/08820048/owlsort-cli/internal/exitcode"
	"github.com/08820048/owlsort-cli/internal/output"
)

// BuildInfo is stamped in at release time.
type BuildInfo struct {
	Version, Commit, Date string
}

// ExitError carries a process exit code out of a command.
type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string { return e.Message }

func notImplemented(command, milestone string) error {
	return &ExitError{
		Code:    exitcode.Error,
		Message: fmt.Sprintf("`%s` is not built yet (planned for %s, see docs/roadmap.md)", command, milestone),
	}
}

// flags are the global flags.
type flags struct {
	output.Options
	Config string
	Lang   string
}

// New builds the command tree.
func New(info BuildInfo) *cobra.Command {
	g := &flags{}
	root := &cobra.Command{
		Use:   "owlsort",
		Short: "Tidy the loose files in Downloads and Desktop: preview first, undo anytime",
		Long: `Owlsort moves the screenshots, PDFs, installers and archives piling up in your
folders into tidy places by rules. Every move is shown first, every run can be
undone, and nothing is ever deleted for good.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", info.Version, info.Commit, info.Date),
	}
	pf := root.PersistentFlags()
	pf.BoolVar(&g.JSON, "json", false, "print machine-readable JSON")
	pf.BoolVarP(&g.Quiet, "quiet", "q", false, "print only what matters")
	pf.BoolVar(&g.NoColor, "no-color", false, "no colors (also honours NO_COLOR)")
	pf.StringVar(&g.Config, "config", "", "configuration file (default: see docs/config.md)")
	pf.StringVar(&g.Lang, "lang", "", "interface language: en, zh-Hans (default: from LANG)")

	root.AddCommand(
		initCmd(g), previewCmd(g), tidyCmd(g), undoCmd(g), historyCmd(g), explainCmd(g), uncoveredCmd(g),
		rulesCmd(g), foldersCmd(g), keepCmd(g), watchCmd(g), serviceCmd(g), doctorCmd(g),
	)
	return root
}
