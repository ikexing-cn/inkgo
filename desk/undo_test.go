package desk

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"writer/internal/testshot"
	"writer/prefs"
	"writer/workspace"
)

func diskBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCheckDecisionsUndoTheChapterResult(t *testing.T) {
	for _, which := range []int{0, 1, 2} {
		t.Run(string(rune('0'+which)), func(t *testing.T) {
			a, tt, node, _ := checkFixture(t)
			click(t, tt, "立即检查")
			until(t, tt, "check", func() bool { return len(a.checkItems(false)) == 3 })
			original, err := a.Store.ReadCheck(node)
			if err != nil {
				t.Fatal(err)
			}
			a.openCheck(a.checkItems(false)[which].f.ID)
			tt.Frame()
			label, notice := "无需修改", "已标为「无需修改」"
			if which == 2 {
				label, notice = "忽略", "已忽略"
			}
			click(t, tt, label)
			hasText(t, tt, notice)
			changed, err := a.Store.ReadCheck(node)
			if err != nil || changed.Equal(original) {
				t.Fatal("decision did not write")
			}
			click(t, tt, "撤销提示")
			restored, err := a.Store.ReadCheck(node)
			if err != nil || !original.Equal(restored) || len(a.checkItems(false)) != 3 || tt.HasText("撤销提示") {
				t.Fatal("undo did not restore the check and close its toast", tt.Texts())
			}
		})
	}
}

func TestChangeDecisionsUndoTogether(t *testing.T) {
	for _, take := range []bool{false, true} {
		t.Run(map[bool]string{false: "ignore", true: "take"}[take], func(t *testing.T) {
			a, tt, node := writingFixture(t, "林晚不再怕黑。")
			e := a.book.Setting().Entries[0]
			u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "她不再怕黑"})
			if err != nil {
				t.Fatal(err)
			}
			r, err := a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: a.session, Turn: u.ID, Ops: []workspace.Op{
				{Op: workspace.OpChange, Entry: e.ID, From: node, Place: workspace.PlaceOpen, Text: "主角不再怕黑"},
			}})
			if err != nil {
				t.Fatal(err)
			}
			k := workspace.Check{Of: workspace.ProseHash("林晚不再怕黑。"), At: time.Now(), Findings: []workspace.Finding{
				{ID: "conflict", Kind: workspace.FindConflict, Entry: e.ID, Quote: "不再怕黑", Says: "怕黑", Title: "怕黑"},
				{ID: "unrelated", Kind: workspace.FindNew, Name: "林晚", Quote: "林晚", Title: "林晚", Category: "人物"},
			}}
			if err := a.book.SaveCheck(node, k); err != nil {
				t.Fatal(err)
			}
			if err := a.refresh(); err != nil {
				t.Fatal(err)
			}
			settingPath := filepath.Join(a.Store.Root(), "setting.jsonl")
			before := diskBytes(t, settingPath)
			beforeCheck, err := a.Store.ReadCheck(node)
			if err != nil {
				t.Fatal(err)
			}
			showTodo(a, true)
			tt.Frame()
			label := "忽略-"
			if take {
				label = "采纳-"
			}
			click(t, tt, label+r.Ops[0].Change)
			if take {
				hasText(t, tt, "已记到「人物」· 第一章起")
				got, err := a.Store.ReadCheck(node)
				if err != nil || len(got.Findings) != 1 || got.Findings[0].ID != "unrelated" {
					t.Fatal(got, err)
				}
			} else {
				hasText(t, tt, "已忽略")
			}
			click(t, tt, "撤销提示")
			restored, err := a.Store.ReadCheck(node)
			if err != nil || !bytes.Equal(before, diskBytes(t, settingPath)) || !beforeCheck.Equal(restored) || len(a.ideas()) != 1 || len(a.checkItems(false)) != 2 {
				t.Fatal("the setting and findings did not come back together", tt.Texts())
			}
		})
	}
}

