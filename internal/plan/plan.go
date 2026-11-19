// Package plan is what the engine hands to the executor: a list of moves (or
// copies, renames, trashes) decided but not yet done. `preview` prints it,
// `tidy` carries it out. Nothing in this package touches the disk.
package plan

import "time"

// Action is what happens to a file.
type Action string

const (
	Move   Action = "move"
	Copy   Action = "copy"
	Rename Action = "rename"
	Trash  Action = "trash"
)

// SkipReason says why a file is left where it is. The values are part of the
// JSON output (docs/commands.md); do not rename them.
type SkipReason string

const (
	SkipInCodeProject   SkipReason = "in_code_project"
	SkipInNotesLibrary  SkipReason = "in_notes_library"
	SkipKept            SkipReason = "kept"
	SkipDownloading     SkipReason = "downloading"
	SkipHidden          SkipReason = "hidden"
	SkipConflict        SkipReason = "conflict"
	SkipNeedsForeground SkipReason = "needs_foreground" // trash or similar, not allowed unattended
	SkipChanged         SkipReason = "changed"          // not what the preview saw any more
)

// Text is how a skip reason is said to a person. It lives here so `preview`,
// `explain` and any later report cannot word the same reason differently.
func (r SkipReason) Text() string {
	switch r {
	case SkipInCodeProject:
		return "in a code project"
	case SkipInNotesLibrary:
		return "in a notes library"
	case SkipKept:
		return "kept"
	case SkipDownloading:
		return "still downloading"
	case SkipHidden:
		return "hidden"
	case SkipConflict:
		return "a name clash"
	case SkipNeedsForeground:
		return "needs a foreground run"
	case SkipChanged:
		return "changed since the preview"
	}
	return string(r)
}

// Item is one decided step.
type Item struct {
	Source      string `json:"source"`
	Destination string `json:"destination,omitempty"` // empty for trash
	Action      Action `json:"action"`
	Rule        string `json:"rule"`

	// Conflict is the strategy this step's rule asked for, or "" to use the
	// file's default. It is not part of the --json shape.
	Conflict string `json:"-"`

	// Size and Modified are what the preview saw. The executor compares them
	// with the file right before it moves it, so a file that changed in the
	// meantime is skipped instead of moved (docs/safety-model.md, rule 7).
	// They are not part of the --json shape.
	Size     int64     `json:"-"`
	Modified time.Time `json:"-"`
}

// Skipped is a file the engine looked at and left alone.
type Skipped struct {
	Path   string     `json:"path"`
	Reason SkipReason `json:"reason"`
	Rule   string     `json:"rule,omitempty"`
}

// Folder is the plan for one watched folder.
type Folder struct {
	Path    string    `json:"path"`
	Items   []Item    `json:"items"`
	Skipped []Skipped `json:"skipped"`
}

// Version is the version of the --json shape in docs/commands.md. Bump it when
// a field is renamed or removed; adding a field does not need a bump.
const Version = 1

// Plan is the plan for every folder asked about.
type Plan struct {
	Version int      `json:"version"`
	Folders []Folder `json:"folders"`
}

// Count is the number of steps across all folders.
func (p Plan) Count() int {
	n := 0
	for _, f := range p.Folders {
		n += len(f.Items)
	}
	return n
}
