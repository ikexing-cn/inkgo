package export

import (
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"writer/ext/export/exporttext"
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
			pairs = append(pairs, [2]string{got, []string{exporttext.ExportRangeBook(), exporttext.ExportRangeVolume(), locales.ExportRangeChapter(), exporttext.ExportRangeCustom()}[i]})
		}
		for _, tc := range []struct {
			what, why, want string
		}{
			{format.ProseContent, format.SkipNoChapters, exporttext.ExportSkipBody()},
			{format.OutlineContent, format.SkipNoChapters, exporttext.ExportSkipOutlineNone()},
			{format.OutlineContent, format.SkipNoOutlines, exporttext.ExportSkipOutlineEmpty()},
			{format.SettingContent, format.SkipNoLore, exporttext.ExportSkipSettings()},
		} {
			pairs = append(pairs, [2]string{skipLabel(tc.what, tc.why), tc.want})
		}
		_, d, _, _ := dialogFixture(t, "Text")
		for _, tc := range []struct {
			options format.Options
			want    string
		}{
			{format.Options{Prose: true, Outline: true}, exporttext.ExportAboutMany()},
			{format.Options{Setting: true}, exporttext.ExportAboutSettings()},
			{format.Options{Outline: true}, exporttext.ExportAboutOutline()},
			{format.Options{Prose: true, Text: true}, exporttext.ExportAboutText()},
			{format.Options{Prose: true}, exporttext.ExportAboutWord()},
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

func TestSpanExportFileNamesFollowInterfaceLanguage(t *testing.T) {
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	for _, tc := range []struct{ lang, part string }{{"zh-Hans", "第一章至第三章"}, {"en", "第一章 to 第三章"}} {
		tag = tc.lang
		h, d, _, _ := dialogFixture(t, "Text")
		if _, err := h.store.NewChapter("", "第三章", false); err != nil {
			t.Fatal(err)
		}
		d.Show()
		d.preferences().Text = true
		d.selection = format.Selection{Scope: format.ScopeSpan, From: "第一章", To: "第三章"}
		d.Replan()
		// Render through the tester so export receives only the current frame.
		run := ui.NewTester(func(c *ui.Context) { d.export(c, d.plan) }, 100, 100)
		run.Frame()
		path := filepath.Join(h.store.Root(), "导出", "测试作品-"+tc.part+".txt")
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}
