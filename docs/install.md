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

## macOS: the binary is not signed

The macOS binaries are **not signed or notarized**. A copy downloaded through a
browser carries the quarantine flag, and macOS refuses to run it until you say so.

Two honest ways, both fine:

- **Build it yourself** (`go install …` above). A binary you compiled locally has no
  quarantine flag, which is why this is the recommendation rather than a workaround.
- **Allow the download once**, after checking the checksum against the release page:

  ```sh
  shasum -a 256 ~/Downloads/owlsort_0.1.0_darwin_arm64.tar.gz
  xattr -d com.apple.quarantine /usr/local/bin/owlsort
  ```

Signing and notarization need a paid Apple Developer account
([release.md](release.md) describes what it would take). Until then this page says
so, and the release notes say so.

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
