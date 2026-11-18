//go:build windows

package trash

import "syscall"

// errCrossDevice is ERROR_NOT_SAME_DEVICE (17).
var errCrossDevice = syscall.Errno(17)
