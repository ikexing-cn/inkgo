package export

import (
	"github.com/egoist/mygo/i18n"
	"strings"
	"testing"
	"unicode"
	"writer/ext/export/format"
	"writer/locales"
)

func TestExportMappingsInBothLocales(t *testing.T) {
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	for _, next := range []string{"zh-Hans", "en"} {
		tag = next
		pairs := [][2]string{}
		for i, got := range scopeLabels([]string{format.ScopeWork, format.ScopeVolume, format.ScopeChapter, format.ScopeSpan}) {
			pairs = append(pairs, [2]string{got, []string{locales.ExportRangeBook(), locales.ExportRangeVolume(), locales.ExportRangeChapter(), locales.ExportRangeCustom()}[i]})
		}
		for _, tc := range []struct {
			what, why, want string
		}{
			{format.ProseContent, format.SkipNoChapters, locales.ExportSkipBody()},
			{format.OutlineContent, format.SkipNoChapters, locales.ExportSkipOutlineNone()},
			{format.OutlineContent, format.SkipNoOutlines, locales.ExportSkipOutlineEmpty()},
			{format.SettingContent, format.SkipNoLore, locales.ExportSkipSettings()},
		} {
			pairs = append(pairs, [2]string{skipLabel(tc.what, tc.why), tc.want})
		}
		_, d, _, _ := dialogFixture(t, "Text")
		for _, tc := range []struct {
			options format.Options
			want    string
		}{
			{format.Options{Prose: true, Outline: true}, locales.ExportAboutMany()},
			{format.Options{Setting: true}, locales.ExportAboutSettings()},
			{format.Options{Outline: true}, locales.ExportAboutOutline()},
			{format.Options{Prose: true, Text: true}, locales.ExportAboutText()},
			{format.Options{Prose: true}, locales.ExportAboutWord()},
		} {
			d.preferences().Options = tc.options
			d.Open = true
			d.Replan()
			pairs = append(pairs, [2]string{aboutLabel(d.plan.About), tc.want})
		}
		for _, pair := range pairs {
			if pair[0] != pair[1] || tag == "en" && strings.ContainsFunc(pair[0], func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Fatal(tag, pair)
			}
		}
	}
}