func TestIgnorePendingOnlyHidesTheListAndUndoes(t *testing.T) {
	a, tt, _, _ := checkFixture(t)
	click(t, tt, "立即检查")
	until(t, tt, "check", func() bool { return len(a.checkItems(false)) == 3 })
	a.savePrefs()
	before := maps.Clone(a.write.ignored)
	click(t, tt, "待处理", "全部忽略")
	hasText(t, tt, "已忽略 3 条待处理")
	a.markDoubts()
	if len(a.pendingGroups()) != 0 || len(a.checkItems(false)) != 3 || len(a.write.doubts.val) == 0 {
		t.Fatal("ignore changed more than the list")
	}
	other, err := a.book.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.openNode(other)
	a.doc.width, a.doc.hidden = 450, true
	if err := a.savePrefs(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tabs := slices.Clone(a.write.tabs)
	click(t, tt, "撤销提示")
	saved := prefs.Load(a.Store.PrefsPath())
	if !maps.Equal(before, a.write.ignored) || !slices.Equal(saved.Ignored, a.write.ignoredKeys()) || len(a.pendingGroups()) != 1 {
		t.Fatal("pending undo did not restore disk and list", tt.Texts())
	}
	if a.write.node != other || saved.Chapter != other || !slices.Equal(saved.Tabs, tabs) || !slices.Equal(a.write.tabs, tabs) || saved.PanelWidth != 450 || !saved.SettingHidden {
		t.Fatal("pending undo changed navigation or pane preferences", saved)
	}
}

func TestShotUndoAtTheFoot(t *testing.T) {
	testshot.Dir(t)
	a, tt, _, _ := checkFixture(t)
	click(t, tt, "立即检查")
	until(t, tt, "check", func() bool { return len(a.checkItems(false)) == 3 })
	click(t, tt, "待处理", "全部忽略")
	shot(t, tt, "toast-undo-bottom.png")
}

func TestUndoToastAtTheBottom(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("雨下着。", 60))
	a.status.Did("已忽略", func() error { return nil })
	tt.Frame()
	tt.Frame()
	undo := find(t, tt, "撤销提示")
	foot := find(t, tt, "底栏")
	if undo.Y < 600 || undo.Y+undo.H > foot.Y-8 {
		t.Fatalf("undo toast is not above footer: %v %v", undo, foot)
	}
}

func TestUndoLeavesALaterWriteAlone(t *testing.T) {
	a, tt, node := writingFixture(t, "雨停了。")
	e := a.book.Setting().Entries[0]
	u, _ := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "不再怕黑"})
	r, err := a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: a.session, Turn: u.ID, Ops: []workspace.Op{
		{Op: workspace.OpChange, Entry: e.ID, From: node, Place: workspace.PlaceOpen, Text: "不再怕黑"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	showTodo(a, true)
	tt.Frame()
	click(t, tt, "忽略-"+r.Ops[0].Change)
	if _, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{
		{Op: workspace.OpEdit, Entry: e.ID, Text: "后来又改过的设定"},
	}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.Store.Root(), "setting.jsonl")
	after := diskBytes(t, path)
	click(t, tt, "撤销提示")
	if !bytes.Equal(after, diskBytes(t, path)) || !tt.HasText("设定已再次修改") {
		t.Fatal("undo overwrote a later write", tt.Texts())
	}
}

func TestNewToastIsBelowTheOlderOnes(t *testing.T) {
	a, tt, _ := fixture(t)
	a.status.Did("第一次")
	tt.Frame()
	tt.Frame()
	a.status.Hint("较新的提示")
	tt.Frame()
	tt.Frame()
	if find(t, tt, "第一次").Y >= find(t, tt, "较新的提示").Y {
		t.Fatal("newer hint is not below notice")
	}
	a.status.Did("第二次")
	tt.Frame()
	tt.Frame()
	if find(t, tt, "第二次").Y <= find(t, tt, "较新的提示").Y {
		t.Fatal("a renewed notice kept its old position")
	}
}

