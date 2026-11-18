package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/exitcode"
	"github.com/08820048/owlsort-cli/internal/output"
	"github.com/08820048/owlsort-cli/internal/tomledit"
)

// The commands in this file change the configuration file. They edit its text
// rather than re-encoding it, so the user's comments, order and spacing survive
// (internal/tomledit), and every write is checked by config.Edit before it lands.

// rulesList prints the rules.
func rulesList(cmd *cobra.Command, g *flags) error {
	path, err := configPath(g)
	if err != nil {
		return &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return configError(err, path)
	}
	if g.JSON {
		return writeJSONOrFail(cmd, rulesOut(cfg))
	}
	return fail(output.Rules(cmd.OutOrStdout(), cfg, path, g.Options))
}

// rulesShow prints one rule.
func rulesShow(cmd *cobra.Command, name string, g *flags) error {
	path, err := configPath(g)
	if err != nil {
		return &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return configError(err, path)
	}
	rule, ok := findRule(cfg, name)
	if !ok {
		return &ExitError{Code: exitcode.Error, Message: fmt.Sprintf("no rule called %q; `owlsort rules list` shows the names", name)}
	}
	if g.JSON {
		return writeJSONOrFail(cmd, ruleOutOf(rule))
	}
	return fail(output.Rule(cmd.OutOrStdout(), rule, path, g.Options))
}

// rulesSet turns a rule on or off by editing its `enabled` key.
func rulesSet(cmd *cobra.Command, name string, enabled bool, g *flags) error {
	path, err := configPath(g)
	if err != nil {
		return &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}

	// Read first, so a typo in the name is refused before the file is touched and
	// the "already set" case writes nothing.
	cfg, err := config.Load(path)
	if err != nil {
		return configError(err, path)
	}
	if _, ok := findRule(cfg, name); !ok {
		return &ExitError{Code: exitcode.Error, Message: fmt.Sprintf("no rule called %q; `owlsort rules list` shows the names", name)}
	}

	changed, err := config.Edit(path, func(doc *tomledit.Doc) error {
		block, err := doc.FindRule(name)
		if err != nil {
			return err
		}
		if current, ok := doc.BoolValue(block, "enabled"); ok && current == enabled {
			return nil // already what was asked for
		}
		return doc.SetBool(block, "enabled", enabled)
	})
	if err != nil {
		return configError(err, path)
	}

	state := "on"
	if !enabled {
		state = "off"
	}
	if !changed {
		fmt.Fprintf(cmd.OutOrStdout(), "Rule %q is already %s; nothing changed.\n", name, state)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Rule %q is %s.\n", name, state)
	}

	// The file was legal before the edit; say so if it is not legal now for some
	// other reason, rather than letting `preview` fail later without explanation.
	if cfg, err := config.Load(path); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "owlsort: %v\n", err)
		return &ExitError{Code: exitcode.Usage}
	} else if len(cfg.Problems()) > 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "owlsort: the configuration still has problems; `owlsort rules check` lists them.")
	}
	return nil
}

// rulesEdit opens the configuration in the user's editor and checks the result.
func rulesEdit(cmd *cobra.Command, g *flags) error {
	path, err := configPath(g)
	if err != nil {
		return &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return &ExitError{
			Code:    exitcode.Usage,
			Message: fmt.Sprintf("no configuration at %s; run `owlsort init` first", path),
		}
	}

	editor, err := editorCommand()
	if err != nil {
		return &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}

	// The editor needs the terminal it was started from; handing it a pipe would
	// only open something that cannot be typed into.
	run := exec.Command(editor[0], append(editor[1:], path)...)
	run.Stdin = cmd.InOrStdin()
	run.Stdout = cmd.OutOrStdout()
	run.Stderr = cmd.ErrOrStderr()
	if err := run.Run(); err != nil {
		return &ExitError{Code: exitcode.Error, Message: fmt.Sprintf("%s: %v", editor[0], err)}
	}

	// "Validate on save": the editor cannot be hooked, so the check happens the
	// moment it exits, and reports what is wrong with what was saved.
	out := cmd.OutOrStdout()
	cfg, loadErr := config.Load(path)
	if loadErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "owlsort: %v\n", loadErr)
		fmt.Fprintln(cmd.ErrOrStderr(), "The file was saved as you left it. Fix the problem above, or undo the change.")
		return &ExitError{Code: exitcode.Usage}
	}
	problems := cfg.Problems()
	if len(problems) == 0 {
		fmt.Fprintf(out, "%s: no problems found.\n", path)
		return nil
	}
	for _, p := range problems {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", p.Message)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "The file was saved as you left it; the program will not run until these are fixed.")
	return &ExitError{Code: exitcode.Usage}
}

// editorCommand is $VISUAL, else $EDITOR, else an editor this machine is likely to
// have. It refuses when there is no terminal to open an editor in.
func editorCommand() ([]string, error) {
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return strings.Fields(value), nil
		}
	}
	if !stdinIsTerminal() {
		return nil, errors.New("no editor: set $EDITOR, or run this from a terminal with `vi`")
	}
	for _, candidate := range []string{"vi", "nano"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return []string{candidate}, nil
		}
	}
	return nil, errors.New("no editor found: set $EDITOR")
}

// listStrings prints a list such as `folders`.
func listStrings(cmd *cobra.Command, key, title, empty string, g *flags) error {
	path, err := configPath(g)
	if err != nil {
		return &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return configError(err, path)
	}
	values := cfg.Folders
	if key == "keep" {
		values = cfg.Keep
	}
	if g.JSON {
		return writeJSONOrFail(cmd, valuesOutOf(key, values))
	}
	return fail(output.Strings(cmd.OutOrStdout(), title, path, values, empty, g.Options))
}

