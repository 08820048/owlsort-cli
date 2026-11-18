//go:build linux

package trash

import (
	"fmt"
	"os"
	"path/filepath"
)

// available reports whether the freedesktop trash folder can be written to. A
// missing folder is fine, `move` creates it; a folder that cannot be written to
// is not.
func available() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("trash: %w", err)
	}
	base := filepath.Join(home, ".local", "share", "Trash")
	if _, err := os.Stat(base); err != nil {
		// It will be created on the first move if the parent is writable.
		parent := filepath.Dir(base)
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return fmt.Errorf("trash: %w", err)
		}
		return nil
	}
	files := filepath.Join(base, "files")
	if info, err := os.Stat(files); err == nil && !info.IsDir() {
		return fmt.Errorf("trash: %s is not a folder", files)
	}
	if err := os.MkdirAll(files, 0o700); err != nil {
		return fmt.Errorf("trash: %w", err)
	}
	return nil
}
