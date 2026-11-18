//go:build !darwin && !linux && !windows

package trash

import "fmt"

// available says so: a platform with no trash implementation cannot tidy files,
// and `doctor` must not pretend it can.
func available() error { return fmt.Errorf("trash: not supported on this platform") }
