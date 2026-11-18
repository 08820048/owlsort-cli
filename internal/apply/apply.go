// Package apply carries a plan out.
//
// Contract (docs/safety-model.md):
//   - Every step is re-checked right before it runs: the source is still
//     there and unchanged since the preview, the destination folder is
//     creatable. A step that fails the check is skipped with a reason; the
//     rest go on.
//   - Never overwrite. A clash is resolved by the rule's conflict strategy
//     (rename "name 2.ext", or skip).
//   - A step is written to the journal before the next one starts.
//   - Moves across volumes copy, verify the size, then remove the source.
//   - Undo walks a batch backwards and, for each step, puts the file back only
//     if it is still at the destination and the original place is free.
package apply

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/journal"
	"github.com/08820048/owlsort-cli/internal/naming"
	"github.com/08820048/owlsort-cli/internal/plan"
	"github.com/08820048/owlsort-cli/internal/trash"
)

// hashLimit is the size below which a cross-volume copy is verified with a hash
// as well as a byte count. A large file is compared by size only: reading it
// twice would double the cost of moving a film.
const hashLimit = 1 << 20

// Options shape a run.
type Options struct {
	// Conflict is the configuration's default strategy for a name clash.
	Conflict config.Conflict
	// Now pins the batch id, so a test can assert on it.
	Now time.Time
}

func (o Options) withDefaults() Options {
	if o.Conflict == "" {
		o.Conflict = config.ConflictRename
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	return o
}

// Result of a run.
type Result struct {
	Batch   string
	Done    int
	Skipped []plan.Skipped
	Failed  int
	// Failures are the steps that were tried and did not work, with the reason.
	Failures []Failure
}

// Failure is a step that did not go through because something went wrong.
type Failure struct {
	Path string
	Rule string
	Err  error
}

// Run carries out p and records it in j.
func Run(p plan.Plan, j journal.Journal, opts Options) (Result, error) {
	opts = opts.withDefaults()
	res := Result{Batch: journal.NewBatchID(opts.Now)}

	for _, folder := range p.Folders {
		for _, item := range folder.Items {
			entry := journal.Entry{
				Version: journal.EntryVersion,
				Batch:   res.Batch,
				Time:    time.Now(),
				Op:      string(item.Action),
				Source:  item.Source,
				Dest:    item.Destination,
				Rule:    item.Rule,
			}

			step := runStep(item, opts)
			final, skip, err := step.destination, step.skip, step.err
			switch {
			case skip != "":
				entry.Result = journal.Skipped
				entry.Error = string(skip)
				res.Skipped = append(res.Skipped, plan.Skipped{
					Path: item.Source, Reason: skip, Rule: item.Rule,
				})
			case err != nil:
				entry.Result = journal.Failed
				entry.Error = err.Error()
				res.Failed++
				res.Failures = append(res.Failures, Failure{Path: item.Source, Rule: item.Rule, Err: err})
			default:
				entry.Result = journal.OK
				entry.Dest = final
				entry.Created = step.created
				res.Done++
			}

			// The line goes in before the next step starts: a crash loses at
			// most the step in flight (docs/architecture.md, "The journal").
			if jerr := j.Append(entry); jerr != nil {
				return res, fmt.Errorf("apply: %w", jerr)
			}
		}
	}
	return res, nil
}

// stepOutcome is what running one step came to.
type stepOutcome struct {
	destination string          // where the file ended up
	created     string          // the topmost folder the step had to make
	skip        plan.SkipReason // set when the step was left alone on purpose
	err         error           // set when it was tried and did not work
}

// runStep re-checks one step and performs it.
func runStep(item plan.Item, opts Options) stepOutcome {
	// Rule 7: the file may have changed between the preview and now.
	info, statErr := os.Lstat(item.Source)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return stepOutcome{skip: plan.SkipChanged}
		}
		return stepOutcome{err: fmt.Errorf("apply: %w", statErr)}
	}
	if !item.Modified.IsZero() && (!info.ModTime().Equal(item.Modified) || info.Size() != item.Size) {
		return stepOutcome{skip: plan.SkipChanged}
	}

	if item.Action == plan.Trash {
		target, err := trash.Move(item.Source)
		if err != nil {
			return stepOutcome{err: err}
		}
		return stepOutcome{destination: target}
	}

	if item.Destination == "" {
		return stepOutcome{err: errors.New("apply: a step has no destination")}
	}

	// Never overwrite: a clash is resolved by the strategy, and both strategies
	// leave every existing file alone.
	target, ok, err := resolve(item.Destination, opts.Conflict)
	if err != nil {
		return stepOutcome{err: err}
	}
	if !ok {
		return stepOutcome{skip: plan.SkipConflict}
	}

	// Remember what did not exist yet, so undo can take it away again.
	created := topmostMissing(filepath.Dir(target))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return stepOutcome{err: fmt.Errorf("apply: %w", err)}
	}

	switch item.Action {
	case plan.Move, plan.Rename:
		if err := movePath(item.Source, target); err != nil {
			return stepOutcome{created: created, err: err}
		}
	case plan.Copy:
		if err := copyVerify(item.Source, target); err != nil {
			return stepOutcome{created: created, err: err}
		}
	default:
		return stepOutcome{err: fmt.Errorf("apply: unknown action %q", item.Action)}
	}
	return stepOutcome{destination: target, created: created}
}

