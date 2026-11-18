# Working on this repository

For anyone picking this up, human or AI assistant. One page.

## What this is

`owlsort`: an open-source, free command line tool that tidies the loose files in
Downloads and Desktop. Independent of the paid Owlsort desktop apps: no shared
code, no shared data, no feature parity required. Written in Go, no cgo, no network.

## Read first, in this order

1. [docs/safety-model.md](docs/safety-model.md): what must never break.
2. [docs/architecture.md](docs/architecture.md): packages and contracts.
3. [docs/roadmap.md](docs/roadmap.md): **start at the first unchecked item of the earliest milestone (M0).**
4. [docs/config.md](docs/config.md) and [docs/commands.md](docs/commands.md): the specs.
5. [docs/decisions.md](docs/decisions.md): what is decided, what is open.

## State of the code

- **Done and tested:** `internal/config` (model, validation, built-in rules, default path,
  never-overwrite write), `category`, `pathvars`, `i18n`, `exitcode`, `policy`, `scan`,
  `protect`, `rules` (M0), `output` (text preview + JSON), and `init` / `preview` / `explain`.
- **Contracts written, code to do:** `apply`, `journal`, `trash` (M1); `watch`, `service`
  (M3). Each file starts with a comment that is the contract to build to. Stubs return
  `ErrNotImplemented`.
- The remaining commands print "not built yet (planned for M?)" until their milestone lands.

## Commands

```sh
make check    # vet + test + gofmt. Run before every commit.
make build    # ./owlsort
make cross    # six platform builds; proves nothing needs cgo or a platform API by accident
```

## Rules that are easy to break by accident

- **Never run the program, or a test, against the real Downloads, Desktop or
  Documents.** Use `t.TempDir()` or a throwaway folder under `/tmp`. This applies to
  you, an assistant, too: do not "try it" on the user's real folders.
- **No cgo, no network, no deletion outside `internal/apply` and `internal/trash`.**
  `go test ./internal/policy` enforces the last two.
- **Do not weaken a rule in `docs/safety-model.md`** to make a test or a feature
  easier. Record a decision instead.
- **A bad config stops everything.** Never fall back to the built-in rules.
- **Keep `rules` and `plan` pure:** no disk, no clock, no environment.
- **Exit codes and JSON field names are a public interface.**
- **Do not commit** real configs, journals or file lists from a test run.

## Adding things

- A **condition**: add the field to `config.Condition` and to `validate`, to the
  table in `docs/config.md`, to the matcher in `rules`, with table tests.
- An **action**: the same, plus how `apply` performs it, how it is journaled and how
  `undo` reverses it. Anything irreversible is not allowed.
- A **command**: in `internal/cli`, wired in `New`, documented in `docs/commands.md`;
  logic goes in a package, not in the command.
- A **platform behaviour**: a `_<os>.go` file with a build tag behind an interface.

## The names

Repository `owlsort-cli`, command `owlsort`. The name collision check against
Homebrew, Scoop and winget is still to do (decisions.md, Q1).
