package desk

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"writer/editor"
	"writer/internal/testshot"
	"writer/workspace"
)

func marginFixture(t *testing.T, compact bool) (*App, *ui.Tester, string) {
	t.Helper()
	lines := []string{"雨还没有停。", "林婉提着灯。", "她独自走进黑巷。", "晚晚停住了脚步。", "旧钥匙掉了出来。", "老周站在门边。"}
	if !compact {
		lines = nil
		for i := range 80 {
			lines = append(lines, fmt.Sprintf("雨夜的第%d盏灯亮着，她沿着街道往前走。", i))
		}
		lines[1], lines[2], lines[12], lines[20], lines[30] = "林婉提着灯。", "她独自走进黑巷。", "晚晚停住了脚步。", "旧钥匙掉了出来。", "老周站在门边。"
	}
	text := strings.Join(lines, "\n")
	a, tt, node := writingFixture(t, text)
	a.doc.tab = paneMargin
	a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "林晚怕黑", Names: []string{"林晚", "晚晚"}})
	e := a.book.Setting().Entries[len(a.book.Setting().Entries)-1]
	u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "她不再怕黑"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.book.Revise(workspace.Revision{By: workspace.ByTalk, Session: a.session, Turn: u.ID, Ops: []workspace.Op{{Op: workspace.OpChange, Entry: e.ID, From: node, Place: workspace.PlaceOpen, Text: "林晚从此不再怕黑"}}}); err != nil {
		t.Fatal(err)
	}
	check := workspace.Check{Of: workspace.ProseHash(text), At: time.Now(), Findings: []workspace.Finding{
		{ID: "new", Kind: workspace.FindNew, Quote: "老周", Name: "老周", Title: "老周", Category: "人物", Text: "站在门边的人"},
		{ID: "conflict", Kind: workspace.FindConflict, Quote: "独自走进黑巷", Entry: e.ID, Title: "林晚怕黑，却独自走进黑巷", Says: "林晚怕黑"},
		{ID: "name", Kind: workspace.FindName, Quote: "林婉", Name: "林晚", Title: "林婉应为林晚"},
		{ID: "gone", Kind: workspace.FindNew, Quote: "原句已经删去", Name: "已删", Title: "已删"},
	}}
	if err = a.book.SaveCheck(node, check); err != nil {
		t.Fatal(err)
	}
	start := utf8.RuneCountInString(text[:strings.Index(text, "旧钥匙")])
	shownEditor(a).AddMark(editor.Mark{Kind: editor.MarkForeshadow, Key: "key", Start: start, End: start + 3, Note: "旧钥匙是谁留下的"})
	tt.Move(0, 0)
	tt.Frame()
	tt.Frame()
	return a, tt, node
}

func marginPositions(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	floor := float32(math.Inf(-1))
	last := -1
	for _, n := range a.marginNotes() {
		if n.start < last {
			t.Fatal("notes are out of prose order")
		}
		last = n.start
		row := a.write.margin.boxes[n.id]
		anchor, ok := shownEditor(a).RangeBoxUnclipped(n.start, n.end)
		want := max(a.LastProse.Y+anchor.Y-3, floor)
		if !ok || math.Abs(float64(row.Y-want)) > 1 {
			t.Fatalf("%s top %v, anchor %v, floor %v", n.id, row.Y, a.LastProse.Y+anchor.Y, floor)
		}
		floor = row.Y + row.H + 8
	}
}

