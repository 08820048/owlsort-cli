package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/exitcode"
	"github.com/08820048/owlsort-cli/internal/i18n"
	"github.com/08820048/owlsort-cli/internal/output"
	"github.com/08820048/owlsort-cli/internal/protect"
	"github.com/08820048/owlsort-cli/internal/rules"
)

// Every command below is a stub: the arguments are real (so `--help` and the
// shell completions can be reviewed), the work is not. Each names its milestone.

func initCmd(g *flags) *cobra.Command {
	var folders []string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the configuration file with the built-in rules",
		Long: `Writes the configuration with the built-in rules, then stops. It never
overwrites a file that is already there: edit the existing one instead.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := configPath(g)
			if err != nil {
				return &ExitError{Code: exitcode.Usage, Message: err.Error()}
			}
			template := config.Template(i18n.Resolve(g.Lang), folders)
			if err := config.WriteNew(path, template); err != nil {
				return &ExitError{Code: exitcode.Error, Message: err.Error()}
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Wrote %s\n", path)
			fmt.Fprintln(out, "Next: `owlsort preview` shows what would be tidied. Nothing moves until you run `owlsort tidy`.")
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&folders, "folders", nil, "folders to watch (default: ~/Downloads and ~/Desktop)")
	return cmd
}

func previewCmd(g *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "preview [path...]",
		Short: "Show what would be tidied, and where it would go (moves nothing)",
		Long: `Shows the same plan "tidy" would carry out, and moves nothing. Use it as
often as you like: it is a read-only view of your own folders.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := load(g)
			if err != nil {
				return err
			}
			p, err := planFor(s, args, false)
			if err != nil {
				return &ExitError{Code: exitcode.Error, Message: err.Error()}
			}
			out := cmd.OutOrStdout()
			if g.JSON {
				return output.WriteJSON(out, p)
			}
			return output.Preview(out, p, g.Options)
		},
	}
}

func tidyCmd(g *flags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "tidy [path...]",
		Short: "Show the preview, then tidy after you confirm",
		Long: `Shows what would be tidied and asks before moving anything.

Without a terminal (a script, a pipe) it only previews and exits with code 4,
unless --yes is given: running in a script never moves files by accident.`,
		RunE: func(cmd *cobra.Command, args []string) error { return tidy(cmd, args, g, yes) },
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	return cmd
}

func undoCmd(g *flags) *cobra.Command {
	var batch string
	cmd := &cobra.Command{
		Use:   "undo",
		Short: "Put back the newest batch (or the one given with --batch)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return undo(cmd, batch, g) },
	}
	cmd.Flags().StringVar(&batch, "batch", "", "batch id from `owlsort history`")
	return cmd
}

func historyCmd(g *flags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "history",
		Short: "List past runs, newest first: when, how many files, where to",
		RunE:  func(cmd *cobra.Command, _ []string) error { return history(cmd, limit, g) },
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "how many runs to show")
	return cmd
}

