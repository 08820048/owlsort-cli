//go:build windows

package apply

import (
	"errors"
	"syscall"
)

// isCrossDevice reports whether a failed rename means the two paths are on
// different volumes: ERROR_NOT_SAME_DEVICE (17).
var errNotSameDevice = syscall.Errno(17)

func isCrossDevice(err error) bool { return errors.Is(err, errNotSameDevice) }
