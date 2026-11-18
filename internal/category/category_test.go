package category

import "testing"

func TestForExtension(t *testing.T) {
	for ext, want := range map[string]Category{
		"png": Image, ".PNG": Image, "HEIC": Image, "pdf": Document, "docx": Document,
		"mp4": Video, "mp3": Audio, "zip": Archive, "swift": Code, "xyz": Other, "": Other,
	} {
		if got := ForExtension(ext); got != want {
			t.Errorf("%q: got %s, want %s", ext, got, want)
		}
	}
}

func TestValid(t *testing.T) {
	if !Valid("image") || Valid("spreadsheet") || Valid("") {
		t.Error("Valid is wrong")
	}
}
