# Changelog

All notable changes are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- A rule may set its own `conflict`, overriding the file's. The safety model always
  said "the rule's `conflict` decides", but only a file-wide one existed — and an
  unknown key is an error, so following the document was blocked by the loader
  (decisions.md, D14).
- `explain --json`: `{path, planned, rule, action, destination}` when the file would be
  tidied, with `skip`, `filed` or `needs_foreground` saying why when it would not.

- `rules list`, `rules show`, `rules enable`, `rules disable` and `rules edit`: see the
  rules, switch one off without deleting it, or open the file in `$EDITOR` and have it
  checked when the editor exits.
- `folders add|remove` and `keep add|remove`: change which folders are watched and which
  paths are never touched. A path that is not there is refused, because a typo would
  otherwise watch (or protect) nothing, silently.
- These commands edit the configuration **as text**, so comments, order and spacing
  survive (`internal/tomledit`). Every write is re-parsed first and refused if the file
  would not load or would not hold the same rules; the replacement is atomic and a
  symlink is followed rather than replaced (decisions.md, D12).
- `owlsort init` now writes a commented template in the documented style instead of
  `config.Marshal` output, generated from the built-in rules and held to them by a test
  (decisions.md, D13).
- `internal/atomicfile`: replace a file in one step, so a write is never half done.

- `rules check`: every problem with a configuration, one per line — a condition that
  tests two things, a bad regex, a size that is not a size, a destination that climbs
  out, a name used twice, and a rule no file can reach because an earlier one already
  takes every file it would. A mistake exits 1; a warning does not, because the
  configuration is valid. Reads a broken configuration on purpose, so one mistake does
  not hide the rest.
- `uncovered`: the loose files no rule takes, grouped by extension with a few examples.
  Protected, hidden and downloading files are left out — the plan already explains
  them, and a rule for them would be wrong advice.
- `doctor`: configuration readable and valid, journal writable, watched folders present
  and readable, system trash usable. Creates nothing, so it can be run to find out
  what is wrong. The trash is only a failure when a rule actually uses it.
- `config.Problems`: validation reports one problem at a time with the rule it belongs
  to, instead of one long joined error; `config.ParseLoose` parses without validating.

- M1: `tidy`, `undo` and `history`. `tidy` shows the plan and asks; with no terminal
  and no `--yes` it only previews and exits 4. Every step is re-checked right before
  it runs (source still there, same size and modification time), a clash is resolved
  by the configuration's strategy and never overwrites, and each step is written to
  an append-only JSON Lines journal before the next one starts. `undo` walks a batch
  backwards, puts a file back only if it is still where the run left it and the
  original place is free, and takes away the empty folders the run made.
- `internal/trash`: `~/.Trash` on macOS, the freedesktop.org spec on Linux, the Shell
  recycle bin (`SHFileOperationW` with `FOF_ALLOWUNDO`) on Windows. No permanent delete.
- `internal/naming`: `report 2.pdf`, shared by the move, the copy and the trash.
- `internal/apply`: cross-volume moves copy, verify the size (and a hash for files up
  to 1 MB), and only then remove the source.
- The terminal is asked of the kernel rather than guessed from the file mode, so a
  `tidy` with its stdin on `/dev/null` counts as having no one to ask.

- M0: `init`, `preview` and `explain` do their work. `init` writes the configuration at
  the platform's location and never overwrites one; `preview` scans the watched folders
  and prints the plan, as text or as the documented JSON, moving nothing; `explain`
  reports the rule or the protection that decides one file, through the same code path.
- `internal/scan`: top level only, packages as one file, hidden files marked, cloud
  placeholders and files still being written recognised, symbolic links never followed.
- `internal/protect`: code projects, Obsidian / Logseq libraries and the keep list.
- `internal/rules`: `Build` and `Explain` — conditions, `match`, `not`, `continue`,
  settled files, destination checks, absolute destinations and trash rules only by hand.
- `internal/output`: the text plan, with the skipped summary, and JSON for scripts.

- The skeleton: command tree (every command and flag, unbuilt ones report their
  milestone), configuration model with validation and the built-in rules, file
  categories, destination variables, interface language, exit codes, plan and
  journal types.
- Policy tests: no network, and nothing removes files outside `internal/apply`
  and `internal/trash`.
- CI on macOS, Linux and Windows; cross-compilation of six targets.
- Documentation: architecture, safety model, configuration, commands, roadmap,
  decisions, and the requirements (Chinese).
