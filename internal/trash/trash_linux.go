//go:build linux

package trash

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// move follows the freedesktop.org trash specification: the file goes into
// ~/.local/share/Trash/files and a matching ~/.local/share/Trash/info/*.trashinfo
// records where it came from, so the desktop's own "Restore" works too.
func move(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("trash: %w", err)
	}
	base := filepath.Join(home, ".local", "share", "Trash")
	files := filepath.Join(base, "files")
	info := filepath.Join(base, "info")
	for _, dir := range []string{files, info} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("trash: %w", err)
		}
	}

	target := freePath(files, filepath.Base(path))
	// The .trashinfo goes first: a file in the trash without one cannot be
	// restored, and a stray .trashinfo for a file that never moved is harmless.
	record := filepath.Join(info, filepath.Base(target)+".trashinfo")
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("trash: %w", err)
	}
	body := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		escape(absolute), time.Now().Format("2006-01-02T15:04:05"))
	if err := os.WriteFile(record, []byte(body), 0o600); err != nil {
		return "", fmt.Errorf("trash: %w", err)
	}
	if err := os.Rename(path, target); err != nil {
		os.Remove(record) // the file did not move, so the record must not stay
		if isCrossDevice(err) {
			return "", fmt.Errorf("trash: %s is on another volume than %s; move it there by hand first", path, files)
		}
		return "", fmt.Errorf("trash: %w", err)
	}
	return target, nil
}

// escape percent-encodes a path the way the specification wants it.
func escape(path string) string {
	const hex = "0123456789ABCDEF"
	var b []byte
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c > 0x20 && c < 0x7f && c != '%' {
			b = append(b, c)
			continue
		}
		b = append(b, '%', hex[c>>4], hex[c&0x0f])
	}
	return string(b)
}
