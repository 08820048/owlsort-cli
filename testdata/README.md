# testdata

Fixture trees for tests (a fake code project, a fake notes vault, mixed files). Create files in a test with `t.TempDir()` when you can; keep here only what is easier to read as a committed tree.

A fake code project cannot be committed: git refuses to store a path that contains a
`.git` folder, and that folder is the marker the tests look for. So the trees for
`scan` and `protect` are built in `t.TempDir()` instead, and this folder stays for
fixtures that are plain files.

Never put real files, paths or configuration from a real machine here.
