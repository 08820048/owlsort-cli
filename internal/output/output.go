// Package output prints results: for people (aligned text, color when the
// output is a terminal) and for scripts (--json). Every command goes through
// here so the two forms stay in step.
//
// Colors are off when stdout is not a terminal, when NO_COLOR is set, or with
// --no-color.
package output

import (
	"encoding/json"
	"io"
)

// Options are the global flags that shape output.
type Options struct {
	JSON    bool
	Quiet   bool
	NoColor bool
}

// JSON writes v as indented JSON, followed by a newline.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
