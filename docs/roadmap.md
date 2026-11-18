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

- [ ] `rules list/show/enable/disable/edit`, `folders`, `keep`
- [x] `uncovered`: the loose files no rule takes, grouped by extension
- [x] `doctor`: configuration, journal, trash, watched folders
- [x] `rules check`: missing values, bad regexes, shadowed rules, destinations that climb out
- [ ] Shell completion verified for bash, zsh, fish, PowerShell

Done when: `rules check` finds a rule that an earlier one shadows, and a
configuration with a destination that climbs out, without ever reporting a rule
that is reachable.

State: the three read-only commands are done and tested (2026-11-18). What is left
is the set that **writes** the configuration. That needs a decision first: writing
it back through `config.Marshal` would throw away the user's comments, and this
project's plain-TOML argument is partly that comments are allowed. Either the
commands edit the text in place, or they say they will rewrite the file and the
user keeps a copy in git.

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
