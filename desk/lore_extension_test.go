package desk

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
	"writer/ext"
	exportext "writer/ext/export"
	"writer/ext/export/exporttext"
	"writer/ext/hostapi"
	"writer/internal/testshot"
	"writer/locales"
)

func TestLoreGlobalAndBookSwitchRemoveAndRestoreDependentInterfaces(t *testing.T) {
	for _, bookScope := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[bookScope], func(t *testing.T) {
			a, tt, _ := marginFixture(t, true)
			for _, on := range []bool{false, true} {
				a.setExtension("lore", on, bookScope)
				tt.Frame()
				tt.Frame()
				if (len(a.write.nameLinks.val) > 0) != on || (len(a.write.doubts.val) > 0) != on {
					t.Fatal("prose name links or check marks", on)
				}
				if a.hasEntries() != on || a.checksOn() != on || (len(a.entities()) > 0) != on {
					t.Fatal("entry consumers", on)
				}
				if a.pages[pageCheck].available(a) != on {
					t.Fatal("check page", on)
				}
				if slices.Contains(a.commandList(), cmdTake) != on {
					t.Fatal("selection command", on)
				}
				if (len(ext.Items(a.extensions, hostapi.MarginSources)) > 0) != on || (len(ext.Items(a.extensions, hostapi.SelectionMenus)) > 0) != on {
					t.Fatal("lore contributions", on)
				}
				if _, ok := a.extensionTab(paneSetting); ok != on {
					t.Fatal("setting tab", on)
				}
				for _, note := range a.marginNotes() {
					if !on && (note.idea != nil || note.check != nil) {
						t.Fatal("disabled margin note")
					}
				}
				for _, g := range a.pendingGroups() {
					if !on && g.id == "check" {
						t.Fatal("disabled pending checks")
					}
				}
				for _, page := range []page{pageTalk, pagePlan} {
					a.page = page
					tt.Frame()
					if a.hasPane() != on {
						t.Fatal("beside pane", page, on)
					}
					// DESIGN.md: the switch stays in place, disabled, on a page without the column.
					if _, ok := tt.Find(cmdPane.name(a)); !ok {
						t.Fatal("pane switch removed", page, on)
					}
				}
				a.page = pageWrite
				a.openPrefs("appearance")
				tt.Frame()
				if tt.HasText(locales.SettingsMarkNames()) != on {
					t.Fatal("appearance", on)
				}
				a.closePrefs()
			}
			a.page = pageCheck
			a.setExtension("lore", false, bookScope)
			tt.Frame()
			if a.page != pageWrite {
				t.Fatal("check page did not return to prose")
			}
		})
	}
}

func TestContextSwitchStopsLoreChecksAndNamesAndLoreEnableStartsContext(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	a.setExtension("context", false, false)
	tt.Frame()
	if a.extensions.Ledger("lore").State == ext.On || a.hasEntries() || a.checksOn() || len(a.entities()) != 0 {
		t.Fatal("dependent interfaces survived")
	}
	a.setExtension("lore", true, false)
	tt.Frame()
	if a.extensions.Ledger("context").State != ext.On || !a.hasEntries() || !a.checksOn() || len(a.entities()) == 0 {
		t.Fatal("requirements not restored")
	}
}

func TestLoreStopSavesEntryAndChangeFormsAndRestartDoesNotLeakBookState(t *testing.T) {
	a, _, e := fixture(t)
	p := lorePanel(a)
	p.Edit(e)
	p.Doc.Text = "关闭前写的设定"
	a.setExtension("lore", false, false)
	entry, _ := a.book.Setting().Entry(e.ID)
	if entry.Text != "关闭前写的设定" {
		t.Fatal(entry)
	}
	a.setExtension("lore", true, false)
	p = lorePanel(a)
	p.Jump(e.ID)
	p.Doc.Route.EditChange("new")
	chapter, err := a.book.NewChapter("", "变化起点", false)
	if err != nil {
		t.Fatal(err)
	}
	p.Doc.From = chapter
	p.Doc.Text = "关闭前写的变化"
	a.setExtension("lore", false, false)
	a.setExtension("lore", true, false)
	entry, _ = a.book.Setting().Entry(e.ID)
	if len(entry.Changes) != 1 || entry.Changes[0].Text != "关闭前写的变化" {
		t.Fatal(entry)
	}
	start := time.Now()
	for range 50 {
		a.extensions.Close()
		requireExtension(t, a.extensions.Open(ext.Book{Dir: a.Store.Root()}))
	}
	elapsed := time.Since(start)
	t.Logf("50 book Stop/Start cycles: %s", elapsed)
	if elapsed > time.Second {
		t.Fatal("book restart too expensive", elapsed)
	}
	if lorePanel(a).Doc.Text != "" {
		t.Fatal("draft leaked across session")
	}
	requireExtension(t, a.Open(extensionBook(t)))
	if lorePanel(a).Doc.Text != "" {
		t.Fatal("draft leaked across books")
	}
}

func TestLoreExportOptionFollowsGlobalAndBookSwitchWithoutLosingTick(t *testing.T) {
	for _, perBook := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[perBook], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "Text")
			v := exportext.Choices{}
			v.Setting, v.Text = true, true
			saveChoices(t, a, v)
			runExport(t, a)
			tt.Frame()
			flip := func(on bool) {
				t.Helper()
				var err error
				if perBook {
					err = a.extensions.SetBook("lore", on)
				} else {
					err = a.extensions.Set("lore", on)
				}
				if err != nil {
					t.Fatal(err)
				}
				tt.Frame()
			}
			flip(false)
			if tt.HasText("设定") || !tt.HasText(exporttext.ExportAboutText()) || tt.HasText("范围") {
				t.Fatal(tt.Texts())
			}
			// Clicking the disabled export button must create no output.
			r, ok := tt.Find("取消")
			if !ok {
				t.Fatal(tt.Texts())
			}
			tt.ClickAt(r.X+r.W+32, r.Y+r.H/2)
			tt.Frame()
			if _, err := os.Stat(filepath.Join(a.Store.Root(), "导出")); !os.IsNotExist(err) {
				t.Fatal("disabled export wrote files", err)
			}
			if !choicesOf(t, a).Setting {
				t.Fatal("stored tick cleared")
			}
			flip(true)
			if !tt.HasText("设定") {
				t.Fatal(tt.Texts())
			}
			r, _ = tt.Find("取消")
			tt.ClickAt(r.X+r.W+32, r.Y+r.H/2)
			tt.Frame()
			if _, err := os.Stat(filepath.Join(a.Store.Root(), "导出", "测试作品-设定.txt")); err != nil {
				t.Fatal("restored tick did not export", err)
			}
		})
	}
}

func TestShotLoreExtension(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := writingFixture(t, "林晚沿着路灯走进旧城。")
	a.openPrefs("extensions")
	shot(t, tt, "ext-lore-overview.png")
	click(t, tt, "扩展-lore")
	shot(t, tt, "ext-lore-details.png")
	a.closePrefs()
	a.setExtension("lore", false, false)
	shot(t, tt, "ext-lore-off-writing.png")
	a.page = pageTalk
	shot(t, tt, "ext-lore-off-talk.png")
	runExport(t, a)
	shot(t, tt, "ext-lore-off-export.png")
}
