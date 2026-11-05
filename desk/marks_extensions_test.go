package desk

import (
	"github.com/egoist/mygo/ui"
	"slices"
	"testing"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/check/checktext"
	"writer/ext/hooks/hookstext"
	"writer/ext/hostapi"
	"writer/ext/marks/marksapi"
	"writer/ext/names/namestext"
	"writer/locales"
)

func TestEachProseExtensionGlobalAndBookOffRemovesItsSurfacesAndOnRestoresThem(t *testing.T) {
	for _, id := range []string{"hooks", "notes", "names"} {
		for _, book := range []bool{false, true} {
			t.Run(id+map[bool]string{false: "/global", true: "/book"}[book], func(t *testing.T) {
				a, tt, _ := marginFixture(t, true)
				// This orphan is pending independently of the reminder threshold.
				lost := shownEditor(a).AddMark(editor.Mark{Kind: editor.MarkForeshadow, Key: "lost"})
				shownEditor(a).Doc.EditMark(lost, func(m *editor.Mark) { m.Orphaned = true; m.Quote = "gone" })
				tt.Frame()
				for _, on := range []bool{false, true} {
					a.setExtension(id, on, book)
					tt.Frame()
					tt.Frame()
					if (a.extensions.Ledger(id).State == ext.On) != on {
						t.Fatal("switch", id, on)
					}
					for _, point := range []string{string(hostapi.PaneTabs), string(hostapi.SelectionMenus), string(hostapi.PendingGroups), string(hostapi.NameHighlights), string(marksapi.MarginSources)} {
						count := 0
						switch point {
						case string(hostapi.PaneTabs):
							for _, v := range ext.Items(a.extensions, hostapi.PaneTabs) {
								if v.Owner == id {
									count++
								}
							}
						case string(hostapi.SelectionMenus):
							for _, v := range ext.Items(a.extensions, hostapi.SelectionMenus) {
								if v.Owner == id {
									count++
								}
							}
						case string(hostapi.PendingGroups):
							for _, v := range ext.Items(a.extensions, hostapi.PendingGroups) {
								if v.Owner == id {
									count++
								}
							}
						case string(hostapi.NameHighlights):
							for _, v := range ext.Items(a.extensions, hostapi.NameHighlights) {
								if v.Owner == id {
									count++
								}
							}
						case string(marksapi.MarginSources):
							for _, v := range ext.Items(a.extensions, marksapi.MarginSources) {
								if v.Owner == id {
									count++
								}
							}
						}
						declared := false
						for _, v := range a.extensions.Ledger(id).Contributions {
							declared = declared || v.Point == point
						}
						if declared && (count > 0) != on {
							t.Fatal("surface", point, on, count)
						}
					}
					if id == "names" && (len(a.entities()) > 0) != on {
						t.Fatal("completion")
					}
					if id == "hooks" {
						if (len(a.foreshadows()) > 0) != on {
							t.Fatal("reference provider")
						}
						hasPending := slices.ContainsFunc(a.pendingGroups(), func(g pendGroup) bool { return g.id == "marks" })
						if hasPending != on {
							t.Fatal("pending")
						}
					}
					a.openPrefs("extensions")
					a.settings.ext = id
					tt.Frame()
					if id == "names" && tt.HasText(namestext.SettingsMarkNames()) != on {
						t.Fatal("settings")
					}
					a.closePrefs()
				}
			})
		}
	}
}

func TestMarksOffStopsHooksAndNotesAndRemovesEveryMarkIncludingCheck(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	shownEditor(a).AddMark(editor.Mark{Kind: editor.MarkAI, Start: 0, End: 1})
	tt.Frame()
	for _, on := range []bool{false, true} {
		a.setExtension("marks", on, false)
		tt.Frame()
		tt.Frame()
		if a.marksOn() != on || a.checksOn() != on {
			t.Fatal("check prerequisite", on)
		}
		for _, id := range []string{"hooks", "notes"} {
			if (a.extensions.Ledger(id).State == ext.On) != on {
				t.Fatal(id, on)
			}
		}
		var marks []editor.ProseMark
		for _, it := range ext.Items(a.extensions, hostapi.ProseMarks) {
			marks = append(marks, it.Value.Marks(a.write.node, shownEditor(a).Doc.String())...)
		}
		if (len(marks) > 0) != on {
			t.Fatal("prose marks", on, marks)
		}
		if !on && len(checkRanges(a)) > 0 {
			t.Fatal("check marks")
		}
	}
}

