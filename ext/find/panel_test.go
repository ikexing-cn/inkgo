package find

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/find/findtext"
	"writer/internal/testshot"
	"writer/locales"
)

// The same matrix checks the live host and supplies the bilingual screenshots.
func TestFindPanelPrototype(t *testing.T) {
	for _, lang := range []string{"zh-Hans", "en"} {
		for _, wide := range []bool{false, true} {
			for _, hits := range []bool{true, false} {
				name := fmt.Sprintf("%s-book-%t-hits-%t", lang, wide, hits)
				t.Run(name, func(t *testing.T) {
					a, tt, _ := writingFixture(t, "林晚走进雨里，林晚停下。")
					second, err := a.Store.NewChapter("", "第二章", false)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = a.Store.WriteChapter(second, "林晚又看见了灯。", nil); err != nil {
						t.Fatal(err)
					}
					markUnchecked(a)
					i18n.SetLocaleFunc(func() string { return lang })
					t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "zh-Hans" }) })
					v := findOf(a)
					v.Finding, v.Wide, v.Query, v.With = true, wide, "林晚", "苏晴"
					if !hits {
						v.Query = "未出现"
					}
					tt.Frame()
					tt.Frame()
					rect := func(label string) ui.Rect {
						t.Helper()
						r, ok := tt.Find(label)
						if !ok {
							t.Fatalf("missing %q: %v", label, tt.Texts())
						}
						return r
					}
					absent := func(label string) {
						t.Helper()
						if _, ok := tt.Find(label); ok {
							t.Fatalf("unexpected %q", label)
						}
					}
					click := func(label string) {
						t.Helper()
						if err := tt.Click(label); err != nil {
							t.Fatal(err)
						}
						tt.Frame()
					}
					query, icon, count := rect(findtext.FindBox()), rect("find-query-icon"), rect("find-count")
					if icon.W != kit.IconSm || icon.H != kit.IconSm || icon.X >= query.X || icon.Y < query.Y || icon.Y+icon.H > query.Y+query.H {
						t.Fatalf("icon is not inside the query line: icon=%v input=%v", icon, query)
					}
					if count.W < 64 || count.X <= query.X || count.Y < query.Y || count.Y+count.H > query.Y+query.H {
						t.Fatalf("count is not on the query line: count=%v input=%v", count, query)
					}
					want := locales.FindNone()
					if hits {
						want = "1 / 2"
					}
					if wide {
						want = findtext.FindNoneBook()
						if hits {
							want = findtext.FindBookCount(2, 3)
						}
					}
					occurrences := 0
					for _, text := range tt.Texts() {
						if text == want {
							occurrences++
						}
					}
					if occurrences != 1 {
						t.Fatalf("count %q occurs %d times: %v", want, occurrences, tt.Texts())
					}
					countText := rect(want)
					if countText.Y < query.Y || countText.Y+countText.H > query.Y+query.H {
						t.Fatal("separate total line", countText)
					}
					replaceInput := rect(findtext.FindReplaceWith())
					if wide {
						absent(findtext.FindPrev())
						absent(findtext.FindNext())
						absent(findtext.FindReplace())
						absent(findtext.FindReplaceAll())
						rect(findtext.FindReplaceBook())
						if hits {
							heading := rect(locales.FindChapterHead("第一章", 2))
							if heading.Y < replaceInput.Y+replaceInput.H || heading.Y > replaceInput.Y+replaceInput.H+24 {
								t.Fatal("results do not directly follow replacement", heading, replaceInput)
							}
						} else {
							absent("全书结果-第一章-1")
						}
					} else {
						rect(findtext.FindPrev())
						rect(findtext.FindNext())
						rect(findtext.FindReplace())
						rect(findtext.FindReplaceAll())
						absent(findtext.FindReplaceBook())
					}
					testshot.Save(t, "find-panel-"+name+".png", tt.Image())
					if wide {
						click(findtext.FindReplaceBook())
						if v.Sure != hits {
							t.Fatalf("whole-book replacement enabled=%t, hits=%t", v.Sure, hits)
						}
					} else {
						click(findtext.FindReplace())
						if hits {
							if got := shownEditor(a).Doc.String(); got != "苏晴走进雨里，林晚停下。" {
								t.Fatal("first click must replace", got)
							}
						} else if tt.Focused(findtext.FindReplace()) {
							t.Fatal("no-hit replace took focus")
						}
						click(findtext.FindReplaceAll())
						if hits {
							if got := shownEditor(a).Doc.String(); got != "苏晴走进雨里，苏晴停下。" {
								t.Fatal(got)
							}
						} else if tt.Focused(findtext.FindReplaceAll()) {
							t.Fatal("no-hit replace-all took focus")
						}
					}
				})
			}
		}
	}
}

func TestFindPanelEmptyQueryAndOtherChapter(t *testing.T) {
	a, tt, _ := writingFixture(t, "本章没有姓名。")
	second, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(second, "林晚", nil); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	v := findOf(a)
	v.Finding, v.Wide = true, true
	tt.Frame()
	if tt.HasText(findtext.FindNoneBook()) || tt.HasText(locales.FindNone()) {
		t.Fatal("empty query shows no-match message")
	}
	v.Query = "林晚"
	tt.Frame()
	if !tt.HasText(findtext.FindBookCount(1, 1)) {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click(findtext.FindReplaceBook()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !v.Sure {
		t.Fatal("book replacement depends on hits in current chapter")
	}
}

func TestFindPanelQueryResetAndReplaceEnter(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚说，林晚走了。雨雨")
	v := findOf(a)
	v.Finding, v.Query, v.Current, v.With = true, "林晚", 1, "苏晴"
	tt.Frame()
	tt.Frame()
	if err := tt.Click(findtext.FindBox()); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("雨")
	tt.Frame()
	if v.Current != 0 || !tt.HasText("1 / 2") {
		t.Fatal("query did not reset position", v.Current, tt.Texts())
	}
	if err := tt.Click(findtext.FindReplaceWith()); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := shownEditor(a).Doc.String(); got != "林晚说，林晚走了。苏晴雨" {
		t.Fatal(got)
	}
	shownEditor(a).Undo()
	if got := shownEditor(a).Doc.String(); got != "林晚说，林晚走了。雨雨" {
		t.Fatal("cannot undo", got)
	}
	v.Current = 1
	if err := tt.Click(findtext.FindBook()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if v.Current != 0 {
		t.Fatal("scope toggle did not reset position")
	}
}
