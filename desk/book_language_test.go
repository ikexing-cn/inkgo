package desk

import (
	"github.com/egoist/mygo/i18n"
	"strings"
	"testing"
	"writer/internal/testshot"
	"writer/locales"
	"writer/workspace"
)

func TestInterfaceCountUnitFollowsBookUnderChineseAndEnglishInterfaces(t *testing.T) {
	defer i18n.SetLocaleFunc(testLocale)
	for _, tc := range []struct{ ui, book, want string }{
		{"zh-Hans", "zh-Hans", "本章 2 字"}, {"zh-Hans", "en", "本章 2 词"}, {"en", "zh-Hans", "Chapter 2 chars"}, {"en", "en", "Chapter 2 words"},
	} {
		i18n.SetLocaleFunc(func() string { return tc.ui })
		if got := locales.StatusChapterWords(2, workspace.Language(tc.book).Unit()); got != tc.want {
			t.Errorf("%s %s: %q", tc.ui, tc.book, got)
		}
	}
}

func TestShotEnglishBookWritingAndLoreCategories(t *testing.T) {
	testshot.Dir(t)
	a, tt, id := writingFixture(t, "The rain stopped. Mara opened the door.")
	work := a.Store.Work()
	work.Name = "The Last Harbor"
	work.Language = "en"
	work.Categories = append(work.Categories, workspace.Category{Name: "Other", Target: "Ideas"})
	if err := a.Store.SaveWork(work); err != nil {
		t.Fatal(err)
	}
	// Replace fixture lore before switching to the English book's categories.
	set, err := a.Store.Setting()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range set.Entries {
		if _, err = a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpMove, Entry: e.ID, Category: "Other"}}}); err != nil {
			t.Fatal(err)
		}
	}
	work.Categories = workspace.Categories(work.Genre, "en")
	if err = a.Store.SaveWork(work); err != nil {
		t.Fatal(err)
	}
	if err = a.Store.RenameChapter(id, "Chapter One"); err != nil {
		t.Fatal(err)
	}
	if err = a.book.ReloadTree(); err != nil {
		t.Fatal(err)
	}
	if err = a.book.ReloadCounts(); err != nil {
		t.Fatal(err)
	}
	goalOf(a).Counter.DayStart = 0
	if err = a.refresh(); err != nil {
		t.Fatal(err)
	}
	a.write.counted = nil
	tt.Frame()
	tt.Frame()
	if !strings.Contains(strings.Join(paintedTexts(tt), "\n"), "本章 7 词") {
		t.Fatal(paintedTexts(tt))
	}
	shot(t, tt, "book-en-write.png")
	showPage(a, pageCategories)
	tt.Frame()
	tt.Frame()
	shot(t, tt, "book-en-categories.png")
	texts := strings.Join(paintedTexts(tt), "\n")
	if !strings.Contains(texts, "People") || !strings.Contains(texts, "Other") {
		t.Fatal(texts)
	}
}
