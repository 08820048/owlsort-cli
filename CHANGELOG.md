# Changelog

All notable changes are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- The skeleton: command tree (every command and flag, unbuilt ones report their
  milestone), configuration model with validation and the built-in rules, file
  categories, destination variables, interface language, exit codes, plan and
  journal types.
- Policy tests: no network, and nothing removes files outside `internal/apply`
  and `internal/trash`.
- CI on macOS, Linux and Windows; cross-compilation of six targets.
- Documentation: architecture, safety model, configuration, commands, roadmap,
  decisions, and the requirements (Chinese).
