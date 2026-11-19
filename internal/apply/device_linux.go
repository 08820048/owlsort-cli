//go:build linux

package apply

import (
	"os"
	"syscall"
)

// deviceOf is the device a path lives on. Two paths with the same device are on
// the same volume, and a move between them is a rename rather than a copy.
func deviceOf(info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Dev)
	}
	return 0
}

// inodeOf is the file's inode. A rename keeps it; a copy gets a new one. That is
// how a test can tell a move from a copy between volumes instead of assuming.
func inodeOf(info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino)
	}
	return 0
}
