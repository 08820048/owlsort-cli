// Package protect decides which files must not be touched (milestone M0).
//
// What is protected (docs/safety-model.md):
//   - Anything inside a code project: a folder with .git, package.json,
//     go.mod, Cargo.toml, pyproject.toml, *.sln, Package.swift and the like.
//     Build output inside a project is not protected.
//   - Anything inside a notes library: an Obsidian vault (.obsidian/), a
//     Logseq graph (logseq/config.edn).
//   - Anything the user listed with `owlsort keep` or `keep = [...]`.
//   - Subfolders of a watched folder, unless a rule asks for them.
package protect

import "github.com/08820048/owlsort-cli/internal/plan"

// Guard answers whether a path may be touched.
type Guard interface {
	// Protected returns the reason a path must be left alone, or "" if it may be touched.
	Protected(path string) plan.SkipReason
}
