package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultPath is where the configuration lives when --config is not given:
// $XDG_CONFIG_HOME/owlsort/config.toml on macOS and Linux (~/.config/... when
// the variable is unset), %APPDATA%\owlsort\config.toml on Windows.
func DefaultPath() (string, error) {
	if runtime.GOOS == "windows" {
		dir := os.Getenv("APPDATA")
		if dir == "" {
			return "", errors.New("config: APPDATA is not set")
		}
		return filepath.Join(dir, "owlsort", "config.toml"), nil
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "owlsort", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: %w", err)
	}
	return filepath.Join(home, ".config", "owlsort", "config.toml"), nil
}

// WriteNew writes data to path, creating the folder. It never overwrites: `init`
// must not destroy a configuration the user has edited.
func WriteNew(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("config: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("config: %s already exists; left untouched", path)
		}
		return fmt.Errorf("config: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("config: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// ExpandHome replaces a leading "~" with the user's home folder. A path that
// does not start with "~" is returned unchanged, so it is safe to call twice.
func ExpandHome(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") && !strings.HasPrefix(p, `~\`) {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	return filepath.Join(home, p[2:])
}
