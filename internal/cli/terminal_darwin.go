//go:build darwin

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminalFile asks the kernel whether f is a terminal. Checking the file mode
// is not enough: /dev/null is a character device too, and a run with its stdin on
// /dev/null must be treated as having no one to ask.
func isTerminalFile(f *os.File) bool {
	var termios syscall.Termios
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGETA,
		uintptr(unsafe.Pointer(&termios)), 0, 0, 0,
	)
	return errno == 0
}
