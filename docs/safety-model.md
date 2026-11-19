# Safety model

Owlsort moves people's files. These rules are the product; a change that weakens
one of them needs a decision recorded in [decisions.md](decisions.md), not just a
code review.

Each rule is written so it can be tested. The test that proves it is named, or is
to be written in the milestone shown.

| # | Rule | Test / milestone |
|---|---|---|
| 1 | **Preview first.** `preview` moves nothing. `tidy` shows the plan and waits for a yes before it moves anything. | **done**: `cli` `TestTidyWithANoLeavesTheFolderUntouched` |
| 2 | **No terminal, no moves.** Without a terminal (script, pipe, CI) `tidy` without `--yes` only previews and exits with code 4. Running in a script never moves files by accident. | **done**: `cli` `TestTidyWithoutATerminalOnlyPreviews`: the terminal is asked of the kernel, not guessed from the file mode, so `/dev/null` counts as no terminal |
| 3 | **Never overwrite.** If the destination has a file of that name, the rule's `conflict` decides — its own, or the file's default: `rename` keeps both (`name 2.ext`), `skip` leaves the file where it is. Nothing is replaced. | **done**: `apply` `TestALeapFrogIsRenamedAndBothFilesSurvive`, `TestSkipLeavesTheFileWhereItIs`, `TestARuleCanAskForItsOwnConflictStrategy`; `cli` `TestTidyNeverOverwritesAndExitsWithThree` |
| 4 | **Never delete for good.** There is no delete action. The heaviest action is `trash`, which goes to the system trash. | **done**: `internal/policy` `TestOnlyApplyAndTrashRemoveFiles` fails if `os.Remove`/`RemoveAll` appears outside `internal/apply`, `internal/trash` and `internal/atomicfile` — the last removes only the temporary file it created beside the one it replaces |
| 5 | **Everything can be undone.** Each step is written to the journal before the next one starts. `undo` puts a batch back, newest step first, and takes away the empty folders the run made. | **done**: `apply` `TestUndoRestoresTheFolderExactly`, `TestUndoWalksTheBatchBackwards`; `cli` `TestTidyThenUndoRestoresTheFolder` |
| 6 | **Undo checks first.** A step is undone only if the file is still where it was put and the original place is free. Otherwise that step is skipped and reported; nothing is overwritten. | **done**: `apply` `TestUndoSkippedWhenTheOriginalPlaceIsTaken`, `TestUndoSkippedWhenTheFileHasMovedOn`, `TestUndoneStepsAreNotUndoneTwice` |
| 7 | **Re-check before each step.** Between preview and run, files change. Each step checks that the source still exists and is unchanged since the preview (size and modification time); if not, it is skipped with the reason `changed`. | **done**: `apply` `TestAFileThatChangedSinceThePreviewIsLeftAlone`, `TestAFileThatDisappearedSinceThePreviewIsReported` |
| 8 | **Loose files only.** Only the top level of a watched folder is tidied, unless a rule says `subfolders = true`. Subfolders are taken to be finished work. | M0: scan tests |
| 9 | **Unattended means conservative.** In `watch`, only `background = true` rules may `trash`. Moves, copies and renames are allowed. | M3 |
| 10 | **Protected places stay put.** Code projects (`.git`, `package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`, `*.sln`, `Package.swift`, …), notes libraries (Obsidian `.obsidian/`, Logseq `logseq/config.edn`), and everything the user keeps (`keep`). Build output inside a project is not protected. | M0: protect tests with fixture trees |
| 11 | **Cross-volume moves are copy, verify, remove.** Moving between volumes copies the file, checks the size (and a hash for small files), and only then removes the source. A failed check leaves the source in place and removes the copy. | **done**: `apply` `TestMovingAcrossVolumesCopiesThenRemoves` (which compares inodes, so it cannot pass on a rename), `TestMovingABigFileAcrossVolumes`, `TestAFailedCrossVolumeMoveLeavesTheSourceAlone`, `TestUndoAcrossVolumes`. They need two volumes, so CI makes one from a disk image (`make cross-volume-test`) |
| 12 | **A bad configuration stops everything.** If the config cannot be read or is invalid, the program exits with code 2. It does **not** fall back to the built-in rules: that could move files the user's own rules would have left alone. | `internal/config`: `Load` returns an error; M0: commands refuse to run |
| 13 | **Files still being written are left alone.** `.crdownload`, `.part`, `.download`, `.tmp` names, and anything that changed in the last few seconds. | M0 |
| 14 | **A destination cannot climb out.** A relative destination containing `..` is rejected at config time and again when planning. A variable's value is a single path segment (`internal/pathvars`). | done: `config` and `pathvars` tests |
| 15 | **Cloud placeholders are not read.** Reading a macOS `.icloud` placeholder or a Windows OneDrive online-only file would download it. | M0 |
| 16 | **No network.** Nothing in the program opens a connection. | **done**: `internal/policy` `TestNoNetwork` fails if any network package is imported |

## What the preview must show

So that a "yes" is informed (`docs/commands.md` has the exact output):

- how many files will move, from which folder, to which folders;
- each file's source and destination, and the rule that chose it;
- what is **skipped** and why (in a code project, kept, still downloading, a clash
  left alone);
- how to undo.

## If something goes wrong

- `owlsort history` lists every run; `owlsort undo` puts the newest back.
- Every step is in the journal even if the run crashed halfway; undo works on the
  steps that were written.
- A bug report where a file is lost or ended up somewhere unexpected is the
  highest priority issue, ahead of any feature.