// topmostMissing is the highest folder in dir's chain that does not exist yet,
// or "" when dir is already there. It is what a step had to create.
func topmostMissing(dir string) string {
	top := ""
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(d); err == nil {
			return top
		}
		top = d
		parent := filepath.Dir(d)
		if parent == d {
			return top
		}
	}
}

// removeEmptyFolders takes away the folders a step created, as long as they are
// still empty: an undo puts the folder back the way it was.
func removeEmptyFolders(from, stop string) {
	if stop == "" {
		return
	}
	for d := from; ; d = filepath.Dir(d) {
		if err := os.Remove(d); err != nil {
			return // still in use by something the user put there
		}
		if d == stop {
			return
		}
		parent := filepath.Dir(d)
		if parent == d {
			return
		}
	}
}

// resolve decides the final path for a step. ok is false when the strategy is
// to skip a clash.
func resolve(destination string, conflict config.Conflict) (string, bool, error) {
	if _, err := os.Lstat(destination); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return destination, true, nil
		}
		return "", false, fmt.Errorf("apply: %w", err)
	}
	if conflict == config.ConflictSkip {
		return "", false, nil
	}
	dir := filepath.Dir(destination)
	name := naming.Free(filepath.Base(destination), func(candidate string) bool {
		_, err := os.Lstat(filepath.Join(dir, candidate))
		return err == nil
	})
	return filepath.Join(dir, name), true, nil
}

// Undo puts back a batch ("" is the newest). Steps are undone newest first; a
// step whose file has moved on or whose original place is taken is skipped and
// reported, so nothing is ever overwritten.
func Undo(batch string, j journal.Journal, opts Options) (Result, error) {
	opts = opts.withDefaults()
	if batch == "" {
		batches, err := j.Batches()
		if err != nil {
			return Result{}, err
		}
		if len(batches) == 0 {
			return Result{}, nil
		}
		batch = batches[0]
	}
	entries, err := j.Entries(batch)
	if err != nil {
		return Result{}, err
	}

	res := Result{Batch: batch}
	for i := len(entries) - 1; i >= 0; i-- {
		step := entries[i]
		// Only a step that went through has anything to put back, and an undo
		// line is not itself undone.
		if step.Result != journal.OK || step.Op == journal.OpUndo {
			continue
		}

		skip, err := undoStep(step)
		entry := journal.Entry{
			Version: journal.EntryVersion,
			Batch:   batch,
			Time:    time.Now(),
			Op:      journal.OpUndo,
			Source:  step.Dest,
			Dest:    step.Source,
			Rule:    step.Rule,
		}
		switch {
		case skip != "":
			entry.Result = journal.Skipped
			entry.Error = string(skip)
			res.Skipped = append(res.Skipped, plan.Skipped{Path: step.Source, Reason: skip, Rule: step.Rule})
		case err != nil:
			entry.Result = journal.Failed
			entry.Error = err.Error()
			res.Failed++
			res.Failures = append(res.Failures, Failure{Path: step.Source, Rule: step.Rule, Err: err})
		default:
			entry.Result = journal.OK
			res.Done++
		}
		if jerr := j.Append(entry); jerr != nil {
			return res, fmt.Errorf("apply: %w", jerr)
		}
	}
	return res, nil
}

