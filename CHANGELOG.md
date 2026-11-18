# Changelog

All notable changes are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

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
