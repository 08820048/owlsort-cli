# Architecture

Status: the layout and the contracts below are decided; the engine behind most
of them is not written yet (see [roadmap.md](roadmap.md)). Where a package is a
stub, its `doc.go`-style comment at the top of the file is the contract to
build to.

## Principles

1. **Safe by construction.** The program cannot delete for good, cannot overwrite,
   and does not move anything without showing it first. These are properties of
   the code paths, not of options. See [safety-model.md](safety-model.md).
2. **Pure core, thin edges.** Deciding what to do (rules → plan) is pure and
   does no I/O. Everything that touches the disk (scan, apply, trash, journal) is
   at the edge, behind small interfaces, and is tested against temporary directories.
3. **One binary, no runtime, no cgo.** Every platform builds from one machine with
   `CGO_ENABLED=0`. Anything that seems to need cgo is done with `golang.org/x/sys`
   or left out.
4. **No network.** No telemetry, no update check, no licence check.
5. **Independent of the desktop apps.** Owlsort's desktop apps are separate
   products; this repository shares no code with them and does not read their data.

## Data flow

```
config.toml ──► config ──┐
                         ▼
 watched folders ─► scan ─► files ─► rules.Build ──► plan ──► preview  (print only)
                              ▲            │                    │
                       protect.Guard ──────┘                    ▼
                                                              apply ──► journal
                                                                │          ▲
                                                           trash / fs      │
                                                                           undo
```

- `scan` lists files. `protect` says which must not be touched. `rules` turns
  files + config into a `plan.Plan`. `preview` prints the plan. `apply` carries it
  out, step by step, writing each step to the `journal`. `undo` reads the journal
  backwards.
- `explain` runs the same `rules` logic for one file and reports the reason
  instead of a plan, so it can never disagree with `preview`.

## Packages

| Package | Role | State |
|---|---|---|
| `cmd/owlsort` | `main`: builds the command tree, maps errors to exit codes | done |
| `internal/cli` | cobra commands and flags; parse, call, print; **no logic** | `init`, `preview`, `explain`, `tidy`, `undo`, `history`, `uncovered`, `doctor`, `rules check` done; the rest are stubs |
| `internal/config` | the TOML model, parsing, validation, built-in rules | **done**, tested |
| `internal/category` | extension → category (`image`, `document`, …) | **done**, tested |
| `internal/pathvars` | `{date}` `{year}` … expansion; `FixedPrefix` | **done**, tested |
| `internal/i18n` | language resolution; category names | **done**, tested |
| `internal/exitcode` | exit codes (a public interface) | done |
| `internal/plan` | `Plan`, `Item`, `Skipped`, skip reasons (the JSON shape) | types done |
| `internal/scan` | list a folder's files | **done**, tested |
| `internal/protect` | code projects, notes libraries, the keep list | **done**, tested |
| `internal/rules` | `Build`, `Explain`, `Uncovered` | **done**, tested |
| `internal/apply` | run a plan, undo a batch | **done**, tested |
| `internal/journal` | append-only JSON Lines record | **done**, tested |
| `internal/trash` | system trash per platform (`trash_<os>.go`) | **done**: macOS tested here, Linux and Windows by CI |
| `internal/watch` | foreground monitor | M3 |
| `internal/service` | launchd / systemd / scheduled task | M3 |
| `internal/output` | tables for people, JSON for scripts | text preview and JSON done; explain has no JSON shape yet |
| `internal/naming` | a name that does not collide (`report 2.pdf`) | **done**, tested |
| `internal/check` | what is wrong with a configuration (`rules check`) | **done**, tested |
| `internal/doctor` | will this work on this machine | **done**, tested |

Dependency direction: `cli → {rules, apply, scan, journal, output, config}`;
`rules → {config, plan, protect, scan, pathvars, category}`; leaf packages
(`category`, `i18n`, `exitcode`, `plan`) import nothing of ours. `rules` and
`plan` must never import `os` for anything that touches files.

