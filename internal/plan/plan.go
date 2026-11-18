// Package plan is what the engine hands to the executor: a list of moves (or
// copies, renames, trashes) decided but not yet done. `preview` prints it,
// `tidy` carries it out. Nothing in this package touches the disk.
package plan

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

// Item is one decided step.
type Item struct {
	Source      string `json:"source"`
	Destination string `json:"destination,omitempty"` // empty for trash
	Action      Action `json:"action"`
	Rule        string `json:"rule"`
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

// Plan is the plan for every folder asked about.
type Plan struct {
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
