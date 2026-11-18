//go:build darwin

package trash

// move is the darwin implementation. Milestone M1.
func move(path string) (string, error) { return "", ErrNotImplemented }
