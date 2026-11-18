//go:build windows

package trash

// move is the windows implementation. Milestone M1.
func move(path string) (string, error) { return "", ErrNotImplemented }
