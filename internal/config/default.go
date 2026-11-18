package config

import "github.com/08820048/owlsort-cli/internal/i18n"

func on(b bool) *bool { return &b }

// Default is what `owlsort init` writes: the built-in rules, in priority
// order. Screenshots and recordings come before the general pictures and
// videos that would otherwise take them; installers before archives.
// Rules that move many kinds of file are off until the user turns them on.
func Default(lang i18n.Language) Config {
	zh := lang == i18n.SimplifiedChinese
	pick := func(en, cn string) string {
		if zh {
			return cn
		}
		return en
	}

	return Config{
		Folders:  []string{"~/Downloads", "~/Desktop"},
		Conflict: ConflictRename,
		Rules: []Rule{
			{
				Name: "Desktop by date", Enabled: on(false),
				When:   []Condition{{InFolder: "~/Desktop"}},
				MoveTo: pick("Desktop Archive/{date}/{category}", "桌面归档/{date}/{category}"),
			},
			{
				Name: "Screenshots", Match: "all",
				When: []Condition{
					{Category: "image"},
					{NameContains: []string{"Screenshot", "Screen Shot", "截屏", "截图", "屏幕快照", "Snipaste", "CleanShot", "Shottr", "Xnip", "iShot", "PixPin"}},
				},
				MoveTo: pick("Pictures/Screenshots", "图片/截图"),
			},
			{
				Name: "Screen recordings", Enabled: on(false), Match: "all",
				When: []Condition{
					{Category: "video"},
					{NameContains: []string{"Screen Recording", "屏幕录制", "录屏", "Bildschirmaufnahme", "画面収録"}},
				},
				MoveTo: pick("Videos/Screen Recordings", "视频/录屏"),
			},
			{
				Name:   "PDFs",
				When:   []Condition{{Ext: []string{"pdf"}}},
				MoveTo: pick("Documents/PDF", "文档/PDF"),
			},
			{
				Name:   "Installers",
				When:   []Condition{{Ext: []string{"dmg", "pkg", "mpkg", "iso", "exe", "msi", "apk", "deb", "rpm", "appimage"}}},
				MoveTo: pick("Installers", "安装包"),
			},
			{
				Name:   "Archives",
				When:   []Condition{{Category: "archive"}},
				MoveTo: pick("Archives", "压缩包"),
			},
			{
				Name: "Office documents", Enabled: on(false),
				When:   []Condition{{Ext: []string{"doc", "docx", "xls", "xlsx", "ppt", "pptx", "csv", "tsv", "txt", "md", "rtf", "odt", "ods", "odp", "pages", "numbers", "key"}}},
				MoveTo: pick("Documents/Office", "文档/办公"),
			},
			{
				Name: "Other pictures", Enabled: on(false),
				When:   []Condition{{Category: "image"}},
				MoveTo: pick("Pictures/Other", "图片/其他"),
			},
			{
				Name: "Audio and video", Enabled: on(false),
				When:   []Condition{{Ext: []string{"mp4", "mov", "m4v", "mkv", "avi", "webm", "wmv", "flv", "mp3", "wav", "aac", "m4a", "flac", "ogg", "opus", "aiff"}}},
				MoveTo: pick("Media", "影音"),
			},
		},
	}
}
