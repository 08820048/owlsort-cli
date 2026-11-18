// Package atomicfile replaces a file's contents in one step.
//
// It exists so that writing a file is never half-done: the new bytes go to a
// temporary file in the same folder, are flushed, and are then renamed over the
// target. A crash leaves the old file, not an empty new one, and a reader never
// sees a partial write.
//
// This is one of the only three packages allowed to remove a file — the others
// are apply and trash (docs/safety-model.md, rule 4). The reason is narrow and
// worth being exact about: the only file it ever removes is the temporary one it
// created itself a moment earlier in the destination folder. It never removes,
// and never sees, a file the user put there. Anything broader belongs in
// internal/apply or internal/trash, where it can be reviewed on its own.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write replaces path with data and gives it perm. The temporary file is created
// in the same folder as path, so the rename cannot cross a volume.
func Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".owlsort-*.tmp")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	name := tmp.Name()

	// discard removes the temporary file. It is the only removal here, and it is
	// of a file this call just made.
	discard := func(cause error) error {
		tmp.Close()
		os.Remove(name)
		return fmt.Errorf("write %s: %w", path, cause)
	}

	if _, err := tmp.Write(data); err != nil {
		return discard(err)
	}
	if err := tmp.Chmod(perm); err != nil {
		return discard(err)
	}
	// Flush before the rename: otherwise a crash can leave the new name pointing
	// at bytes that never reached the disk.
	if err := tmp.Sync(); err != nil {
		return discard(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
