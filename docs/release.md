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
4. On a Mac, download a darwin archive **in a browser** and run it. That is the path
   Gatekeeper judges; `curl` sets no quarantine flag, so an installer test proves
   nothing about it.

## macOS signing and notarization

`.goreleaser.yaml` signs the darwin binaries with a Developer ID and notarizes them
through Apple's `notarytool`, so a download is not blocked by Gatekeeper. The
`Release` workflow **refuses to publish without it**: an unsigned macOS binary is
unusable for anyone who downloads it in a browser, and shipping one while the README
promises otherwise is worse than not shipping.

### The five secrets

Add these under **Settings → Secrets and variables → Actions**. Nothing about them
belongs in the repository, in a commit message, or in a chat window.

| Secret | What it is |
|---|---|
| `MACOS_SIGN_P12` | base64 of the **Developer ID Application** certificate, exported as `.p12` from Keychain Access |
| `MACOS_SIGN_PASSWORD` | the password you set on that `.p12` when exporting it |
| `MACOS_NOTARY_KEY_ID` | the Key ID of an App Store Connect API key |
| `MACOS_NOTARY_ISSUER_ID` | the Issuer ID shown above the key list in App Store Connect |
| `MACOS_NOTARY_KEY` | base64 of the `AuthKey_XXXX.p8` file that key downloaded |

Getting them:

```sh
# 1. Is the certificate there, and is it the right kind?
security find-identity -v -p codesigning
#    It must say "Developer ID Application: Your Name (TEAMID)".
#    "Mac App Distribution" and "Apple Development" are the wrong kind: they cannot
#    sign something for distribution outside the App Store.

# 2. Export it: Keychain Access → My Certificates → right-click the Developer ID
#    Application certificate → Export → .p12, set a password. Then:
base64 -i DeveloperID.p12 | pbcopy

# 3. An API key: App Store Connect → Users and Access → Integrations →
#    App Store Connect API → Keys → +, role "Developer". The .p8 downloads once.
base64 -i AuthKey_XXXXXXXXXX.p8 | pbcopy
```

An App Store Connect **API key** is used rather than an Apple ID with an
app-specific password because it does not need two-factor authentication in CI and
does not expire on a password change.

### What proves it worked

The workflow does not take "the release succeeded" as evidence. After the build it
runs `scripts/verify-macos-signature.sh` on both darwin archives, which asks macOS:

1. which certificate signed this binary (`codesign -dv`) — it must be a Developer
   ID Application, and it must not be ad-hoc;
2. whether it carries a Team Identifier — ad-hoc signatures have none;
3. whether Gatekeeper would run it (`spctl --assess`) — only true when Apple has
   notarized it and the ticket can be found.

`codesign --verify` alone is **not** used as the check: it passes on an unsigned Go
binary, because Go ad-hoc signs darwin binaries so they can run at all. That trap is
why the guard is a tested script (`make signature-test`) rather than three lines of
YAML, and the test asserts the trap: it builds an unsigned binary, confirms
`codesign --verify` accepts it, and confirms the script does not.

### What has not been tested

The signing path itself. A Developer ID certificate and an Apple notarization round
trip are not in CI, so **the first signed release is the first real test of it**.
Run the release, then read the verification step's output; if notarization is
rejected, the workflow fails before the release page is finished.

If a notarization is rejected for a reason that takes a while to fix, delete the
draft release and the tag (`git push --delete origin v0.1.0`), fix it, and tag again.
A tag is cheap; a release page people have already downloaded from is not.

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
