# Releasing

A release is a tag someone decides to push. Nothing else publishes anything: CI
runs on every commit and never uploads.

## Why it is a checklist and not a script

Most of it is one command. The parts that are not are the parts that need a person:
whether this is the moment to release, what the notes say, and whether the
binaries can be trusted by a stranger's machine. Those do not belong in a script
that can run unattended.

## Before the tag

1. `make check` — vet, tests, gofmt. CI runs the same on three systems; green there
   is the real signal.
2. `go test ./... -count=1` once more. The cache hides a test that only passed
   because nothing changed.
3. **Run it on a folder you have a backup of.** Not a test — a real folder, with
   `preview`, then `tidy`, then `undo`, and check the folder is exactly as it was.
   No amount of unit tests replaces this before a release.
4. Write the `CHANGELOG.md` entry: **one sentence per change, what was done.** Not
   why, not an example, not the implementation. Move it out of `[Unreleased]` under
   the version and the date, and leave a fresh `[Unreleased]` at the top.
5. Check nothing was left in `[Unreleased]` from an earlier version that has
   already shipped. A published entry is never edited; a new change is a new
   version.
6. `make snapshot` — a local, unsigned build of every platform. Catches a
   `.goreleaser.yaml` mistake before it is in front of users.
7. Read the built binary's version: `./dist/owlsort --version` style, from an
   unpacked archive, and check it is the version you are about to tag.

## The tag

```sh
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

The `Release` workflow does the rest: vet, tests, six builds, archives, checksums,
and a check that every archive `install.sh` looks for is actually on the release
page. If the tag and the tree disagree, the workflow fails before uploading.

Watch it, and read the release page afterwards. Do not assume it worked.

## After the release

1. Download one archive by hand on a clean machine (or a fresh container) and run
   `owlsort --version` and `owlsort init && owlsort preview`.
2. Run the installer the documented way:
   `curl -fsSL …/install.sh | sh`. The checksum step is what makes this safe; if it
   prints a mismatch, stop and pull the release.
3. Check the release notes read the way they should: one sentence per change.

## Not signed yet, and what that costs

The macOS binaries are **not signed or notarized**. A binary downloaded through a
browser is blocked by Gatekeeper until the user allows it. `docs/install.md` says so
plainly and gives the two honest ways round it (allow it once, or build from
source). This is a real limitation, not a detail to hide in a footnote.

To fix it, this is what is needed, and then what to add to `.goreleaser.yaml`:

| Step | Needs |
|---|---|
| Sign the darwin binaries | An Apple Developer ID Application certificate (paid account) |
| Notarize them | An App Store Connect API key (`notarytool`), and the signed zip as the input |
| Staple the ticket | `xcrun stapler`, after notarization, before archiving |

Until then, the release notes say it, and `docs/install.md` says it.

## Homebrew, Scoop and winget

Each needs a repository or an account that does not exist yet. In order of value:

1. **Homebrew tap** — `brew install 08820048/owlsort/owlsort`. Needs the tap
   repository to exist and a `HOMEBREW_TAP_TOKEN` secret with write access. Then
   `brews:` goes into `.goreleaser.yaml`.
2. **Scoop bucket** — the same shape, a `scoops:` stanza and its own token.
3. **winget** — a pull request to `microsoft/winget-pkgs` per version. It can be
   automated later; the first one is by hand.

None of these can sit in `.goreleaser.yaml` disabled: `goreleaser check` rejects a
stanza it cannot resolve, and CI would go red for a feature that is simply not
turned on yet. That is why they are listed here and not there.
