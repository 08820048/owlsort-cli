# Roadmap

Each milestone ends with something that can be run and checked. Pick up the
first unchecked item. `docs/safety-model.md` says which tests each safety rule needs.

## Done: the skeleton

- [x] Repository, module, Makefile, CI on macOS / Linux / Windows, cross-compile of six targets
- [x] Command tree: every command and flag exists, unbuilt ones say which milestone
- [x] `internal/config`: TOML model, parsing, validation, built-in rules (tested)
- [x] `internal/category`, `internal/pathvars`, `internal/i18n`, `internal/exitcode`
- [x] `internal/plan` and `internal/journal` types (the JSON shapes)
- [x] Policy tests: no network, nothing removes files outside `apply` and `trash`
- [x] Documents: architecture, safety model, config, commands

## M0: preview

Goal: `owlsort init && owlsort preview` is correct on all three systems.

- [ ] Config location per OS; `init` writes the file (never overwrites an existing one)
- [ ] `internal/scan`: top level, skip rules (hidden, packages, cloud placeholders, downloading)
- [ ] `internal/protect`: code projects, Obsidian / Logseq, keep list
- [ ] `internal/rules.Build`: all conditions, `match`, `not`, `continue`, settled files, destination checks
- [ ] `internal/rules.Explain` using the same code path as `Build`
- [ ] `preview` and `explain`, text and `--json`
- [ ] Fixture trees under `testdata/` for scan and protect; plan snapshots

Done when: on a temporary folder of mixed files, `preview` lists the right moves and the right skips on macOS, Linux and Windows, and `explain` agrees with it for every file.

## M1: tidy and undo

Goal: tidy, undo, restart the terminal, the history is still there.

- [ ] `internal/journal`: append-only file, tolerant reader
- [ ] `internal/apply`: step-by-step run, re-check before each step, conflict strategies, cross-volume move
- [ ] `internal/trash` for macOS, Linux, Windows
- [ ] `tidy` (confirmation, no-terminal rule), `undo` (checks first), `history`
- [ ] Tests for safety rules 1, 2, 3, 5, 6, 7, 11

Done when: tidy then undo restores a folder exactly, on all three systems, including after a clash and after the user moved a file in between.

## M2: managing rules

- [ ] `rules list/show/enable/disable/edit/check`, `folders`, `keep`, `uncovered`, `doctor`
- [ ] `rules check`: missing values, bad regexes, shadowed rules, destinations that climb out
- [ ] Shell completion verified for bash, zsh, fish, PowerShell

## M3: watching

- [ ] `watch`: top-level only, coalescing, settle time, unattended rules (safety rule 9)
- [ ] `service install/uninstall/status` for launchd, systemd user units, Windows scheduled tasks
- [ ] Detect the desktop app tidying the same folder; `--force`

## M4: release

- [ ] GoReleaser run for real: archives, checksums, changelog
- [ ] macOS: Developer ID signing and notarization
- [ ] Homebrew tap, Scoop, winget, install script
- [ ] README in English and Chinese, man page, SECURITY, TRADEMARK, CONTRIBUTING final pass

## Later

- Import a rule file exported from the desktop apps
- Duplicate files; content-based conditions
- More interface languages
- Finder tags on macOS
- A pipeline of several actions per rule
