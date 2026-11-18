# Changelog

All notable changes are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

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
