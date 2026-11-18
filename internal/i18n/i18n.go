// Package i18n resolves the interface language and holds the few words the
// engine itself produces (category folder names). Command and flag names are
// always English; only output text and default folder names are translated.
package i18n

import (
	"os"
	"strings"

	"github.com/08820048/owlsort-cli/internal/category"
)

// Language is a supported interface language.
type Language string

const (
	English           Language = "en"
	SimplifiedChinese Language = "zh-Hans"
)

// Resolve turns a user setting ("", "en", "zh", "zh-Hans", "zh_CN.UTF-8"...)
// into a supported language. An empty setting follows LC_ALL, LC_MESSAGES and
// LANG, in that order. Anything unknown is English.
func Resolve(setting string) Language {
	if setting == "" {
		for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
			if v := os.Getenv(name); v != "" {
				setting = v
				break
			}
		}
	}
	s := strings.ToLower(setting)
	if strings.HasPrefix(s, "zh") && !strings.Contains(s, "hant") && !strings.Contains(s, "tw") && !strings.Contains(s, "hk") {
		return SimplifiedChinese
	}
	return English
}

var categoryNames = map[Language]map[category.Category]string{
	English: {
		category.Image: "Images", category.Document: "Documents", category.Video: "Videos",
		category.Audio: "Audio", category.Archive: "Archives", category.Code: "Code", category.Other: "Other",
	},
	SimplifiedChinese: {
		category.Image: "图片", category.Document: "文档", category.Video: "视频",
		category.Audio: "音频", category.Archive: "压缩包", category.Code: "代码", category.Other: "其他",
	},
}

// CategoryName is what `{category}` expands to in the given language.
func CategoryName(lang Language, c category.Category) string {
	if names, ok := categoryNames[lang]; ok {
		if n, ok := names[c]; ok {
			return n
		}
	}
	return categoryNames[English][c]
}