// undoStep reverses one step: put the file back where it was, if it is still
// where we left it and the original place is free.
func undoStep(step journal.Entry) (skip plan.SkipReason, err error) {
	switch step.Op {
	case journal.OpCopy:
		// A copy left the original in place, so undoing it means removing the
		// copy — after checking it is still the same size, so a file the user
		// has since replaced at that path is not destroyed.
		info, statErr := os.Lstat(step.Dest)
		if statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				return plan.SkipChanged, nil
			}
			return "", fmt.Errorf("apply: %w", statErr)
		}
		if original, err := os.Lstat(step.Source); err == nil && original.Size() != info.Size() {
			return plan.SkipChanged, nil
		}
		if err := os.Remove(step.Dest); err != nil {
			return "", fmt.Errorf("apply: %w", err)
		}
		return "", nil

	case journal.OpMove, journal.OpRename, journal.OpTrash:
		if step.Dest == "" {
			// The Windows recycle bin keeps its own records; there is no path to
			// move the file back from.
			return plan.SkipNeedsForeground, nil
		}
		if _, err := os.Lstat(step.Dest); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return plan.SkipChanged, nil
			}
			return "", fmt.Errorf("apply: %w", err)
		}
		// Rule 6: the original place must be free. Undo never overwrites.
		if _, err := os.Lstat(step.Source); err == nil {
			return plan.SkipConflict, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("apply: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(step.Source), 0o755); err != nil {
			return "", fmt.Errorf("apply: %w", err)
		}
		if err := movePath(step.Dest, step.Source); err != nil {
			return "", err
		}
		// The folder the run made is empty now, so take it away: an undo should
		// leave no trace.
		removeEmptyFolders(filepath.Dir(step.Dest), step.Created)
		return "", nil
	}
	return plan.SkipChanged, nil
}

// movePath moves a file or folder, copying across volumes: the copy is verified
// before the source is removed (safety rule 11).
func movePath(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if !isCrossDevice(err) {
		return fmt.Errorf("apply: %w", err)
	}
	if err := copyVerify(src, dst); err != nil {
		return err
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("apply: moved to %s but could not remove the original: %w", dst, err)
	}
	return nil
}

// copyVerify copies src to dst and checks the result: the size always, and a
// hash for a small file. A failed check removes the copy and leaves the source
// exactly where it was.
func copyVerify(src, dst string) error {
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("apply: %s already exists", dst)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	defer in.Close()

	// O_EXCL: never overwrite, not even in the middle of a cross-volume move.
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	hasher := sha256.New()
	var sink io.Writer = out
	hash := false
	if size, err := in.Stat(); err == nil && size.Size() <= hashLimit {
		sink = io.MultiWriter(out, hasher)
		hash = true
	}
	written, copyErr := io.Copy(sink, in)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(dst)
		return fmt.Errorf("apply: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(dst)
		return fmt.Errorf("apply: %w", closeErr)
	}

	if err := verify(src, dst, written, hasher, hash); err != nil {
		os.Remove(dst)
		return err
	}
	return nil
}

// verify compares the copy with the source and removes the copy when it does not
// match. Removing our own half-written copy is the only deletion this package
// does that is not a move.
func verify(src, dst string, written int64, hasher interface{ Sum([]byte) []byte }, hashed bool) error {
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	destInfo, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	if sourceInfo.Size() != written || destInfo.Size() != written {
		return fmt.Errorf("apply: the copy of %s is %d bytes, the original is %d", src, destInfo.Size(), sourceInfo.Size())
	}
	if !hashed {
		return nil
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	want, err := hashFile(src)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("apply: the copy of %s does not match the original", src)
	}
	return nil
}

// hashFile is the sha256 of a file, as hex.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("apply: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("apply: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
