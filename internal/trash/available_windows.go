//go:build windows

package trash

import "fmt"

// available reports whether the Shell exports SHFileOperationW, which is what
// puts a file in the recycle bin.
func available() error {
	if err := shFileOperationW.Find(); err != nil {
		return fmt.Errorf("trash: the shell does not offer SHFileOperationW: %w", err)
	}
	return nil
}
