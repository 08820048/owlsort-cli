# Owlsort CLI

**Tidy the loose files in your Downloads and Desktop. Preview first, undo anytime.**

[中文说明](README.zh-CN.md)

> **Status: pre-alpha, design stage.** The command line, configuration format and
> safety rules are specified and the skeleton builds on macOS, Linux and Windows,
> but the commands do not do their work yet (they say which milestone they belong
> to). It is not ready to use. See [docs/roadmap.md](docs/roadmap.md) for where to
> start if you want to help.

`owlsort` moves the screenshots, PDFs, installers and archives that pile up in your
folders into tidy places, by rules you can read and edit. It is built around one
idea: **you see the result before anything moves, and any run can be undone.**

```
$ owlsort preview
Downloads · 6 files can be tidied
  report-final.pdf           → Documents/PDF/
  Screenshot 2026-11-18….png → Pictures/Screenshots/
  Owlsort-0.2.11.dmg         → Installers/
  photos.zip                 → Archives/
Skipped: 2 (in a code project: 1, still downloading: 1)

$ owlsort tidy          # shows the same, asks, then moves
$ owlsort undo          # put the last run back
```

*(Planned output; see [docs/commands.md](docs/commands.md).)*

## Principles

- **Preview first.** `preview` moves nothing; `tidy` asks before it moves anything. Without a terminal it only previews, so a script never moves files by accident.
- **Undo always.** Every step is recorded; `undo` puts a run back, and checks first so it never overwrites anything.
- **Never delete for good.** There is no delete. The heaviest action sends a file to the system trash.
- **Never overwrite.** A name clash keeps both files or skips.
- **Only loose files.** Only the top level of a watched folder is tidied. Your own folders, code projects and notes libraries are left alone.
- **No network.** No telemetry, no update check, no account.
- **Plain configuration.** One TOML file you can read, edit and keep in your dotfiles.

The full list, with the test that proves each, is in [docs/safety-model.md](docs/safety-model.md).

## Platforms

macOS, Linux and Windows, as a single static binary with nothing to install first.

## Documentation

| | |
|---|---|
| [docs/safety-model.md](docs/safety-model.md) | the rules the program must never break |
| [docs/architecture.md](docs/architecture.md) | how it is built, package by package |
| [docs/config.md](docs/config.md) | the configuration file |
| [docs/commands.md](docs/commands.md) | commands, flags, exit codes, JSON output |
| [docs/roadmap.md](docs/roadmap.md) | milestones M0–M4 and what is done |
| [docs/decisions.md](docs/decisions.md) | decisions made, and open questions |
| [docs/requirements.zh-CN.md](docs/requirements.zh-CN.md) | the full requirements (Chinese) |

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md) first. In
short: tests only touch temporary directories, no cgo, no network, and the safety
model comes before any feature.

## Owlsort desktop apps

Owlsort also has desktop apps for Mac and Windows, with a graphical interface, an
animated preview and a desktop robot. They are a separate, paid product at
[owlsort.com](https://owlsort.com). This command line tool is independent of them:
free, open source, with its own code, its own data and its own release numbers.

## License

The code is licensed under the [GNU General Public License v3.0](LICENSE).
The name "Owlsort" and its logo are not covered by that licence; see
[TRADEMARK.md](TRADEMARK.md).
