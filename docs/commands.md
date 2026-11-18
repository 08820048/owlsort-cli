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

Exit code 3 is about the *run*, not the *plan*: a `.DS_Store` that the preview
said it would leave alone is not a skipped step, but a file the strategy refused
to overwrite is. The preview is where the exclusions are reported; the exit code
is what a script checks after asking for the work.

## Exit codes

A public interface; scripts rely on them (`internal/exitcode`).

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | any other error |
| 2 | bad arguments, or an invalid or unreadable configuration |
| 3 | finished, but a step that was planned did not go through: a clash the strategy left alone, a file that changed after the preview, or one that failed |
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
| `owlsort uncovered [path...]` | files no rule covers, grouped by extension |

`tidy` without a terminal and without `--yes` only previews and exits 4.

### Setup

| Command | |
|---|---|
| `owlsort rules list \| show <name> \| enable <name> \| disable <name>` | see and switch the rules |
| `owlsort rules edit` | open the config in `$VISUAL`/`$EDITOR`, check it when the editor exits |
| `owlsort rules check` | missing values, bad regexes, rules shadowed by earlier ones, destinations that climb out (M2) |
| `owlsort folders list \| add <path> \| remove <path>` | watched folders; removing moves nothing |
| `owlsort keep list \| add <path> \| remove <path>` | never touched |
| `owlsort doctor` | config readable, folders exist, journal writable, trash works |
| `owlsort completion bash\|zsh\|fish\|powershell` | shell completion (free from cobra) |

### Background

| Command | |
|---|---|
| `owlsort watch [path...] [--force]` | foreground monitor; tidies a file once it has settled (M3) |
| `owlsort service install \| uninstall \| status` | launchd / systemd user unit / scheduled task running `watch` (M3) |

`watch` does not start if the desktop app is tidying the same folder in the
background, unless `--force`.

## Output

### Changing the configuration

`rules enable`, `rules disable`, `folders add|remove` and `keep add|remove` edit the
configuration file **as text**: your comments, your order, your quoting and your
spacing stay exactly as they were (decisions.md, D12). Before anything is written
the result is parsed again, and the write is refused if the file would not load or
would not hold the same rules. The file is replaced in one step, and a symlink — a
config kept in dotfiles — is followed rather than replaced.

- `rules enable`/`disable` are idempotent: a rule that is already in that state
  writes nothing and says so.
- `folders add` refuses a folder that is not there, and `keep add` refuses a path
  that is not there: the list only matters if the program can look at what it names,
  and a typo would otherwise watch — or protect — nothing, silently.
- Both compare paths as paths, so adding `/Users/me/Downloads` when `~/Downloads`
  is already listed says "already there" instead of adding a second entry.
- `folders remove` and `keep remove` move nothing; they only change what the program
  looks at.

### `rules edit`

Opens the file in `$VISUAL`, else `$EDITOR`, else `vi`/`nano`, and checks it the
moment the editor exits — an editor's save cannot be hooked, so that is the closest
"validate on save" gets. Your edit is never thrown away: a file that no longer loads
is reported and left as you saved it.

### `rules check`

One line per problem, then a summary. A **mistake** (something the configuration
may not say) exits with code 1; a **warning** (a rule no file can reach) does not,
because the configuration is valid and the program runs.

```
error: rule "Broken", condition 1: name_regex "(": error parsing regexp: missing closing ): `(`
warning: rule "Documents" can never take a file: rule "PDFs" above it takes every file it
would. Move "Documents" above "PDFs", or set `continue = true` on "PDFs".

config.toml: 1 error, 1 warning.
```

It reads the configuration even when it is invalid — reporting one problem and
hiding the rest would be the opposite of its job — so it parses without validating
(`config.ParseLoose`) and then lists everything `config.Problems` and the shadowing
analysis find. In `--json`, an array of `{severity, rule, message}`.

A rule is never called shadowed on a guess: only when an earlier rule can be shown
to take every file it would ([decisions.md](decisions.md), D11).

### `uncovered`

The loose files no rule takes, grouped by extension, most first. Protected, hidden
and still-downloading files are **not** listed: the plan already explains those,
and suggesting a rule for them would be wrong advice. Exits with code 3 when it
finds any, 0 when it finds none.

```
6 loose file(s) no rule covers, by extension:

  weird            3 files  e.g. another.weird, notes.weird, thing.weird
  no extension     1 file   e.g. README
  docx             1 file   e.g. x.docx

Add a rule to a copy of the built-in set, or turn on one of the rules that ship off.
```

`--json` is an array of `{extension, count, examples}`.

### `doctor`

One line per check, then a verdict. Exits with code 1 when something would stop
the program from working.

```
ok   configuration    /Users/me/.config/owlsort/config.toml, 9 rule(s)
ok   journal          /Users/me/.local/state/owlsort/journal.jsonl (will be created)
warn system trash     trash: ~/.Trash is not a folder (no rule uses the trash, so this does not stop anything)
ok   watched folder   ~/Downloads (43 file(s) at the top level)

Ready. `owlsort preview` would tidy files from here.
```

The trash is only a **failure** when some enabled rule sends a file to it; without
such a rule it is a warning, because reporting a failure for something the
configuration never uses sends the user after a problem they do not have.
`doctor` changes nothing: it creates no folder and writes no journal, so it can be
run to find out what is wrong. `--json` is `{config_path, journal_path, checks:
[{name, detail, ok, warning}]}`.

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

### `tidy` and `undo` `--json` (draft; version 1)

Both print the same shape: what the run did, step by step.

```json
{
  "batch": "20261118-142503-a1b2",
  "done": 2,
  "skipped": [ { "path": "/Users/me/Downloads/a.pdf", "reason": "conflict", "rule": "PDFs" } ],
  "failed": 0
}
```

A step that was tried and did not work also appears, with the error:

```json
{ "failed": 1,
  "failures": [ { "path": "/Users/me/Downloads/b.pdf", "rule": "PDFs", "error": "apply: ..." } ] }
```

The skip reasons are the same fixed set as `preview`.

### `history` `--json` (draft; version 1)

One object per run, newest first (`internal/journal.Run`):

```json
[
  {
    "batch": "20261118-142503-a1b2",
    "time": "2026-11-18T14:25:03Z",
    "moved": 2,
    "to": ["/Users/me/Downloads/Documents/PDF"],
    "undone": true,
    "undo_done": 2
  }
]
```

### The journal

Each step is one JSON line (`internal/journal.Entry`), documented in
[architecture.md](architecture.md).
