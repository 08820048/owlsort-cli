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
| D9 | **`rules.Build` takes an `Env`: interface language, time zone, clock, and whether the run is unattended.** | The engine must stay pure (no clock, no environment), so everything it needs from outside is passed in. A test pins them and gets the same plan every time. |
| D12 | **The commands that change the configuration edit its text, not its model.** `rules enable/disable` and `folders`/`keep` add and remove bytes; they never decode and re-encode the file. | `config.Marshal` would write every field in a machine's order with no comments, so the first `rules disable` would flatten a file the user had been curating. The cost is a small TOML reader of its own (`internal/tomledit`), which refuses anything it does not understand rather than guessing. Every write is re-parsed first, and it is refused if the result would not load or would not hold the same rules. |
| D13 | **`init` writes a commented template, not `Marshal` output.** | The file `init` writes is the first thing anyone sees, and it is what they will edit. `Marshal` gives `[[rule.when]]` sub-tables, every zero-valued field and not one comment. The template is generated from `Default`, and a test parses it and compares, so the pretty version cannot drift from the built-in rules. |
| D14 | **A rule may set its own `conflict`, and `Explain` returns a value rather than a sentence.** The conflict override makes [safety-model.md](safety-model.md) rule 3 true — it said "the rule's `conflict`" while only a file-wide one existed, and unknown keys are errors, so a reader who followed the document was blocked by the loader. `rules.Explain` now returns `Explained` and `internal/output` writes the words, because deciding belongs in `rules` and saying belongs where every other thing said to a person lives — and it is what lets `explain --json` exist at all. | 
| D11 | **`rules check` reports a shadowed rule only when it can prove it.** A rule is called shadowed when an earlier enabled rule takes every file it would, and the analysis answers "covers" only for cases it is sure about: extension lists, categories, name lists, size and day thresholds, `in_folder`, `path_contains`, with the later rule's `match`/`not` taken into account. Every pair it does not understand is silence. | A checker that cries wolf is worse than none: the second time it is wrong, nobody reads it again. Proof costs a little completeness — two rules whose conditions overlap only partly are not reported — and that is the right trade. |
| D10 | **Protection stops at the home folder and at a volume root: neither is ever classified as a project.** | A stray `package.json` in `~` (common with npm scripts) would otherwise make every file below the home folder "in a code project" and the tool would quietly do nothing. Real projects sit in a named folder below home, where the markers are still found. |

## Open

| | Question | Leaning |
|---|---|---|
| Q1 | **Name collisions.** Is `owlsort` taken in Homebrew core, Scoop, winget, or as a package name elsewhere? | Check before M4; if taken, the binary can stay `owlsort` with a different package name. |
| Q2 | **Repository owner.** This account, or an organisation? | This account for now; GitHub can transfer a repository later. |
| Q3 | **Languages at v0.1.** English only, or English + Simplified Chinese? | Both: the strings are few and the config already localises folder names. |
| Q4 | **Contributor agreement.** The code is GPL and the desktop apps are separate, so none is needed today. | None; revisit only if code is ever to be shared with a closed product. |
| Q5 | **Accept packages (Homebrew core, AUR, …) from the community?** | Yes, once v0.1 is released. |
