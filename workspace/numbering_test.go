package workspace

import "testing"

func TestNextTitle(t *testing.T) {
	for before, want := range map[string]string{
		"第四章 名单":    "第五章",
		"第九章":       "第十章",
		"第十九章 雨":    "第二十章",
		"第九十九回":     "第一百回",
		"第一百零九章":    "第一百一十章",
		"第一千九百九十九章": "第二千章",
		"第二卷":       "第三卷",
		"第12章 夜":    "第13章",
		"楔子":        "",
		"第一二章":      "",
		"":          "",
	} {
		if got := nextTitle(before); got != want {
			t.Errorf("after %q: %q, want %q", before, got, want)
		}
	}
}
