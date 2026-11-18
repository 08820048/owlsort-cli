// Package exitcode holds the process exit codes. They are part of the public
// interface (scripts rely on them): see docs/commands.md.
package exitcode

const (
	OK            = 0 // success
	Error         = 1 // any other error
	Usage         = 2 // bad arguments or an invalid configuration
	Partial       = 3 // finished, but a step that was planned did not go through
	NotConfirmed  = 4 // nothing was moved: no confirmation (or no terminal)
	NothingToUndo = 5 // there is no batch to undo
)
