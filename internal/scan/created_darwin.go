//go:build darwin

package scan

import (
	"os"
	"syscall"
	"time"
)

// created is the file's birth time. macOS keeps it, so {date} uses the day the
// file landed rather than the day it was last written.
func created(info os.FileInfo) time.Time {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(st.Birthtimespec.Sec, st.Birthtimespec.Nsec)
	}
	return time.Time{}
}
