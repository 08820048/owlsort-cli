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

- [x] Config location per OS; `init` writes the file (never overwrites an existing one)
- [x] `internal/scan`: top level, skip rules (hidden, packages, cloud placeholders, downloading)
- [x] `internal/protect`: code projects, Obsidian / Logseq, keep list
- [x] `internal/rules.Build`: all conditions, `match`, `not`, `continue`, settled files, destination checks
- [x] `internal/rules.Explain` using the same code path as `Build`
- [ ] `preview` and `explain`, text and `--json`: `preview` prints text and the documented JSON;
      `explain` prints text only, its JSON shape is not specified yet
- [ ] Fixture trees under `testdata/` for scan and protect; plan snapshots: the trees are built in
      `t.TempDir()` instead, because git refuses to store a folder named `.git` (see `testdata/README.md`)

Done when: on a temporary folder of mixed files, `preview` lists the right moves and the right skips on macOS, Linux and Windows, and `explain` agrees with it for every file.

State: **met** (2026-11-18). The same tests pass on macOS, Linux and Windows in CI, and
`explain` is checked against `preview` file by file, which is the acceptance condition.

## M1: tidy and undo

Goal: tidy, undo, restart the terminal, the history is still there.

- [x] `internal/journal`: append-only file, tolerant reader (a half-written last
      line is closed off before the next entry, so one crash costs one line)
- [x] `internal/apply`: step-by-step run, re-check before each step, conflict strategies, cross-volume move
- [x] `internal/trash` for macOS, Linux, Windows (macOS tested here; the other two by CI)
- [x] `tidy` (confirmation, no-terminal rule), `undo` (checks first), `history`
- [ ] Tests for safety rules 1, 2, 3, 5, 6, 7, 11 — 1, 2, 3, 5, 6, 7 are covered; 11
      needs two volumes (below)

Done when: tidy then undo restores a folder exactly, on all three systems, including after a clash and after the user moved a file in between.

State: **met on all three systems** (2026-11-18): tidy then undo restores a folder
exactly, including after a clash and after a file changed in between. Two things
cannot be checked by a CI runner and are listed here so they are not forgotten:

- **A real cross-volume move** (safety rule 11). Mount something, then
  `owlsort tidy` a file from the other volume; the copy must appear before the
  source disappears.
- **The Windows recycle bin** and **undoing a trashed file there.** Explorer
  keeps its own records, so the journal cannot say where a recycled file went;
  undo reports `needs_foreground` instead of guessing. Read the platform section
  of [architecture.md](architecture.md).

## M2: managing rules

- [x] `rules list/show/enable/disable/edit`, `folders`, `keep`
- [x] `uncovered`: the loose files no rule takes, grouped by extension
- [x] `doctor`: configuration, journal, trash, watched folders
- [x] `rules check`: missing values, bad regexes, shadowed rules, destinations that climb out
- [x] A commented template that `init` writes, generated from `Default` (decisions.md, D13)
- [ ] Shell completion verified for bash, zsh, fish, PowerShell

Done when: `rules check` finds a rule that an earlier one shadows, and a
configuration with a destination that climbs out, without ever reporting a rule
that is reachable.

State: **met** (2026-11-18) except shell completion, which needs a person at four
different shells. The commands that change the configuration edit its text rather
than re-encoding it, so a `rules disable` cannot flatten a file the user has been
curating (decisions.md, D12). `init` writes a commented template for the same
reason (D13).

## M3: watching

- [ ] `watch`: top-level only, coalescing, settle time, unattended rules (safety rule 9)
- [ ] `service install/uninstall/status` for launchd, systemd user units, Windows scheduled tasks
- [ ] Detect the desktop app tidying the same folder; `--force`

## M4: release

- [x] GoReleaser run for real: six archives, checksums, and a CI job that runs it on
      every push so the configuration cannot rot again
- [x] `install.sh`, with the download checked against `checksums.txt` before anything
      is installed, tested end to end against a fake release on every push
- [x] `docs/install.md` and `docs/release.md`; README in English and Chinese
- [ ] macOS: Developer ID signing and notarization — needs a paid Apple Developer
      account, which this project does not have yet
- [ ] Homebrew tap (needs a tap repository), Scoop, winget
- [ ] A man page; a final pass over SECURITY, TRADEMARK, CONTRIBUTING
- [ ] The first tagged release

State (2026-11-19): everything that does not need an account somewhere else is done.
`make release-check` builds all six archives locally and CI runs it on every push; the
installer is tested on Linux and macOS against a fake release, including a corrupted
download and a missing checksums file. What is left needs a person: an Apple
Developer ID to sign with, repositories for the package managers, and the decision to
tag.

## Later

- Import a rule file exported from the desktop apps
- Duplicate files; content-based conditions
- More interface languages
- Finder tags on macOS
- A pipeline of several actions per rule