func TestMarginNotesFollowTheProseAndLeaveEightPixels(t *testing.T) {
	a, tt, _ := marginFixture(t, false)
	list := a.marginNotes()
	if len(list) != 5 || list[0].id != "check-name" || list[1].id != "check-conflict" || list[2].kind != "设定待确认" || list[3].fore != "key" || list[4].id != "check-new" {
		t.Fatal("missing, stale or unordered sources", list)
	}
	if list[2].start != utf8.RuneCountInString(strings.Split(shownEditor(a).Doc.String(), "晚晚")[0]) {
		t.Fatal("change did not use the first occurrence of any alias")
	}
	marginPositions(t, a, tt)
	first, second := find(t, tt, "批注-check-name"), find(t, tt, "批注-check-conflict")
	if second.Y != first.Y+first.H+8 {
		t.Fatal("crowded notes need an eight-pixel gap", first, second)
	}
	if tab := find(t, tt, "右栏-批注"); tab.X+10 != first.X+20 {
		t.Fatal("note text and tab text have different left edges")
	}
	before := a.write.margin.notes.builds
	for range 4 {
		tt.Frame()
	}
	if before != a.write.margin.notes.builds || tt.FramePending() {
		t.Fatal("unchanged notes rebuilt or kept asking for frames")
	}
	prose := a.LastProse
	tt.Scroll(prose.X+prose.W/2, prose.Y+prose.H/2, 0, 500)
	marginPositions(t, a, tt)
	if top := a.write.margin.boxes[a.marginNotes()[0].id]; top.Y >= find(t, tt, "页边批注").Y {
		t.Fatal("offscreen annotation pinned to the edge")
	}
	for _, mutate := range []func(){
		func() { a.doc.width = 450 },
		func() { a.sideHidden = true },
		func() { shownEditor(a).SetFontSize(23) },
		func() { tt.SetSize(1420, 820) },
		func() { shownEditor(a).Select(0, 0); shownEditor(a).Insert("门前又下了一夜的雨。\n") },
	} {
		mutate()
		tt.Frame()
		marginPositions(t, a, tt)
		if tt.FramePending() {
			t.Fatal("annotation geometry never settled")
		}
	}
}

func TestMarginActionsUndoAndOpenTheirDetails(t *testing.T) {
	a, tt, node := marginFixture(t, true)
	for _, which := range []struct{ id, action string }{{"check-name", "无需修改"}, {"check-conflict", "无需修改"}, {"check-new", "忽略"}} {
		click(t, tt, "批注-"+which.id+"-"+which.action)
		if a.page != pageWrite || len(a.marginNotes()) != 4 || len(a.write.doubts.val) != 2 {
			t.Fatal("decision did not update the prose and margin together", tt.Texts())
		}
		click(t, tt, "撤销提示")
		if len(a.marginNotes()) != 5 || len(a.write.doubts.val) != 3 {
			t.Fatal("undo did not restore both")
		}
	}
	idea := slices.IndexFunc(a.marginNotes(), func(n marginNote) bool { return n.idea != nil })
	id := a.marginNotes()[idea].id
	for _, act := range []string{"忽略", "采纳"} {
		click(t, tt, "批注-"+id+"-"+act)
		want := 4
		if act == "采纳" {
			want = 3
		}
		if a.page != pageWrite || len(a.marginNotes()) != want {
			t.Fatal("change did not settle related notes", act, tt.Texts())
		}
		click(t, tt, "撤销提示")
		if len(a.marginNotes()) != 5 {
			t.Fatal("undo did not restore change and conflict")
		}
	}
	click(t, tt, "批注-check-new-加入设定")
	if len(a.marginNotes()) != 4 || len(a.listedChecks()) != 2 {
		t.Fatal("adding a setting did not remove its finding")
	}
	hasText(t, tt, "已加入设定")
	if tt.HasText("撤销提示") {
		t.Fatal("adding a setting must not offer undo")
	}
	click(t, tt, "批注-check-conflict-修改正文")
	if a.page != pageWrite || shownEditor(a).SelectedText() != "独自走进黑巷" {
		t.Fatal("modify prose destination")
	}
	click(t, tt, "批注-check-conflict-修改设定")
	if a.doc.tab != paneSetting || a.doc.route.editID() == "" || a.doc.text != "林晚怕黑" {
		t.Fatal("modify setting destination")
	}
	a.doc.route = settingRoute{}
	click(t, tt, "右栏-批注", "批注-check-name")
	if a.page != pageCheck || a.ck.shown != "name" {
		t.Fatal("finding detail destination")
	}
	a.findInChapter(node, "旧钥匙")
	a.doc.tab = paneMargin
	tt.Frame()
	click(t, tt, "批注-fore-key")
	if a.doc.tab != paneFore || a.write.fore.shown != "key" {
		t.Fatal("foreshadow detail destination")
	}
	click(t, tt, "右栏-批注", "批注-"+id)
	if a.doc.tab != paneSetting || !a.doc.route.showingWaiting() {
		t.Fatal("setting suggestion destination")
	}
}

