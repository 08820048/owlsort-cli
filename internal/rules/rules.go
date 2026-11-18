// Package rules turns files and a configuration into a plan (milestone M0).
//
// Contract (docs/architecture.md, "The engine"):
//   - Rules are tried in order; the first enabled rule that matches wins,
//     unless it says `continue`.
//   - A file that is protected (internal/protect) or kept is never planned;
//     it is reported as skipped with the reason.
//   - A file already inside the fixed part of a rule's destination
//     (pathvars.FixedPrefix) is settled: not planned again.
//   - A destination is expanded with pathvars.Expand per file, and must stay
//     inside the watched folder unless it is absolute (absolute destinations
//     are allowed by hand, never unattended).
//   - Pure: no disk access. Same inputs, same plan.
package rules

import (
	"errors"

	"github.com/08820048/owlsort-cli/internal/config"
	"github.com/08820048/owlsort-cli/internal/plan"
	"github.com/08820048/owlsort-cli/internal/protect"
	"github.com/08820048/owlsort-cli/internal/scan"
)

// ErrNotImplemented marks a milestone that has not been built yet.
var ErrNotImplemented = errors.New("not implemented yet")

// Build decides what happens to each file. Milestone M0.
func Build(files []scan.File, cfg config.Config, guard protect.Guard) (plan.Plan, error) {
	return plan.Plan{}, ErrNotImplemented
}

// Explain says why a file is, or is not, going to be tidied: the rule and the
// condition that took it, or the protection that stopped it. Milestone M0.
func Explain(file scan.File, cfg config.Config, guard protect.Guard) (string, error) {
	return "", ErrNotImplemented
}