## The engine (`rules`)

- Rules are tried in order; the first **enabled** rule whose conditions hold
  wins, unless it sets `continue = true`.
- `match = "all"` (default) needs every condition; `"any"` needs one. A
  condition with `not = true` is inverted.
- A file is **never planned** if `protect.Guard` reports it, or if it is in the
  keep list. It is reported in `Folder.Skipped` with the reason.
- A file already inside the fixed part of a rule's destination
  (`pathvars.FixedPrefix`) is **settled**: not planned again. This is what stops
  `Archive/{date}/{category}` from re-filing its own output.
- A destination is expanded per file. A relative destination must stay inside
  the watched folder (`..` is rejected at config time and again when planning); an
  absolute destination is allowed when run by hand and never unattended.
- **Pure and deterministic**: the same files and config give the same plan. The
  clock and time zone are passed in, never read inside.

## Scanning

- Top level only, unless a rule has `subfolders = true`. Subfolders are taken to
  be the user's own finished work.
- A macOS package (`.app`, `.bundle`) is one file and is never opened.
- Cloud placeholders (macOS `.icloud`, a Windows OneDrive online-only file) are
  **not read**: reading one would download it.
- Hidden files are listed but marked, so the plan can say why it skipped them.
- Symbolic links are not followed out of the watched folder.
- "Downloading" = names such as `.crdownload`, `.part`, `.download`, `.tmp`, or a
  file that changed in the last few seconds.

## The journal

An append-only JSON Lines file, one line per step (`internal/journal.Entry`).

- Append only. An undo is a new line.
- A line is written **before** the next step starts, so a crash loses at most the
  step in flight.
- A line that cannot be parsed is skipped with a warning and never blocks undo.
- It contains file paths. The README says so.

Location: `$XDG_STATE_HOME/owlsort/journal.jsonl` (default `~/.local/state/owlsort/`)
on macOS and Linux; `%LOCALAPPDATA%\owlsort\journal.jsonl` on Windows.

A step also records `created`, the topmost folder it had to make. Undo takes that
folder away again when it is empty, so a run and its undo leave the watched folder
exactly as it was rather than with a trail of empty subfolders.

## Platforms

| | macOS | Linux | Windows |
|---|---|---|---|
| Trash | move into `~/.Trash`, renaming on a clash; Finder's "Put Back" is unavailable, undo uses the journal | freedesktop trash spec (`~/.local/share/Trash/files` + `.trashinfo`) so the desktop's "Restore" works | Shell recycle bin (`SHFileOperationW`, `FOF_ALLOWUNDO`) so Explorer can restore |
| Path case | by volume (APFS is case-insensitive by default) | sensitive | insensitive |
| Not read | `.icloud` placeholders, package contents | hidden files | OneDrive online-only files, hidden and system files |
| Background | launchd user agent | systemd user unit | scheduled task at logon |
| Watching | `fsnotify` (kqueue) | `fsnotify` (inotify) | `fsnotify` (ReadDirectoryChangesW) |

Platform code lives in `_<os>.go` files with build tags, behind an interface in a
platform-neutral file. A platform that is not supported returns an error; it never
silently does something else.

## Dependencies

Kept to a minimum on purpose; adding one needs a reason in the PR.

| | Why |
|---|---|
| `github.com/spf13/cobra` | commands, flags, help, shell completion |
| `github.com/BurntSushi/toml` | the config file |
| `github.com/fsnotify/fsnotify` | watching (M3) |
| `golang.org/x/sys` | Windows trash, extended attributes (M1) |

## Testing approach

- Pure functions (rules, variables, categories, validation): table tests.
- Anything that touches files: real operations in `t.TempDir()`, on all three
  operating systems in CI.
- Commands: `--json` output and exit codes as snapshots.
- **Tests never touch the real Downloads, Desktop or Documents.** Not in CI, not
  locally.
- Cross-platform behaviour that cannot be tested on one OS (the recycle bin) is
  isolated behind an interface so everything else is.
