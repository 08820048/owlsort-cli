# Configuration

`owlsort init` writes the file with the built-in rules, already laid out and
commented. `rules enable|disable`, `folders` and `keep` change it for you; they edit
the text, so your own comments and ordering survive. The model, parsing and validation live in `internal/config`, which is the
source of truth; this page describes it.

## Location

| | |
|---|---|
| macOS, Linux | `$XDG_CONFIG_HOME/owlsort/config.toml` (default `~/.config/owlsort/config.toml`) |
| Windows | `%APPDATA%\owlsort\config.toml` |
| anywhere | `--config <file>` |

A file that cannot be read, or is invalid, is an **error** (exit code 2). The
program never falls back to the built-in rules to tidy with
([safety-model.md](safety-model.md), rule 12).

Unknown keys are errors too, so a typo (`move-to`) is caught instead of ignored.

## Top level

```toml
folders  = ["~/Downloads", "~/Desktop"]   # watched folders; "~" is the home folder
keep     = ["~/Downloads/Taxes"]          # never touched: files or folders
conflict = "rename"                       # "rename" (default) or "skip"
language = ""                             # "", "en" or "zh-Hans"; empty follows LANG
```

## Rules

Rules are tried in order. The first enabled rule whose conditions hold takes the
file, unless it sets `continue = true`.

```toml
[[rule]]
name    = "Screenshots"          # required, unique
enabled = true                   # default true
match   = "all"                  # "all" (default) or "any"
when = [                         # at least one condition
  { category = "image" },
  { name_contains = ["Screenshot", "截屏"] },
]
move_to = "Pictures/Screenshots" # exactly one action (below)
```

| Key | |
|---|---|
| `name` | required, unique; what `rules enable <name>` takes |
| `enabled` | `false` switches a rule off without deleting it |
| `match` | `"all"` every condition, `"any"` one is enough |
| `when` | list of conditions |
| `subfolders` | also tidy files below the top level (default `false`) |
| `background` | may run unattended in `watch`; `trash` rules need it |
| `continue` | after this rule, keep trying later rules (default: stop) |

### Actions: exactly one

| Key | What it does |
|---|---|
| `move_to = "folder"` | move into a folder (relative to the watched folder, or absolute) |
| `copy_to = "folder"` | copy into a folder |
| `rename = "template"` | rename in place |
| `trash = true` | send to the system trash. There is no delete |

A relative destination that contains `..` is rejected. An absolute destination is
allowed when run by hand and is never run unattended.

### Variables in destinations and rename templates

| | |
|---|---|
| `{date}` | `2026-11-18`: the day the file arrived (creation time where the OS keeps it, else modification time) |
| `{year}` `{month}` `{day}` | `2026` `11` `18` |
| `{category}` | `Images`, `Documents`, … (in the interface language) |
| `{ext}` | the extension, lower case |

A filled-in value is always a single path segment; it can never carry a `/` or
`..`. A file already inside the fixed part of a destination (`Archive` in
`Archive/{date}/{category}`) is considered filed and is not picked up again.

### Conditions

Each condition tests **one** thing. Add `not = true` to invert it.

| Key | Value | True when |
|---|---|---|
| `ext` | `["pdf", "docx"]` | the extension is one of these |
| `category` | `image` `document` `video` `audio` `archive` `code` `other` | it is that kind |
| `name_equals` | list | the name is exactly one of them |
| `name_contains` | list | the name contains one (case-insensitive) |
| `name_starts_with` | list | the name starts with one |
| `name_ends_with` | list | the name ends with one |
| `name_regex` | list | the name matches one |
| `larger_than` | `"10MB"` | bigger (`B KB MB GB TB`) |
| `smaller_than` | `"500KB"` | smaller |
| `modified_within_days` | `7` | changed in the last N days |
| `modified_older_than_days` | `30` | not changed for N days |
| `created_within_days` | `7` | created in the last N days |
| `created_older_than_days` | `30` | created more than N days ago |
| `in_folder` | `"~/Desktop"` | the file is directly in that folder |
| `path_contains` | list | the full path contains one of them |

## The built-in rules

In priority order (`internal/config/default.go`). Rules that move many kinds of
file start **off**.

| Rule | On | Moves | To |
|---|---|---|---|
| Desktop by date | off | files on the Desktop | `Desktop Archive/{date}/{category}` |
| Screenshots | on | images named like screenshots | `Pictures/Screenshots` |
| Screen recordings | off | videos named like recordings | `Videos/Screen Recordings` |
| PDFs | on | `.pdf` | `Documents/PDF` |
| Installers | on | `dmg pkg mpkg iso exe msi apk deb rpm appimage` | `Installers` |
| Archives | on | zip, rar, 7z, … | `Archives` |
| Office documents | off | docx, xlsx, pptx, csv, txt, md, … | `Documents/Office` |
| Other pictures | off | any other image | `Pictures/Other` |
| Audio and video | off | mp4, mov, mp3, wav, … | `Media` |

Screenshots come before other pictures, recordings before general video, and
installers before archives, so the specific rule gets the file first. Folder
names are localised (`zh-Hans`: `图片/截图`, `文档/PDF`, …).

## Not planned for v0.1

Multiple actions per rule (a pipeline), running scripts, content-based
conditions, and importing the desktop apps' rule files. See
[roadmap.md](roadmap.md), "Later".
