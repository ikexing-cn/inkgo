package desk

import (
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
	"time"
	"writer/desk/kit"
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

func TestEnglishFooterReportedNotBackedUpWithGoal(t *testing.T) {
	defer i18n.SetLocaleFunc(testLocale)
	i18n.SetLocaleFunc(func() string { return "en" })
	for _, goal := range []bool{false, true} {
		t.Run(map[bool]string{false: "goal-off", true: "goal-on"}[goal], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "Short prose.")
			a.setExtension("goal", goal, false)
			a.doc.hidden = false
			backupOf(a).Configure(archive.Settings{})
			other, err := a.book.NewChapter("", "Other", false)
			if err != nil {
				t.Fatal(err)
			}
			a.book.SetCount(other, 312000)
			// Compare against the full natural widths in a wide window.
			labels := []string{"Check available after 200 characters", "Per Chapter ▴", "Not Backed Up"}
			tt.SetSize(2400, 780)
			tt.Frame()
			tt.Frame()
			widths := map[string]float32{}
			for _, label := range labels {
				natural := stillTester(func(c *ui.Context) {
					ui.Row(c).Children(func() { ui.Text(c, label).FontSize(kit.FsNote).Shrink(0).SingleLine() })
				}, 1000, 100)
				widths[label] = find(t, natural, label).W
				t.Log(label, widths[label], find(t, tt, label).W)
			}
			counts := find(t, tt, "Character Count").W
			tt.SetSize(1266, 780)
			tt.Frame()
			tt.Frame()
			foot := find(t, tt, "底栏")
			for _, label := range labels {
				r := find(t, tt, label)
				if r.W < widths[label]-0.01 || r.X < foot.X || r.X+r.W > foot.X+foot.W {
					t.Fatalf("%s truncated: %v, full width %v", label, r, widths[label])
				}
			}
			r := find(t, tt, "Character Count")
			if r.W <= 0 || r.W >= counts-1 {
				t.Fatal("counts should absorb the footer deficit", r, counts)
			}
			t.Logf("goal=%v counts width %.1f / %.1f; fixed footer items retain full widths", goal, r.W, counts)
			testshot.Save(t, map[bool]string{false: "footer-reported-goal-off.png", true: "footer-reported-goal-on.png"}[goal], tt.Image())
		})
	}
}
