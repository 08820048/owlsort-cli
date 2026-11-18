// Package category classifies a file by its extension. The categories are the
// values the `category` condition accepts and what `{category}` expands to.
package category

import "strings"

// Category is a coarse kind of file.
type Category string

const (
	Image    Category = "image"
	Document Category = "document"
	Video    Category = "video"
	Audio    Category = "audio"
	Archive  Category = "archive"
	Code     Category = "code"
	Other    Category = "other"
)

// All lists every category, in display order.
var All = []Category{Image, Document, Video, Audio, Archive, Code, Other}

// Valid reports whether name is a known category.
func Valid(name string) bool {
	for _, c := range All {
		if string(c) == name {
			return true
		}
	}
	return false
}

var byExtension = func() map[string]Category {
	sets := map[Category]string{
		Image:    "png jpg jpeg gif webp heic heif bmp tif tiff svg ico raw cr2 nef arw dng psd",
		Document: "pdf doc docx xls xlsx ppt pptx csv tsv txt md rtf odt ods odp pages numbers key epub mobi",
		Video:    "mp4 mov m4v mkv avi webm wmv flv mpg mpeg",
		Audio:    "mp3 wav aac m4a flac ogg opus aiff wma",
		Archive:  "zip rar 7z tar gz tgz bz2 xz zst",
		Code:     "go py js ts tsx jsx java c h cpp hpp cs swift kt rs rb php sh ps1 json yaml yml toml xml html css sql",
	}
	m := make(map[string]Category)
	for c, list := range sets {
		for _, ext := range strings.Fields(list) {
			m[ext] = c
		}
	}
	return m
}()

// ForExtension returns the category of an extension, with or without the
// leading dot, in any case. Unknown extensions are Other.
func ForExtension(ext string) Category {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if c, ok := byExtension[ext]; ok {
		return c
	}
	return Other
}