func TestUndoRefusesAReplacedChapterCheck(t *testing.T) {
	for _, take := range []bool{false, true} {
		t.Run(map[bool]string{false: "finding", true: "change"}[take], func(t *testing.T) {
			a, tt, node, _ := checkFixture(t)
			click(t, tt, "立即检查")
			until(t, tt, "check", func() bool { return len(a.checkItems(false)) == 3 })
			finding := a.checkItems(false)[0]
			if take {
				u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "她不再怕黑"})
				if err != nil {
					t.Fatal(err)
				}
				r, err := a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: a.session, Turn: u.ID, Ops: []workspace.Op{{
					Op: workspace.OpChange, Entry: finding.entry.ID, From: node, Place: workspace.PlaceOpen, Text: "她不再怕黑",
				}}})
				if err != nil {
					t.Fatal(err)
				}
				if err := a.refresh(); err != nil {
					t.Fatal(err)
				}
				showTodo(a, true)
				tt.Frame()
				click(t, tt, "采纳-"+r.Ops[0].Change)
			} else {
				a.openCheck(finding.f.ID)
				tt.Frame()
				click(t, tt, "无需修改")
			}
			later, err := a.Store.ReadCheck(node)
			if err != nil {
				t.Fatal(err)
			}
			later.At = later.At.Add(time.Second)
			if err := a.book.SaveCheck(node, later); err != nil {
				t.Fatal(err)
			}
			settingPath := filepath.Join(a.Store.Root(), "setting.jsonl")
			checkPath := filepath.Join(a.Store.Root(), "chapters", "第一章.check.json")
			settingBytes, checkBytes := diskBytes(t, settingPath), diskBytes(t, checkPath)
			click(t, tt, "撤销提示")
			if !tt.HasText("本章检查结果已再次修改") || !bytes.Equal(settingBytes, diskBytes(t, settingPath)) || !bytes.Equal(checkBytes, diskBytes(t, checkPath)) {
				t.Fatal("refused undo changed the setting or the new check", tt.Texts())
			}
		})
	}
}

func TestTakingAChangeRollsBackWhenTheCheckCannotSave(t *testing.T) {
	a, tt, node := writingFixture(t, "林晚不再怕黑。")
	e := a.book.Setting().Entries[0]
	u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "不再怕黑"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: a.session, Turn: u.ID, Ops: []workspace.Op{{
		Op: workspace.OpChange, Entry: e.ID, From: node, Place: workspace.PlaceOpen, Text: "不再怕黑",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	// An old result can be read but cannot be saved with an unknown kind.
	k := workspace.Check{Of: workspace.ProseHash("林晚不再怕黑。"), Findings: []workspace.Finding{
		{ID: "conflict", Kind: workspace.FindConflict, Entry: e.ID, Quote: "不再怕黑", Title: "怕黑"},
		{ID: "old", Kind: "unknown", Quote: "林晚", Title: "林晚"},
	}}
	data, err := json.Marshal(k)
	if err != nil {
		t.Fatal(err)
	}
	checkPath := filepath.Join(a.Store.Root(), "chapters", "第一章.check.json")
	if err := os.WriteFile(checkPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.book.ReloadChecks(); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	settingPath := filepath.Join(a.Store.Root(), "setting.jsonl")
	before := diskBytes(t, settingPath)
	showTodo(a, true)
	tt.Frame()
	click(t, tt, "采纳-"+r.Ops[0].Change)
	if !bytes.Equal(before, diskBytes(t, settingPath)) || !bytes.Equal(data, diskBytes(t, checkPath)) || len(a.ideas()) != 1 || a.status.CanUndo() || a.errorText() == "" {
		t.Fatal("failed adoption left half a decision", tt.Texts())
	}
}
