# Installing

One binary, nothing to install first. Choose the way that fits your machine.

## The installer

```sh
curl -fsSL https://raw.githubusercontent.com/08820048/owlsort-cli/main/install.sh | sh
```

It downloads the archive for your machine, checks it against the release's
`checksums.txt`, and only then puts the binary in place. A download that does not
match is thrown away and nothing is installed.

| Flag | |
|---|---|
| `--version v0.1.0` | a specific version (default: the newest release) |
| `--dir ~/.local/bin` | where to install (default: `/usr/local/bin`, else `~/.local/bin`) |

Set `OWLSORT_BASE_URL` to download from a mirror instead of GitHub. That is also
how [`scripts/test-install.sh`](../scripts/test-install.sh) tests the installer
against a fake release, on every push, on Linux and macOS.

The script never reads its own standard input, so `curl … | sh` is safe: a `read`
there would swallow the rest of the script on some shells, and the result would be a
half-run installer.

## Build from source

Needs Go (see `go.mod` for the version). Nothing else: no cgo, no C toolchain.

```sh
go install github.com/08820048/owlsort-cli/cmd/owlsort@latest
```

This is also the way round the macOS Gatekeeper problem below.

## macOS: how a release will be signed

**No version has been released yet**, so there is no binary to download: build it
from source (below). When there is one, it will be signed with a Developer ID and
notarized by Apple, and the release workflow refuses to publish a version that is
not ([release.md](release.md)). The signing is set up and its guard is tested; what
is missing is the certificate in this repository's secrets, and the first release to
use it.

If a build you have does get blocked by Gatekeeper, it is not signed — check that it
came from the releases page of this repository and that the checksum matches. Then
either:

- **Build it yourself**, which is always fine and needs only Go:

  ```sh
  go install github.com/08820048/owlsort-cli/cmd/owlsort@latest
  ```

- **Allow the one you have**, after checking the checksum against the release page:

  ```sh
  shasum -a 256 ~/Downloads/owlsort_0.1.0_darwin_arm64.tar.gz
  xattr -d com.apple.quarantine /usr/local/bin/owlsort
  ```

Note that `install.sh` uses `curl`, which sets no quarantine flag, so the installer
path is never blocked. Only a download through a browser is.

## Windows

Download the `.zip` from the release page and put `owlsort.exe` somewhere on your
`PATH`. The `.tar.gz` archives are for macOS and Linux.

## What it writes

| | macOS, Linux | Windows |
|---|---|---|
| Configuration | `$XDG_CONFIG_HOME/owlsort/config.toml` (usually `~/.config/…`) | `%APPDATA%\owlsort\config.toml` |
| Journal | `$XDG_STATE_HOME/owlsort/journal.jsonl` (usually `~/.local/state/…`) | `%LOCALAPPDATA%\owlsort\journal.jsonl` |

Both are plain text. The journal holds the file paths of what was moved — that is
what makes `undo` work, and it is why it is worth knowing the file exists.

Nothing else is written. No telemetry, no cache, no update check: the program opens
no network connection at all, and a test fails if any source file imports one.

## Uninstalling

```sh
rm "$(command -v owlsort)"
rm -rf ~/.config/owlsort ~/.local/state/owlsort   # the configuration and the journal
```

Removing the journal loses the ability to `undo` past runs. Tidy up first if there
is anything you may want to put back.
