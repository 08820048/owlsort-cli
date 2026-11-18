//go:build windows

package trash

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const foDelete uint32 = 3

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	shFileOperationW = shell32.NewProc("SHFileOperationW")
	// FOF_ALLOWUNDO is the one that matters: it is what puts the file in the
	// recycle bin instead of deleting it for good.
	fofAllowUndo      uint16 = 0x0040
	fofNoConfirmation uint16 = 0x0010
	fofSilent         uint16 = 0x0004
	fofNoErrorUI      uint16 = 0x0400
)

// shFileOpStruct mirrors SHFILEOPSTRUCTW. The layout is the 64-bit one, which
// is what the release matrix builds (amd64 and arm64).
type shFileOpStruct struct {
	hwnd                 uintptr
	wFunc                uint32
	from                 *uint16
	to                   *uint16
	flags                uint16
	anyOperationsAborted int32
	nameMappings         uintptr
	progressTitle        *uint16
}

// move asks the Shell to put path in the recycle bin, so Explorer can restore it.
func move(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("trash: %w", err)
	}
	absolute = strings.ReplaceAll(absolute, "/", `\`)
	from, err := syscall.UTF16FromString(absolute)
	if err != nil {
		return "", fmt.Errorf("trash: %w", err)
	}
	// The list of paths ends with a second NUL.
	from = append(from, 0)

	op := shFileOpStruct{
		wFunc: foDelete,
		from:  &from[0],
		flags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}
	if r, _, callErr := shFileOperationW.Call(uintptr(unsafe.Pointer(&op))); r != 0 {
		return "", fmt.Errorf("trash: the shell refused to recycle %s (code %d, %v)", path, r, callErr)
	}
	if op.anyOperationsAborted != 0 {
		return "", fmt.Errorf("trash: recycling %s was aborted", path)
	}
	if _, err := os.Lstat(path); err == nil {
		return "", fmt.Errorf("trash: %s is still there after recycling it", path)
	}
	// The recycle bin keeps its own records, so the path it went to is only
	// known to Explorer. Undo therefore cannot restore it (see docs/roadmap.md).
	return "", nil
}