func TestMarksAndContextFollowOnlyTheGlobalSwitchAndHaveNoBookMenu(t *testing.T) {
	a, tt, _ := writingFixture(t, "prose")
	for _, id := range []string{"marks", "context"} {
		extensionDetails(t, a, tt, id)
		if _, ok := tt.Find("ext-more-" + id); ok {
			t.Fatal("base extension has a book menu", id)
		}
		if err := a.extensions.SetBook(id, false); err == nil {
			t.Fatal("base extension accepted a book switch", id)
		}
		if _, ok := a.extensions.BookOverride(id); ok {
			t.Fatal("base extension has a book override", id)
		}
	}
}

func TestRemindersMergeHooksAndCheckIntoOnePageAndHideInactiveContent(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	count := 0
	for _, tab := range a.prefTabs {
		if tab.name() == locales.SettingsTabRemind() {
			count++
		}
	}
	if count != 1 {
		t.Fatal("pages", count)
	}
	a.openPrefs("reminders")
	tt.Frame()
	if !tt.HasText(hookstext.SettingsLateAfter()) || !tt.HasText(checktext.SettingsCheckText()) {
		t.Fatal("missing sections")
	}
	a.setExtension("hooks", false, false)
	tt.Frame()
	if tt.HasText(hookstext.SettingsLateAfter()) || !tt.HasText(checktext.SettingsCheckText()) {
		t.Fatal("inactive hooks")
	}
	a.setExtension("marks", false, false)
	tt.Frame()
	if tt.HasText(checktext.SettingsCheckText()) {
		t.Fatal("inactive check")
	}
}

func TestDeclaredSettingsDrawBoolAndChoiceInGroupOrderAndRespectBookScope(t *testing.T) {
	mod := ext.Module{ID: "forms", Setup: func(r *ext.Registrar) {
		ext.Setting(r, "choice", ext.Schema[string]{Title: func() string { return "Choice" }, Choices: []ext.Choice[string]{{Value: "a"}, {Value: "b"}}, Group: "a", Order: 2})
		ext.Setting(r, "flag", ext.Schema[bool]{Title: func() string { return "Flag" }, Group: "a", Order: 1})
		ext.Setting(r, "book", ext.Schema[bool]{Title: func() string { return "Book flag" }, Scope: ext.BookScope, Group: "b"})
	}}
	a, _, _ := fixture(t)
	// Use an isolated host for declaration access and the existing settings palette.
	h, err := ext.New(ext.Options{}, mod)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	saved := a.extensions
	a.extensions = h
	defer func() { a.extensions = saved }()
	if err = h.Open(ext.Book{Dir: a.Store.Root()}); err != nil {
		t.Fatal(err)
	}
	settings := a.simpleSettings("forms")
	if len(settings) != 3 || settings[0].ID != "forms.flag" || settings[1].ID != "forms.choice" {
		t.Fatal(settings)
	}
	tt := stillTester(func(c *ui.Context) { a.drawSimpleSettings(c, "forms") }, 600, 400)
	tt.Frame()
	if !tt.HasText("Flag") || !tt.HasText("a") || !tt.HasText("b") || !tt.HasText("Book flag") {
		t.Fatal(tt.Texts())
	}
	click(t, tt, "Flag")
	if !h.Ledger("forms").Settings[1].Read().(bool) {
		t.Fatal("bool did not persist")
	}
	store := a.Store
	defer func() { a.Store = store }()
	a.Store = nil
	tt.Frame()
	if tt.HasText("Book flag") || len(a.simpleSettings("forms")) != 2 {
		t.Fatal("book setting on shelf")
	}
}

func TestEmptySelectionMenuGroupsDrawNoSeparator(t *testing.T) {
	a, _, _ := writingFixture(t, "prose")
	a.setExtension("ai", false, false)
	a.setExtension("lore", false, false)
	a.setExtension("hooks", false, false)
	menu := &kit.Menu{}
	a.textMenu(menu)
	if len(menu.Rows) != 4 {
		t.Fatal("empty groups left rows", len(menu.Rows))
	}
	for _, r := range menu.Rows {
		if r.Kind != kit.RowItem {
			t.Fatal("empty group separator")
		}
	}
	a.setExtension("hooks", true, false)
	menu = &kit.Menu{}
	a.textMenu(menu)
	if len(menu.Rows) != 7 {
		t.Fatal("hook group did not return", len(menu.Rows))
	}
}