func TestIgnoringPendingKeepsTheMarginAndAILeavesOnlyForeshadowing(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	click(t, tt, "待处理", "全部忽略")
	if len(a.marginNotes()) != 5 || len(a.write.doubts.val) != 3 {
		t.Fatal("ignoring pending erased margin notes")
	}
	click(t, tt, "撤销提示")
	a.shelf.NoAI = true
	tt.Frame()
	if list := a.marginNotes(); len(list) != 1 || list[0].fore != "key" || len(a.write.doubts.val) != 0 {
		t.Fatal("AI-off margin includes AI data", list)
	}
	a.page = pagePlan
	a.plan.Pane = true
	tt.Frame()
	if a.doc.tab != paneMargin || !tt.HasText("右栏设定标题") || tt.HasText("页边批注") {
		t.Fatal("outline did not substitute setting while keeping margin")
	}
	a.page = pageWrite
	tt.Frame()
	if !tt.HasText("页边批注") || a.doc.tab != paneMargin {
		t.Fatal("margin choice was lost")
	}
}

func TestMarginAndUnderlinedProsePointToEachOther(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	tt.Move(0, 0)
	base := tt.Image()
	n := a.marginNotes()[0]
	row := find(t, tt, "批注-"+n.id)
	r, _ := shownEditor(a).RangeBox(n.start, n.end)
	r.X, r.Y, r.W = r.X+a.LastProse.X, r.Y+a.LastProse.Y, 30
	tt.Move(row.X+24, row.Y+14)
	if a.write.margin.lit != n.id || !pixelsChanged(base, tt.Image(), r) {
		t.Fatal("margin hover did not shade the quoted text")
	}
	tt.Move(0, 0)
	if pixelsChanged(base, tt.Image(), r) {
		t.Fatal("highlight remained after pointer left")
	}
	for _, note := range a.marginNotes() {
		if note.check == nil && note.fore == "" {
			continue
		}
		box, _ := shownEditor(a).RangeBox(note.start, note.end)
		tt.Move(a.LastProse.X+box.X+2, a.LastProse.Y+box.Y+box.H/2)
		if a.write.margin.lit != note.id {
			t.Fatal("underlined prose did not light its note", note.id, a.write.margin.lit)
		}
	}
}

func TestShotMargin(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := marginFixture(t, true)
	shot(t, tt, "margin.png")
	row := find(t, tt, "批注-check-conflict")
	tt.Move(row.X+24, row.Y+14)
	shot(t, tt, "margin-hover.png")
	a.shelf.NoAI = true
	shot(t, tt, "margin-ai-off.png")
	a, tt, _ = marginFixture(t, false)
	prose := a.LastProse
	tt.Scroll(prose.X+prose.W/2, prose.Y+prose.H/2, 0, 500)
	shot(t, tt, "margin-scrolled.png")
}

