package desk

import (
	"github.com/egoist/mygo/i18n"
	"strings"
	"testing"
	"time"
	"writer/ext/backup/archive"
	"writer/internal/testshot"
	"writer/workspace"
)

func TestEnglishFooterFitsPrototypeCounts(t *testing.T) {
	defer i18n.SetLocaleFunc(testLocale)
	i18n.SetLocaleFunc(func() string { return "en" })
	prose := "甲" + strings.Repeat("字", 2845)
	a, tt, id := writingFixture(t, prose)
	a.setExtension("goal", false, false)
	a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "setting"})

	other, err := a.book.NewChapter("", "Other", false)
	if err != nil {
		t.Fatal(err)
	}
	a.book.SetCount(other, 312000-2846)
	backupOf(a).Configure(archive.Settings{Dir: t.TempDir(), Last: time.Now()})
	if err = a.book.SaveCheck(id, workspace.Check{Of: workspace.ProseHash(prose), At: time.Now(), Findings: []workspace.Finding{{ID: workspace.NewID(), Kind: workspace.FindNew, Title: "new", Quote: "甲", Name: "new name", Category: "人物", Text: "new character"}}}); err != nil {
		t.Fatal(err)
	}
	a.loadChecks()

	a.book.SetCount(other, 312000-2846)
	tt.SetSize(1266, 780)
	tt.Frame()
	tt.Frame()
	for _, text := range []string{"1 issue found", "Per Chapter ▴", "Backed Up", "Chapter 2,846 chars · Book 312,000 chars"} {
		r := find(t, tt, text)
		minimum := map[string]float32{"1 issue found": 84, "Per Chapter ▴": 84, "Backed Up": 67, "Chapter 2,846 chars · Book 312,000 chars": 268}[text]
		if r.W < minimum {
			t.Fatal(text, r)
		}
		t.Log(text, r)
	}
	testshot.Save(t, "footer-english-prototype.png", tt.Image())
}
