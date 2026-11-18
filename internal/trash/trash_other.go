//go:build !darwin && !linux && !windows

package trash

import "fmt"

func move(path string) (string, error) {
	return "", fmt.Errorf("trash: not supported on this platform")
}
