package scan

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// write makes a file of the given size, last modified at mod.
func write(t *testing.T, path string, size int, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	if !mod.IsZero() {
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
}

// byName indexes a listing, so a test can ask about one file.
func byName(files []File) map[string]File {
	m := make(map[string]File, len(files))
	for _, f := range files {
		m[f.Name] = f
	}
	return m
}

func TestFolderListsTheTopLevelOnly(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-2 * time.Hour)
	write(t, filepath.Join(dir, "report.pdf"), 10, old)
	write(t, filepath.Join(dir, "sub", "inside.pdf"), 10, old)

	got, err := Folder(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "report.pdf" {
		t.Fatalf("top level: got %v", namesOf(got))
	}
	if got[0].Folder != dir {
		t.Errorf("Folder = %q, want %q", got[0].Folder, dir)
	}

	deep, err := Folder(dir, Options{Subfolders: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(deep) != 2 {
		t.Fatalf("subfolders: got %v", namesOf(deep))
	}
	for _, f := range deep {
		if f.Folder != dir {
			t.Errorf("%s: Folder = %q, want the watched folder %q", f.Name, f.Folder, dir)
		}
	}
}

func TestHiddenFilesAreListedAndMarked(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".DS_Store"), 10, time.Now().Add(-time.Hour))
	write(t, filepath.Join(dir, "photo.png"), 10, time.Now().Add(-time.Hour))

	files, err := Folder(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := byName(files)
	if !got[".DS_Store"].Hidden {
		t.Error(".DS_Store should be marked hidden")
	}
	if got["photo.png"].Hidden {
		t.Error("photo.png should not be marked hidden")
	}
	if ext := got[".DS_Store"].Extension; ext != "" {
		t.Errorf("extension of .DS_Store = %q, want none", ext)
	}
}

func TestDownloadingIsRecognised(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-2 * time.Hour)
	write(t, filepath.Join(dir, "film.mp4.crdownload"), 10, old)
	write(t, filepath.Join(dir, "firefox.part"), 10, old)
	write(t, filepath.Join(dir, "just-wrote.png"), 10, time.Time{})
	write(t, filepath.Join(dir, "placeholder.pdf.icloud"), 10, old)
	write(t, filepath.Join(dir, "settled.zip"), 10, old)

	files, err := Folder(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := byName(files)
	for _, name := range []string{"film.mp4.crdownload", "firefox.part", "just-wrote.png", "placeholder.pdf.icloud"} {
		if !got[name].Downloading {
			t.Errorf("%s should be marked as still downloading", name)
		}
	}
	if got["settled.zip"].Downloading {
		t.Error("settled.zip has been quiet for two hours")
	}
}

func TestAPackageIsOneFileAndIsNeverOpened(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Owlsort.app", "Contents", "MacOS", "owlsort"), 10, time.Now().Add(-time.Hour))
	write(t, filepath.Join(dir, "notes.txt"), 10, time.Now().Add(-time.Hour))

	files, err := Folder(dir, Options{Subfolders: true})
	if err != nil {
		t.Fatal(err)
	}
	got := byName(files)
	pkg, ok := got["Owlsort.app"]
	if !ok {
		t.Fatalf("the package should be listed as one file, got %v", namesOf(files))
	}
	if !pkg.Package {
		t.Error("Owlsort.app should be marked as a package")
	}
	if pkg.Size != 0 {
		t.Errorf("a package's size is not computed, got %d", pkg.Size)
	}
	if _, ok := got["owlsort"]; ok {
		t.Error("the scan walked into the package")
	}
	if len(files) != 2 {
		t.Errorf("got %v, want the package and notes.txt", namesOf(files))
	}
}

func TestSymbolicLinksAreNotFollowed(t *testing.T) {
	outside := t.TempDir()
	write(t, filepath.Join(outside, "away.pdf"), 10, time.Now().Add(-time.Hour))

	dir := t.TempDir()
	link := filepath.Join(dir, "link.pdf")
	if err := os.Symlink(filepath.Join(outside, "away.pdf"), link); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}

	files, err := Folder(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "link.pdf" {
		t.Fatalf("got %v, want the link itself and nothing it points at", namesOf(files))
	}
}

func TestCreatedComesFromTheFilesystem(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.pdf")
	write(t, path, 10, time.Now().Add(-time.Hour))

	files, err := Folder(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %v", namesOf(files))
	}
	if runtime.GOOS == "darwin" && files[0].Created.IsZero() {
		t.Error("macOS keeps a birth time; Created should not be zero")
	}
	if files[0].Arrived().IsZero() {
		t.Error("Arrived never falls back to nothing")
	}
}

func TestFolderRejectsWhatIsNotAFolder(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "report.pdf")
	write(t, file, 10, time.Time{})

	if _, err := Folder(file, Options{}); err == nil {
		t.Error("a file is not a folder")
	}
	if _, err := Folder(filepath.Join(dir, "missing"), Options{}); err == nil {
		t.Error("a folder that is not there is an error")
	}
}

func namesOf(files []File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Name
	}
	return out
}
