//go:build windows

package cli

import (
	"os"
	"syscall"
)

// isTerminalFile asks the console whether f is one. A redirected stdin is not.
func isTerminalFile(f *os.File) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(f.Fd()), &mode) == nil
}
