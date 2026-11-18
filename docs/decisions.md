# Decisions

## Made

| | Decision | Why |
|---|---|---|
| D1 | **Independent of the desktop apps.** Own code, own data, own release numbers; not aligned feature for feature. | Sharing an engine would force changes to the desktop apps and tie the licence of this project to theirs. Parity was never a requirement. |
| D2 | **Go, no cgo.** | One machine builds every platform; small static binaries; no runtime to install. |
| D3 | **TOML config**, edited by hand. | Fewer traps than YAML; comments are allowed; good for dotfiles. |
| D4 | **JSON Lines journal**, not a database. | No cgo (so no SQLite C library); readable and recoverable by hand; append-only fits undo. |
| D5 | **GPL-3.0 for the code; the name and logo are reserved** ([TRADEMARK.md](../TRADEMARK.md)). | Forks stay open and cannot be sold as closed products; the brand stays with the project. |
| D6 | **No permanent delete, no scripts, no network.** | A file tidier that can lose files, or run arbitrary commands, or phone home, is not safe to leave running. |
| D7 | **Preview first, undo always** are properties of the code ([safety-model.md](safety-model.md)), not options. | It is the reason to pick this tool over a shell script. |
| D8 | **Defaults for the provisional choices below are TOML, English + Simplified Chinese, command `owlsort`.** | So work can start; each is cheap to change before v0.1. |

## Open

| | Question | Leaning |
|---|---|---|
| Q1 | **Name collisions.** Is `owlsort` taken in Homebrew core, Scoop, winget, or as a package name elsewhere? | Check before M4; if taken, the binary can stay `owlsort` with a different package name. |
| Q2 | **Repository owner.** This account, or an organisation? | This account for now; GitHub can transfer a repository later. |
| Q3 | **Languages at v0.1.** English only, or English + Simplified Chinese? | Both: the strings are few and the config already localises folder names. |
| Q4 | **Contributor agreement.** The code is GPL and the desktop apps are separate, so none is needed today. | None; revisit only if code is ever to be shared with a closed product. |
| Q5 | **Accept packages (Homebrew core, AUR, …) from the community?** | Yes, once v0.1 is released. |
