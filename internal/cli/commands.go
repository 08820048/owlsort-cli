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
			cfg := config.Default(i18n.Resolve(g.Lang))
			if len(folders) > 0 {
				cfg.Folders = folders
			}
			path, err := configPath(g)
			if err != nil {
				return &ExitError{Code: exitcode.Usage, Message: err.Error()}
			}
			if err := config.WriteNew(path, cfg); err != nil {
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
			if g.JSON {
				return &ExitError{Code: exitcode.Usage, Message: "explain has no --json shape yet (see docs/commands.md)"}
			}
			s, err := load(g)
			if err != nil {
				return err
			}
			f, err := fileFor(config.ExpandHome(args[0]))
			if err != nil {
				return &ExitError{Code: exitcode.Error, Message: err.Error()}
			}
			text, err := rules.Explain(f, s.cfg, protect.New(s.cfg), s.env)
			if err != nil {
				return &ExitError{Code: exitcode.Usage, Message: err.Error()}
			}
			fmt.Fprintln(cmd.OutOrStdout(), text)
			return nil
		},
	}
}

func uncoveredCmd(g *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "uncovered [path...]",
		Short: "List files no rule covers, grouped by extension",
		RunE:  func(*cobra.Command, []string) error { return notImplemented("uncovered", "M2") },
	}
}

func rulesCmd(g *flags) *cobra.Command {
	cmd := &cobra.Command{Use: "rules", Short: "List, switch and check the rules"}
	sub := func(use, short, milestone string, args cobra.PositionalArgs) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: args,
			RunE: func(c *cobra.Command, _ []string) error { return notImplemented("rules "+c.Name(), milestone) },
		}
	}
	cmd.AddCommand(
		sub("list", "List the rules and whether each is on", "M2", cobra.NoArgs),
		sub("show <name>", "Show one rule", "M2", cobra.ExactArgs(1)),
		sub("enable <name>", "Turn a rule on", "M2", cobra.ExactArgs(1)),
		sub("disable <name>", "Turn a rule off", "M2", cobra.ExactArgs(1)),
		sub("edit", "Open the configuration in $EDITOR and check it on save", "M2", cobra.NoArgs),
		sub("check", "Find mistakes: missing values, bad regexes, rules shadowed by earlier ones", "M2", cobra.NoArgs),
	)
	return cmd
}

func foldersCmd(g *flags) *cobra.Command {
	cmd := &cobra.Command{Use: "folders", Short: "List, add and remove watched folders"}
	sub := func(use, short string, args cobra.PositionalArgs) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: args,
			RunE: func(c *cobra.Command, _ []string) error { return notImplemented("folders "+c.Name(), "M2") },
		}
	}
	cmd.AddCommand(
		sub("list", "List watched folders", cobra.NoArgs),
		sub("add <path>", "Watch a folder", cobra.ExactArgs(1)),
		sub("remove <path>", "Stop watching a folder (moves nothing)", cobra.ExactArgs(1)),
	)
	return cmd
}

func keepCmd(g *flags) *cobra.Command {
	cmd := &cobra.Command{Use: "keep", Short: "Files and folders that are never touched"}
	sub := func(use, short string, args cobra.PositionalArgs) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: args,
			RunE: func(c *cobra.Command, _ []string) error { return notImplemented("keep "+c.Name(), "M2") },
		}
	}
	cmd.AddCommand(
		sub("list", "List what is kept", cobra.NoArgs),
		sub("add <path>", "Never touch this file or folder", cobra.ExactArgs(1)),
		sub("remove <path>", "Allow tidying it again", cobra.ExactArgs(1)),
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
		RunE:  func(*cobra.Command, []string) error { return notImplemented("doctor", "M2") },
	}
}
