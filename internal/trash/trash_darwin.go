//go:build darwin

package trash

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// move puts path in ~/.Trash. The Finder's "Put Back" needs a record macOS only
// writes itself, so undo works from our own journal instead.
func move(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("trash: %w", err)
	}
	trashDir := filepath.Join(home, ".Trash")
	info, err := os.Lstat(trashDir)
	if err != nil {
		return "", fmt.Errorf("trash: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("trash: ~/.Trash is not a folder")
	}
	return renameInto(path, trashDir)
}
