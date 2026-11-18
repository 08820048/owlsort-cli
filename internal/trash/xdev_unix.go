//go:build !windows

package trash

import "syscall"

// errCrossDevice is what the kernel returns when a rename would leave the volume.
var errCrossDevice = syscall.EXDEV