func TestMarginForeshadowPayoffsAndEmptyChapters(t *testing.T) {
	a, tt, one := writingFixture(t, "旧钥匙掉了出来。")
	shownEditor(a).AddMark(editor.Mark{Kind: editor.MarkForeshadow, Key: "key", Start: 0, End: 3, Note: "旧钥匙"})
	if err := a.saveText(); err != nil {
		t.Fatal(err)
	}
	two, err := a.book.NewChapter("", "第2章：门", false)
	if err != nil {
		t.Fatal(err)
	}
	a.openNode(two)
	shownEditor(a).Insert("她用旧钥匙打开了门。")
	shownEditor(a).AddMark(editor.Mark{Kind: editor.MarkPayoff, Key: "pay", Ref: "key", Start: 2, End: 5})
	a.doc.tab = paneMargin
	tt.Frame()
	list := a.marginNotes()
	if len(list) != 1 || list[0].kind != "伏笔 · 在此回收" || list[0].text != "埋在第一章" || list[0].start != 2 {
		t.Fatal("payoff does not identify its buried chapter", list)
	}
	a.openNode(one)
	tt.Frame()
	if list = a.marginNotes(); len(list) != 1 || list[0].kind != "伏笔 · 在此埋下，已回收" {
		t.Fatal("buried note did not reflect its payoff", list)
	}
	a.openNode("")
	tt.Frame()
	hasText(t, tt, "打开一章后，这里列出它的批注")
	vol, err := a.book.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	a.openNode(vol)
	tt.Frame()
	hasText(t, tt, "打开一章后，这里列出它的批注")
	blank, err := a.book.NewChapter("", "空章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.openNode(blank)
	tt.Frame()
	hasText(t, tt, "本章没有批注")
}

func TestMarginChangeWithoutANameStartsAtTheChapter(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	ed := shownEditor(a)
	ed.Select(0, ed.Doc.Len())
	ed.Insert("门前只剩下一盏旧灯。")
	tt.Frame()
	list := a.marginNotes()
	if len(list) != 1 || list[0].idea == nil || list[0].start != 0 || list[0].end != 0 {
		t.Fatal("unnamed change did not use chapter start", list)
	}
	marginPositions(t, a, tt)
}

func TestMarginRequestsItsOwnCorrectionInTheNextFrame(t *testing.T) {
	a, _, _ := marginFixture(t, false)
	var frames []bool
	tt := stillTester(func(c *ui.Context) {
		// A glyph-atlas refill can paint the same frame again. Count the
		// built view once, then keep its last painted alignment.
		frame := len(frames)
		frames = append(frames, false)
		ui.Box(c).Fill().Children(func() { a.View(c) }).DrawOver(func(_ *ui.Painter, _ ui.Rect) {
			aligned := true
			floor := float32(math.Inf(-1))
			for _, n := range a.marginNotes() {
				box := a.write.margin.boxes[n.id]
				r, _ := shownEditor(a).RangeBoxUnclipped(n.start, n.end)
				top := max(a.LastProse.Y+r.Y-3, floor)
				aligned = aligned && math.Abs(float64(box.Y-top)) <= 1
				floor = box.Y + box.H + 8
			}
			frames[frame] = aligned
		})
	}, 1266, 780)
	tt.Frame()
	for _, mutate := range []func(){
		func() { a.doc.width = 450 },
		func() { a.sideHidden = true },
		func() { shownEditor(a).SetFontSize(23) },
		func() { shownEditor(a).Select(0, 0); shownEditor(a).Insert("她站在门边，迟迟没有动。\n") },
	} {
		frames = nil
		mutate()
		tt.Frame()
		if len(frames) == 0 || !frames[len(frames)-1] || len(frames) > 2 || tt.FramePending() {
			t.Fatal("the next frame must correct the notes without another input", frames)
		}
	}
}

func TestTypingKeepsMarginNotesApartFromTheFirstFrame(t *testing.T) {
	a, _, _ := marginFixture(t, true)
	ed := shownEditor(a)
	ed.Select(0, 0)
	ed.Focus()
	checking, frames := false, 0
	tt := stillTester(func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() { a.View(c) }).DrawOver(func(_ *ui.Painter, _ ui.Rect) {
			if !checking {
				return
			}
			frames++
			floor := float32(math.Inf(-1))
			list := a.marginNotes()
			if len(list) != 5 {
				t.Fatal("typing lost a margin note")
			}
			for _, n := range list {
				box := a.write.margin.boxes[n.id]
				if box.H <= 0 || box.Y < floor-0.5 {
					t.Fatalf("frame %d: %s at %v overlaps the preceding note ending at %v", frames, n.id, box, floor-8)
				}
				floor = box.Y + box.H + 8
			}
		})
	}, 1266, 780)
	tt.Frame()
	checking = true
	for range 4 {
		frames = 0
		before := a.write.margin.notes.builds
		tt.Type("雨")
		if frames == 0 || a.write.margin.notes.builds <= before {
			t.Fatal("typing did not render rebuilt margin notes")
		}
	}
}
