# Commands

Every command is wired into `owlsort --help` already; those marked with a
milestone return "not built yet" until that milestone lands
([roadmap.md](roadmap.md)). This page is the specification they are built to.

## Global flags

| | |
|---|---|
| `--json` | machine-readable output (shapes below) |
| `--config <file>` | configuration file |
| `--lang <en\|zh-Hans>` | interface language; default from `LC_ALL`, `LC_MESSAGES`, `LANG` |
| `-q`, `--quiet` | print only what matters |
| `--no-color` | no colors; also honours `NO_COLOR`, and colors are off when stdout is not a terminal |

Command names and flags are always English. Only output text and default folder
names are translated.

## Exit codes

A public interface; scripts rely on them (`internal/exitcode`).

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | any other error |
| 2 | bad arguments, or an invalid or unreadable configuration |
| 3 | finished, but some files were skipped |
| 4 | nothing was moved: not confirmed, or no terminal and no `--yes` |
| 5 | nothing to undo |

## Commands

### Tidying

| Command | |
|---|---|
| `owlsort init [--folders a,b]` | write the config with the built-in rules (M0) |
| `owlsort preview [path...]` | what would be tidied and where it would go; moves nothing (M0) |
| `owlsort tidy [path...] [-y]` | show the preview, ask, then tidy (M1) |
| `owlsort undo [--batch id]` | put back the newest batch, or the one given (M1) |
| `owlsort history [--limit 20]` | one line per run: id, time, files, where to (M1) |
| `owlsort explain <file>` | the rule and condition that would take it, or the protection that stops it (M0) |
| `owlsort uncovered [path...]` | files no rule covers, grouped by extension (M2) |

`tidy` without a terminal and without `--yes` only previews and exits 4.

### Setup

| Command | |
|---|---|
| `owlsort rules list \| show <name> \| enable <name> \| disable <name>` | (M2) |
| `owlsort rules edit` | open the config in `$EDITOR`, validate on save (M2) |
| `owlsort rules check` | missing values, bad regexes, rules shadowed by earlier ones, destinations that climb out (M2) |
| `owlsort folders list \| add <path> \| remove <path>` | watched folders; removing moves nothing (M2) |
| `owlsort keep list \| add <path> \| remove <path>` | never touched (M2) |
| `owlsort doctor` | config readable, folders exist, trash works, permissions (M2) |
| `owlsort completion bash\|zsh\|fish\|powershell` | shell completion (free from cobra) |

### Background

| Command | |
|---|---|
| `owlsort watch [path...] [--force]` | foreground monitor; tidies a file once it has settled (M3) |
| `owlsort service install \| uninstall \| status` | launchd / systemd user unit / scheduled task running `watch` (M3) |

`watch` does not start if the desktop app is tidying the same folder in the
background, unless `--force`.

## Output

### `preview`, for people

```
Downloads · 6 files can be tidied
  report-final.pdf           → Documents/PDF/
  Screenshot 2026-11-18….png → Pictures/Screenshots/
  Owlsort-0.2.11.dmg         → Installers/
  photos.zip                 → Archives/
  …
Skipped: 2 (in a code project: 1, still downloading: 1)
Run `owlsort tidy` to move them. Every run can be undone with `owlsort undo`.
```

### `preview --json` (draft; version 1)

The shape is `internal/plan.Plan`. Skip reasons are a fixed set; do not rename them.

```json
{
  "version": 1,
  "folders": [
    {
      "path": "/Users/me/Downloads",
      "items": [
        { "source": "/Users/me/Downloads/report-final.pdf",
          "destination": "/Users/me/Downloads/Documents/PDF/report-final.pdf",
          "action": "move", "rule": "PDFs" }
      ],
      "skipped": [
        { "path": "/Users/me/Downloads/app/build.zip", "reason": "in_code_project" }
      ]
    }
  ]
}
```

Skip reasons: `in_code_project`, `in_notes_library`, `kept`, `downloading`,
`hidden`, `conflict`, `needs_foreground`, `changed`.

### The journal

Each step is one JSON line (`internal/journal.Entry`), documented in
[architecture.md](architecture.md).
