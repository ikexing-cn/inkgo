package exportdialog

import (
	"slices"
	"testing"

	"writer/desk/kit"
	"writer/export"
	"writer/workspace"
)

func TestExportPlanChangesWithChoices(t *testing.T) {
	h, x, tt, _ := dialogFixture(t, "一二三")
	for _, title := range []string{"第二章", "第三章"} {
		if _, err := h.store.NewChapter("", title, false); err != nil {
			t.Fatal(err)
		}
	}
	h.current = ""
	tt.Frame()
	x.Show()
	tt.Frame()
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

func TestLaterLinesFromChapters(t *testing.T) {
	h, _, _, _ := dialogFixture(t, "一二三")
	if _, err := h.store.NewChapter("", "第2章：井沿", false); err != nil {
		t.Fatal(err)
	}
	three, err := h.store.NewChapter("", "第3章：停电", false)
	if err != nil {
		t.Fatal(err)
	}
	added, err := h.store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "配角，林晚的同事。左手总揣在兜里，从来不拿出来。"}}})
	if err != nil {
		t.Fatal(err)
	}
	id := added.Ops[0].Entry
	if _, err := h.store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpChange, Entry: id, From: three, Place: workspace.PlaceBody, Text: "配角，学会了修灯。"}}}); err != nil {
		t.Fatal(err)
	}
	if err := h.content.ReloadTree(); err != nil {
		t.Fatal(err)
	}
	if err := h.content.ReloadSetting(); err != nil {
		t.Fatal(err)
	}
	e, _ := h.content.Setting().Entry(id)
	if got := laterLines(h.content, e); !slices.Equal(got, []string{"第3章起：配角，学会了修灯。"}) {
		t.Fatalf("exports %q", got)
	}
}

func TestExportKeepsItsOpenedText(t *testing.T) {
	h, x, tt, id := dialogFixture(t, "一二三")
	if h.prefs.Export != nil {
		t.Fatal("a closed export dialog created preferences")
	}
	x.Show()
	if err := h.store.RenameChapter(id, "磁盘新名"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.NewChapter("", "磁盘新章", false); err != nil {
		t.Fatal(err)
	}
	if err := h.content.ReloadTree(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !slices.Equal(x.plan.Names, []string{"第一章"}) || x.plan.Words != 3 {
		t.Fatal("disk edits replaced the opened export snapshot", x.plan)
	}
	if err := h.store.TrashChapter(id); err != nil {
		t.Fatal(err)
	}
	if err := h.content.ReloadTree(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if x.plan.Current != id || !slices.Equal(x.plan.Names, []string{"第一章"}) {
		t.Fatal("deletion changed the opened export snapshot", x.plan)
	}
}

func TestExportKeepsTheWorksPenWhileProseIsHidden(t *testing.T) {
	h, x, _, _ := dialogFixture(t, "雨下着。")
	work := h.store.Work()
	work.Pen = "雨声"
	if err := h.store.SaveWork(work); err != nil {
		t.Fatal(err)
	}
	x.preferences().Prose, x.preferences().Outline = false, true
	x.Show()
	if x.pen != "雨声" {
		t.Fatal("the pen name or why the outline cannot go", x.pen)
	}
}

func TestExportFallsBackWhileRangeIsHidden(t *testing.T) {
	h, x, _, id := dialogFixture(t, "一二三")
	x.Show()
	x.selection.Scope = export.ScopeChapter
	x.Replan()
	if x.plan.Scope != export.ScopeChapter {
		t.Fatal(x.plan)
	}
	x.preferences().Options = export.Options{Setting: true}
	h.current = ""
	x.Replan()
	if x.selection.Scope != export.ScopeWork || x.plan.Scope != export.ScopeWork {
		t.Fatal("hidden range did not follow the plan", x.selection, x.plan)
	}
	h.current = id
	x.preferences().Prose = true
	x.Replan()
	if x.selection.Scope != export.ScopeWork {
		t.Fatal(x.selection)
	}
}
