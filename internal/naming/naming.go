// Package naming finds a name that does not collide: "report 2.pdf". It is pure,
// so the rule is written once and the move, the copy and the trash cannot drift
// apart on what a clash looks like.
package naming

import (
	"path/filepath"
	"strconv"
	"strings"
)

// maxAttempts bounds the search. A folder with a thousand files of the same
// name is not a case worth looping forever over.
const maxAttempts = 10000

// Free returns a name that exists() reports as free. The first candidate is name
// itself; then "name 2.ext", "name 3.ext", … The extension is kept, and a name
// with no extension or a dotfile is numbered at the end ("README 2", ".env 2").
//
// exists is called with a full name, so the caller decides what folder it means.
func Free(name string, exists func(string) bool) string {
	if !exists(name) {
		return name
	}
	stem, ext := split(name)
	for i := 2; i < maxAttempts; i++ {
		candidate := stem + " " + strconv.Itoa(i) + ext
		if !exists(candidate) {
			return candidate
		}
	}
	return stem + " " + strconv.Itoa(maxAttempts) + ext
}

// split separates "report.tar.gz" into "report.tar" and ".gz", and leaves a
// dotfile whole: ".env" becomes ".env" and "".
func split(name string) (stem, ext string) {
	base := filepath.Base(name)
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return name, ""
	}
	return name[:len(name)-(len(base)-dot)], base[dot:]
}
