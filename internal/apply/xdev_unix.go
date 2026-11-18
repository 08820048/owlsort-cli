//go:build !windows

package apply

import (
	"errors"
	"syscall"
)

// isCrossDevice reports whether a failed rename means the two paths are on
// different volumes, which is the one case that needs copy, verify, remove.
func isCrossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }
