package i18n

import "testing"

func TestResolve(t *testing.T) {
	for setting, want := range map[string]Language{
		"en": English, "en_US.UTF-8": English, "zh": SimplifiedChinese, "zh-Hans": SimplifiedChinese,
		"zh_CN.UTF-8": SimplifiedChinese, "zh_TW.UTF-8": English, "zh-Hant": English, "fr": English,
	} {
		if got := Resolve(setting); got != want {
			t.Errorf("%q: got %s, want %s", setting, got, want)
		}
	}
}

func TestResolveFollowsTheEnvironment(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "zh_CN.UTF-8")
	if Resolve("") != SimplifiedChinese {
		t.Error("an empty setting should follow LANG")
	}
	t.Setenv("LC_ALL", "en_US.UTF-8")
	if Resolve("") != English {
		t.Error("LC_ALL wins over LANG")
	}
}
