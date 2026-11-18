//go:build !darwin

package scan

import (
	"os"
	"time"
)

// created is the zero time where the platform does not hand us a birth time
// through FileInfo. File.Arrived falls back to the modification time.
//
// TODO(windows): the creation time is available through
// syscall.Win32FileAttributeData, which needs golang.org/x/sys (milestone M1).
func created(os.FileInfo) time.Time { return time.Time{} }