func explainCmd(g *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "explain <file>",
		Short: "Say why a file would (or would not) be tidied",
		Long: `Runs the same rules as "preview" over one file and reports the reason
instead of a plan, so the two can never disagree.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := load(g)
			if err != nil {
				return err
			}
			f, err := fileFor(config.ExpandHome(args[0]))
			if err != nil {
				return &ExitError{Code: exitcode.Error, Message: err.Error()}
			}
			explained, err := rules.Explain(f, s.cfg, protect.New(s.cfg), s.env)
			if err != nil {
				return &ExitError{Code: exitcode.Usage, Message: err.Error()}
			}
			out := cmd.OutOrStdout()
			if g.JSON {
				return writeJSONOrFail(cmd, explained)
			}
			return fail(output.Explain(out, explained))
		},
	}
}

func uncoveredCmd(g *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "uncovered [path...]",
		Short: "List files no rule covers, grouped by extension",
		RunE:  func(cmd *cobra.Command, args []string) error { return uncovered(cmd, args, g) },
	}
}

func rulesCmd(g *flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "List, show, switch and check the rules",
		Long: `Rules live in the configuration file, in the order they are tried. These
commands read and change that file; they edit it as text, so your comments,
your order and your spelling of each rule stay exactly as you wrote them.`,
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List the rules and whether each is on",
			Args:  cobra.NoArgs,
			RunE:  func(cmd *cobra.Command, _ []string) error { return rulesList(cmd, g) },
		},
		&cobra.Command{
			Use:   "show <name>",
			Short: "Show one rule",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return rulesShow(cmd, args[0], g)
			},
		},
		&cobra.Command{
			Use:   "enable <name>",
			Short: "Turn a rule on",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return rulesSet(cmd, args[0], true, g)
			},
		},
		&cobra.Command{
			Use:   "disable <name>",
			Short: "Turn a rule off",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return rulesSet(cmd, args[0], false, g)
			},
		},
		&cobra.Command{
			Use:   "edit",
			Short: "Open the configuration in $EDITOR and check it when you are done",
			Long: `Opens the configuration file in $VISUAL, or $EDITOR, or an editor this
machine has. When the editor exits the file is checked and any problem is
reported; the file is never thrown away, so a half-finished edit is still yours.`,
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error { return rulesEdit(cmd, g) },
		},
		&cobra.Command{
			Use:   "check",
			Short: "Find mistakes: missing values, bad regexes, rules shadowed by earlier ones",
			Long: `Reads the configuration and reports what is wrong with it, one problem per
line. A mistake makes the command exit with code 1; a warning (a rule no file can
reach, because an earlier rule already takes everything it would) does not.`,
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error { return rulesCheck(cmd, g) },
		},
	)
	return cmd
}

func foldersCmd(g *flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folders",
		Short: "List, add and remove watched folders",
		Long: `A watched folder is one the program looks at: only its top level is tidied
unless a rule asks for subfolders. Removing a folder from the list moves nothing;
it only stops the program looking there.`,
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List watched folders",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return listStrings(cmd, "folders", "Watched folders", "none yet: `owlsort folders add ~/Downloads`", g)
			},
		},
		&cobra.Command{
			Use:   "add <path>",
			Short: "Watch a folder",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return addString(cmd, "folders", args[0], g)
			},
		},
		&cobra.Command{
			Use:   "remove <path>",
			Short: "Stop watching a folder (moves nothing)",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return removeString(cmd, "folders", args[0], g)
			},
		},
	)
	return cmd
}

func keepCmd(g *flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keep",
		Short: "Files and folders that are never touched",
		Long: `Everything on this list is left alone, and so is everything inside it. It is
the strongest promise the program makes about a file, so it is checked when the
entry is added rather than at the next run: a typo here would protect nothing.`,
	}
	add := func(use, short, empty string) *cobra.Command {
		return &cobra.Command{
			Use:   use,
			Short: short,
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return listStrings(cmd, "keep", "Never touched", empty, g)
			},
		}
	}
	cmd.AddCommand(
		add("list", "List what is kept", "nothing yet: `owlsort keep add ~/Downloads/Taxes`"),
		&cobra.Command{
			Use:   "add <path>",
			Short: "Never touch this file or folder",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return addString(cmd, "keep", args[0], g)
			},
		},
		&cobra.Command{
			Use:   "remove <path>",
			Short: "Allow tidying it again",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return removeString(cmd, "keep", args[0], g)
			},
		},
	)
	return cmd
}

func watchCmd(g *flags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "watch [path...]",
		Short: "Watch folders and tidy new files once they have settled",
		Long: `Runs in the foreground until interrupted. Unattended, it is more careful than
a run by hand: only rules with background = true may send files to the trash.`,
		RunE: func(*cobra.Command, []string) error { return notImplemented("watch", "M3") },
	}
	cmd.Flags().BoolVar(&force, "force", false, "start even if the desktop app is tidying the same folder")
	return cmd
}

func serviceCmd(g *flags) *cobra.Command {
	cmd := &cobra.Command{Use: "service", Short: "Run `watch` in the background, started by the system"}
	sub := func(use, short string) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error { return notImplemented("service "+c.Name(), "M3") },
		}
	}
	cmd.AddCommand(
		sub("install", "Install (launchd on macOS, systemd user unit on Linux, a scheduled task on Windows)"),
		sub("uninstall", "Remove it"),
		sub("status", "Is it installed and running?"),
	)
	return cmd
}

func doctorCmd(g *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the setup: configuration, folders, trash, permissions",
		RunE:  func(cmd *cobra.Command, _ []string) error { return doctorRun(cmd, g) },
	}
}
