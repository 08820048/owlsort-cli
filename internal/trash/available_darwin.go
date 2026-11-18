//go:build darwin

package trash

import (
	"fmt"
	"os"
	"path/filepath"
)

// available reports whether ~/.Trash is there and is a folder.
func available() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("trash: %w", err)
	}
	dir := filepath.Join(home, ".Trash")
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("trash: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("trash: %s is not a folder", dir)
	}
	return nil
}
