package desk

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"writer/desk/kit"
	"writer/export"
	"writer/workspace"
)

func TestExportPlanChangesWithChoices(t *testing.T) {
	a, tt, _ := writingFixture(t, "一二三")
	for _, title := range []string{"第二章", "第三章"} {
		if _, err := a.Store.NewChapter("", title, false); err != nil {
			t.Fatal(err)
		}
	}
	showPage(a, pageTalk)
	a.sideHidden, a.doc.hidden = true, true
	tt.Frame()
	a.exporting.show()
	tt.Frame()
	x := &a.exporting
	x.plan.Skip["cache"] = "kept"
	tt.Frame()
	if x.plan.Skip["cache"] != "kept" {
		t.Fatal("drawing rebuilt the plan")
	}
	change := func(labels ...string) {
		t.Helper()
		x.plan.Skip["cache"] = "kept"
		for _, label := range labels {
			if label == "从" || label == "到" {
				// The caption and the control share a label; click below the caption.
				r, ok := tt.Find(label)
				if !ok {
					t.Fatal(label)
				}
				tt.ClickAt(r.X+30, r.Y+r.H+6+kit.HControl/2)
			} else if err := tt.Click(label); err != nil {
				t.Fatal(err, tt.Texts())
			}
			tt.Frame()
		}
		if x.plan.Skip["cache"] != "" {
			t.Fatal("choice did not rebuild the plan", labels)
		}
	}
	for _, label := range []string{"正文", "正文", "大纲", "大纲", "设定", "设定", "纯文本", "指定范围"} {
		change(label)
	}
	change("到", "第二章")
	change("从", "第三章")
	if x.selection.To != "第三章" || x.plan.Part != "第三章" {
		t.Fatal("end was not moved with the start", x.selection, x.plan)
	}
	change("从", "第一章")
	if x.plan.Part != "第一章至第三章" {
		t.Fatal(x.plan.Part)
	}
	change("Word")
}

func TestExportKeepsItsOpenedTextAndTracksContext(t *testing.T) {
	a, tt, id := writingFixture(t, "一二三")
	if a.shelf.Export != nil {
		t.Fatal("a closed export dialog created preferences")
	}
	a.exporting.show()
	x := &a.exporting
	if err := a.Store.RenameChapter(id, "磁盘新名"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.NewChapter("", "磁盘新章", false); err != nil {
		t.Fatal(err)
	}
	a.checkDisk()
	tt.Frame()
	if !slices.Equal(x.plan.Names, []string{"第一章"}) || x.plan.Words != 3 {
		t.Fatal("disk edits replaced the opened export snapshot", x.plan)
	}
	if err := a.Store.TrashChapter(id); err != nil {
		t.Fatal(err)
	}
	a.checkDisk()
	tt.Frame()
	if x.plan.Current != id || !slices.Equal(x.plan.Names, []string{"第一章"}) {
		t.Fatal("deletion changed the opened export snapshot", x.plan)
	}
	showPage(a, pageTalk)
	tt.Frame()
	if x.plan.Current != "" || slices.Contains(x.plan.Scopes, export.ScopeChapter) {
		t.Fatal("page change left a stale current chapter", x.plan)
	}
	entry := setting(a).Entries[0].ID
	if _, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpMove, Entry: entry, Place: workspace.PlaceDropped}}}); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(a.Store.Root(), "sessions", a.session+".jsonl")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, []byte("{broken\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err == nil || x.plan.Skip[export.SettingContent] != "还没有已定的设定，不导出设定。" {
		t.Fatal("a later refresh error left the setting plan stale", err, x.plan)
	}
	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	if x.plan.Skip[export.SettingContent] != "还没有已定的设定，不导出设定。" {
		t.Fatal("setting refresh did not update the plan", x.plan)
	}
	other, err := workspace.Create(filepath.Join(t.TempDir(), "other"), "另一部", "网文")
	if err != nil {
		t.Fatal(err)
	}
	other.Close()
	if err := a.Open(other.Root()); err != nil {
		t.Fatal(err)
	}
	if a.exporting.open {
		t.Fatal("switching works left the export dialog open")
	}
}

func TestExportFallsBackWhileRangeIsHidden(t *testing.T) {
	a, _, _ := writingFixture(t, "一二三")
	a.exporting.show()
	x := &a.exporting
	x.selection.Scope = export.ScopeChapter
	a.exporting.replan()
	if x.plan.Scope != export.ScopeChapter {
		t.Fatal(x.plan)
	}
	a.exporting.preferences().Options = export.Options{Setting: true}
	showPage(a, pageTalk)
	a.exporting.replan()
	if x.selection.Scope != export.ScopeWork || x.plan.Scope != export.ScopeWork {
		t.Fatal("hidden range did not follow the plan", x.selection, x.plan)
	}
	showPage(a, pageWrite)
	a.exporting.preferences().Prose = true
	a.exporting.replan()
	if x.selection.Scope != export.ScopeWork {
		t.Fatal(x.selection)
	}
}
