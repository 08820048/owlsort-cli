package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/exitcode"
	"github.com/08820048/owlsort-cli/internal/i18n"
	"github.com/08820048/owlsort-cli/internal/plan"
	"github.com/08820048/owlsort-cli/internal/protect"
	"github.com/08820048/owlsort-cli/internal/rules"
	"github.com/08820048/owlsort-cli/internal/scan"
)

// settings is what a command needs once the flags and the configuration have
// been read.
type settings struct {
	cfg  config.Config
	lang i18n.Language
	env  rules.Env
}

// load reads the configuration the way safety rule 12 requires: a file that
// cannot be read or is invalid is an error (exit code 2), never a silent fall
// back to the built-in rules.
func load(g *flags) (settings, error) {
	path, err := configPath(g)
	if err != nil {
		return settings{}, &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}
	cfg, err := config.Load(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return settings{}, &ExitError{
				Code:    exitcode.Usage,
				Message: fmt.Sprintf("no configuration at %s; run `owlsort init` first", path),
			}
		}
		return settings{}, &ExitError{Code: exitcode.Usage, Message: err.Error()}
	}
	lang := i18n.Resolve(languageSetting(g, cfg))
	env := rules.Env{Lang: lang, Location: time.Local, Now: time.Now()}
	return settings{cfg: cfg, lang: lang, env: env}, nil
}

// configPath is --config when given, else the platform default.
func configPath(g *flags) (string, error) {
	if g.Config != "" {
		return config.ExpandHome(g.Config), nil
	}
	return config.DefaultPath()
}

// languageSetting is --lang, else the configuration's language, else the
// environment (i18n.Resolve reads LANG when the setting is empty).
func languageSetting(g *flags, cfg config.Config) string {
	if g.Lang != "" {
		return g.Lang
	}
	return cfg.Language
}

// planFor builds the plan for the folders named on the command line, or for the
// configured ones. Nothing here moves a file.
func planFor(s settings, args []string, unattended bool) (plan.Plan, error) {
	folders := args
	if len(folders) == 0 {
		folders = s.cfg.Folders
	}

	// Subfolders are only walked at all when some rule asks for them.
	// TODO(M3): walk a subfolder only where such a rule can reach, so a huge
	// node_modules is not listed to be skipped file by file.
	subfolders := false
	for _, r := range s.cfg.Rules {
		if r.Subfolders {
			subfolders = true
			break
		}
	}

	var files []scan.File
	seen := make(map[string]bool, len(folders))
	for _, folder := range folders {
		path := filepath.Clean(config.ExpandHome(folder))
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		listed, err := scan.Folder(path, scan.Options{Subfolders: subfolders})
		if err != nil {
			return plan.Plan{}, err
		}
		files = append(files, listed...)
	}

	env := s.env
	env.Unattended = unattended
	return rules.Build(files, s.cfg, protect.New(s.cfg), env)
}

// fileFor describes one file the way a scan of its folder would.
func fileFor(path string) (scan.File, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return scan.File{}, err
	}
	dir := filepath.Dir(abs)
	files, err := scan.Folder(dir, scan.Options{})
	if err != nil {
		return scan.File{}, err
	}
	for _, f := range files {
		if f.Path == abs {
			return f, nil
		}
	}
	return scan.File{}, fmt.Errorf("%s is not a file in %s", abs, dir)
}
