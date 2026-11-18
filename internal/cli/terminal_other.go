//go:build !darwin && !linux && !windows

package cli

import "os"

// isTerminalFile answers no on a platform this build does not know: with no way
// to ask, `tidy` previews and stops, which is the safe answer.
func isTerminalFile(*os.File) bool { return false }
