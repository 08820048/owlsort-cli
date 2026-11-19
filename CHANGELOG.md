# Changelog

All notable changes are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- macOS binaries signed with a Developer ID and notarized by Apple.
- A release that refuses to publish unless both darwin archives are signed and
  accepted by Gatekeeper.
- `owlsort init`, `preview` and `explain`.
- `owlsort tidy`, `undo` and `history`.
- `owlsort rules check`, `uncovered` and `doctor`.
- `owlsort rules list`, `show`, `enable`, `disable` and `edit`.
- `owlsort folders list`, `add` and `remove`.
- `owlsort keep list`, `add` and `remove`.
- JSON output for every command, including `explain`.
- A `conflict` strategy on a single rule, overriding the file's.
- `install.sh`, which checks the download against the release's checksums first.
- Scanning that leaves code projects, notes libraries, hidden files, cloud
  placeholders and files still being written alone.
- Moving, copying, renaming and trashing, with the system trash on macOS, Linux and
  Windows.
- An append-only journal that `undo` reads backwards.
- A commented configuration file that `init` writes.
- `docs/install.md` and `docs/release.md`.
- CI that builds the release and tests the installer on every push.
- A release published from a `v` tag.

### Changed

- The commands that change the configuration edit its text, so comments and order
  survive.
- The release notes are the `CHANGELOG.md` entry, not a generated changelog.