// addString appends a folder or a kept path, refusing one that is not there: the
// point of the list is that the program looks at it, and a typo would silently
// watch nothing.
func addString(cmd *cobra.Command, key, value string, g *flags) error {
	path, err := configPath(g)
	if err != nil {
		return &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return &ExitError{Code: exitcode.Usage, Message: "a path is required"}
	}
	expanded := config.ExpandHome(value)
	info, statErr := os.Stat(expanded)
	if statErr != nil {
		return &ExitError{Code: exitcode.Usage, Message: fmt.Sprintf("%s: %v", expanded, statErr)}
	}
	if key == "folders" && !info.IsDir() {
		return &ExitError{Code: exitcode.Usage, Message: fmt.Sprintf("%s is not a folder", expanded)}
	}

	cfg, err := config.Load(path)
	if err != nil {
		return configError(err, path)
	}
	for _, existing := range listOf(cfg, key) {
		if config.SamePath(existing, value) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s is already in %s; nothing changed.\n", existing, key)
			return nil
		}
	}

	if _, err := config.Edit(path, func(doc *tomledit.Doc) error { return doc.Append(key, value) }); err != nil {
		return configError(err, path)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Added %s to %s.\n", value, key)
	afterEditHint(cmd, path)
	return nil
}

// removeString takes a folder or a kept path out of the list. Nothing is moved:
// this only stops the program from looking there (or stops protecting it).
func removeString(cmd *cobra.Command, key, value string, g *flags) error {
	path, err := configPath(g)
	if err != nil {
		return &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return configError(err, path)
	}

	// Find the entry as it is written in the file, so removing "~/Downloads"
	// works when the user typed "/Users/me/Downloads", and the file keeps the
	// short form it already has.
	written := ""
	for _, existing := range listOf(cfg, key) {
		if config.SamePath(existing, value) {
			written = existing
			break
		}
	}
	if written == "" {
		return &ExitError{
			Code:    exitcode.Error,
			Message: fmt.Sprintf("%s is not in %s; `owlsort %s list` shows what is", value, key, key),
		}
	}

	if _, err := config.Edit(path, func(doc *tomledit.Doc) error {
		_, err := doc.Remove(key, written)
		return err
	}); err != nil {
		return configError(err, path)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Removed %s from %s. Nothing was moved.\n", written, key)
	afterEditHint(cmd, path)
	return nil
}

// afterEditHint warns when the configuration was already imperfect, so a command
// that succeeded does not hide a problem the user has to fix before tidying works.
func afterEditHint(cmd *cobra.Command, path string) {
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "owlsort: %v\n", err)
		return
	}
	if len(cfg.Problems()) > 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "owlsort: the configuration has other problems; `owlsort rules check` lists them.")
	}
}

// listOf is the configured list a key names.
func listOf(cfg config.Config, key string) []string {
	if key == "keep" {
		return cfg.Keep
	}
	return cfg.Folders
}

// findRule looks a rule up by name.
func findRule(cfg config.Config, name string) (config.Rule, bool) {
	for _, r := range cfg.Rules {
		if r.Name == name {
			return r, true
		}
	}
	return config.Rule{}, false
}

// configError maps a failure to read or write the configuration onto exit code 2,
// which is what the documented contract says a bad configuration gets.
func configError(err error, path string) error {
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return err
	}
	message := err.Error()
	if errors.Is(err, os.ErrNotExist) {
		message = fmt.Sprintf("no configuration at %s; run `owlsort init` first", path)
	}
	return &ExitError{Code: exitcode.Usage, Message: message}
}

// writeJSONOrFail prints v as JSON, mapping a write failure to exit code 1.
func writeJSONOrFail(cmd *cobra.Command, v any) error {
	if err := output.WriteJSON(cmd.OutOrStdout(), v); err != nil {
		return &ExitError{Code: exitcode.Error, Message: err.Error()}
	}
	return nil
}

// fail maps a printing error to exit code 1.
func fail(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: exitcode.Error, Message: err.Error()}
}

// ruleOut is one rule for --json.
type ruleOut struct {
	Name       string   `json:"name"`
	Enabled    bool     `json:"enabled"`
	Match      string   `json:"match,omitempty"`
	Continue   bool     `json:"continue,omitempty"`
	Subfolders bool     `json:"subfolders,omitempty"`
	Background bool     `json:"background,omitempty"`
	Action     string   `json:"action"`
	To         string   `json:"to,omitempty"`
	When       []string `json:"when"`
}

func ruleOutOf(r config.Rule) ruleOut {
	out := ruleOut{
		Name: r.Name, Enabled: r.IsEnabled(), Match: r.Match,
		Continue: r.Continue, Subfolders: r.Subfolders, Background: r.Background,
		When: conditionTexts(r),
	}
	out.Action, out.To = output.ActionOf(r)
	return out
}

// conditionTexts is each condition as one line, for --json.
func conditionTexts(r config.Rule) []string {
	out := make([]string, 0, len(r.When))
	for _, c := range r.When {
		out = append(out, output.ConditionText(c))
	}
	return out
}

func rulesOut(cfg config.Config) []ruleOut {
	out := make([]ruleOut, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		out = append(out, ruleOutOf(r))
	}
	return out
}

// valuesOut is the --json shape of `folders list` and `keep list`.
type valuesOut struct {
	Folders []string `json:"folders,omitempty"`
	Keep    []string `json:"keep,omitempty"`
}

func valuesOutOf(key string, values []string) valuesOut {
	if key == "keep" {
		return valuesOut{Keep: values}
	}
	return valuesOut{Folders: values}
}
