# Contributing

Thank you for helping. Owlsort moves people's files, so a few rules matter more
than speed.

## Before you start

1. Read [docs/safety-model.md](docs/safety-model.md). A change that weakens a rule
   there needs a decision recorded in [docs/decisions.md](docs/decisions.md), not
   only a review.
2. Read [docs/architecture.md](docs/architecture.md) and pick an unchecked item in
   [docs/roadmap.md](docs/roadmap.md). Open an issue first for anything large.
3. Read [AGENTS.md](AGENTS.md): the working conventions in one page.

## Setup

You need Go (see `go.mod` for the version). Nothing else.

```sh
git clone https://github.com/08820048/owlsort-cli.git
cd owlsort-cli
make check      # vet, tests, gofmt: what CI runs
make build      # ./owlsort
make cross      # prove all six platform builds compile
```

## The rules of the repository

- **Tests only touch temporary directories** (`t.TempDir()`). Never the real
  Downloads, Desktop or Documents, not even "just to try", not in CI.
- **No cgo.** Every platform must build from one machine with `CGO_ENABLED=0`.
- **No network.** `internal/policy` fails the build if a network package is imported.
- **No new dependency** without saying why in the pull request.
- **Only `internal/apply` and `internal/trash` remove files**, and never for good
  (`internal/policy` checks this too).
- **Platform code** goes in `_<os>.go` files with build tags, behind an interface in
  a neutral file. An unsupported platform returns an error, never a silent substitute.
- **Pure core**: `rules` and `plan` do not read the disk or the clock; pass them in.
- Output for people goes through `internal/output`, and `--json` must stay in step
  with it. Exit codes and JSON field names are a public interface: changing one is
  a breaking change.

## Pull requests

- Keep them focused; one concern each.
- Add or update tests, and docs if a command, flag, exit code or config key changed.
- Follow the checklist in the pull request template.
- Commit messages: a short imperative summary, then why.

## A bug that lost or misplaced a file

Is the most important kind. Say so at the top of the issue, include the output of
`owlsort history`, and run `owlsort undo` before anything else if you can.

## Security issues

Do not open a public issue; see [SECURITY.md](SECURITY.md).
