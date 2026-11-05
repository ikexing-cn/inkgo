package desk

import (
	"slices"
	"testing"
	"time"
	"writer/ext/propose"

	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/find/findapi"
	"writer/ext/hostapi"
	"writer/workspace"
)

func TestDisabledFileExtensionsRemoveOnlyTheirCommandsAndOverlays(t *testing.T) {
	for _, id := range []string{"snaps", "trash"} {
		for _, book := range []bool{false, true} {
			t.Run(id+map[bool]string{false: "Local", true: "Book"}[book], func(t *testing.T) {
				a, tt, _ := writingFixture(t, "正文")
				if id == "snaps" {
					a.openSnapshots()
				} else {
					a.openTrash()
				}
				a.setExtension(id, false, book)
				tt.Frame()
				for _, c := range a.extensionCommands() {
					if len(c.id) > len(id) && c.id[:len(id)+1] == id+"." {
						t.Fatal("command survived", c.id)
					}
				}
				for _, c := range ext.Items(a.extensions, hostapi.Overlays) {
					if c.Owner == id {
						t.Fatal("overlay survived", c.ID)
					}
				}
				if a.extensions.Ledger(id).State == ext.On {
					t.Fatal("still on")
				}
			})
		}
	}
}

func TestSnapshotsOffStillTakesSnapshotsOnSaveAndProposalAcceptance(t *testing.T) {
	a, tt, id := writingFixture(t, "她走到井边。")
	a.setExtension("snaps", false, false)
	a.write.cur().lastSnap = time.Now().Add(-2 * snapEvery)
	shownEditor(a).Insert("雨。")
	a.saveNow()
	snaps, err := a.Store.Snapshots(id)
	if err != nil || !slices.ContainsFunc(snaps, func(s workspace.Snapshot) bool { return s.Reason == workspace.SnapInterval }) {
		t.Fatal(snaps, err)
	}
	proposeOf(a).Frame.Session = talkOf(a).Session
	proposeOf(a).TakeProposal("snapshot-off", workspace.Proposal{Chapter: id, At: workspace.ProposeReplace, Quote: "她走到井边。", Text: "她慢慢走到井边。"}, false, "test-model")
	tt.Frame()
	if len(proposeOf(a).Props) == 0 {
		t.Fatal("no proposal")
	}
	proposeOf(a).TakeProps([]*propose.Proposal{proposeOf(a).Props[0]})
	snaps, err = a.Store.Snapshots(id)
	if err != nil || !slices.ContainsFunc(snaps, func(s workspace.Snapshot) bool { return s.Reason == workspace.SnapAI }) {
		t.Fatal(snaps, err)
	}
}

func TestTrashOffStillMovesTreeNodesIntoCoreTrash(t *testing.T) {
	a, tt, id := writingFixture(t, "正文")
	a.setExtension("trash", false, false)
	if err := tt.RightClick("章节-第一章"); err != nil {
		t.Fatal(err)
	}
	if err := pick(tt, "移到回收站"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	items, err := a.Store.Trash()
	if err != nil || len(items) != 1 || items[0].ID != id {
		t.Fatal(items, err)
	}
	if _, ok := a.book.Tree().Node(id); ok {
		t.Fatal("tree kept trashed node")
	}
}

func TestFindOffClosesBarAndRemovesKeysMenusToolsAndProsePoint(t *testing.T) {
	for _, book := range []bool{false, true} {
		t.Run(map[bool]string{false: "Local", true: "Book"}[book], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "林晚")
			tt.Key(ui.Cmd, ui.KeyF)
			tt.Frame()
			if !findOf(a).Finding {
				t.Fatal("not open")
			}
			a.setExtension("find", false, book)
			tt.Frame()
			if findOf(a).Finding || tt.HasText("替换为") {
				t.Fatal("bar survived")
			}
			for _, c := range a.commandList() {
				if c.name(a) == "查找替换" {
					t.Fatal("command survived")
				}
			}
			for _, r := range a.editCommands() {
				if r.id == "find.find" {
					t.Fatal("menu survived")
				}
			}
			for _, i := range ext.Items(a.extensions, hostapi.Tools) {
				if i.Owner == "find" {
					t.Fatal("tool survived")
				}
			}
			if hasOwner(ext.Items(a.extensions, hostapi.BeforeProse), "find") || hasOwner(ext.Items(a.extensions, hostapi.WritingKeys), "find") {
				t.Fatal("points survived")
			}
			tt.Key(ui.Cmd, ui.KeyF)
			tt.Frame()
			if findOf(a).Finding {
				t.Fatal("key survived")
			}
		})
	}
}

// renameForFindReview changes a lore name through its real edit form.
func renameForFindReview(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角修路灯", Names: []string{"林晚"}})
	e := setting(a).Entries[len(setting(a).Entries)-1]
	editEntry(a, e.ID)
	lorePanel(a).Doc.Text, lorePanel(a).Doc.Category, lorePanel(a).Doc.Names = e.Text, e.Category, "沈砚"
	tt.Frame()
	click(t, tt, "保存")
	if !tt.HasText("正文中仍有 2 处「林晚」") {
		t.Fatal("missing rename notice", tt.Texts())
	}
}

func TestFindOffHidesLoreReplacementButKeepsOldNameAndSearchCounting(t *testing.T) {
	for _, book := range []bool{false, true} {
		t.Run(map[bool]string{false: "Local", true: "Book"}[book], func(t *testing.T) {
			a, tt, id := writingFixture(t, "林晚走了，林晚回来了。")
			a.setExtension("find", false, book)
			renameForFindReview(t, a, tt)
			if _, ok := tt.Find("将旧名替换为新名"); ok {
				t.Fatal("disabled replacement link visible")
			}
			if _, ok := tt.Find("保留旧名"); !ok {
				t.Fatal("keep-old action missing")
			}
			if n := (extensionHost{a}).CountText("林晚"); n != 2 {
				t.Fatal("host search unavailable", n)
			}
			if _, ok := ext.Lookup(a.extensions, findapi.ReplacementService); ok {
				t.Fatal("disabled find service available")
			}
			if a.extensions.Ledger("lore").State != ext.On {
				t.Fatal("optional consumer stopped")
			}
			ch, err := a.Store.ReadChapter(id)
			if err != nil || ch.Text != "林晚走了，林晚回来了。" || shownEditor(a).Doc.String() != ch.Text {
				t.Fatal("prose changed without confirmation", ch, err)
			}
			// The optional ref updates without restarting lore or losing its rename notice.
			a.setExtension("find", true, book)
			tt.Frame()
			if _, ok := tt.Find("将旧名替换为新名"); !ok {
				t.Fatal("review link did not return")
			}
			a.setExtension("find", false, book)
			tt.Frame()
			if _, ok := tt.Find("将旧名替换为新名"); ok {
				t.Fatal("review link survived second disable")
			}
			click(t, tt, "保留旧名")
			if lorePanel(a).RenameCount() != 0 || findOf(a).Finding {
				t.Fatal("keep-old opened find or kept notice")
			}
			if shownEditor(a).Doc.String() != ch.Text {
				t.Fatal("keep-old changed prose")
			}
		})
	}
}

func TestLoreReplacementOpensOptionalFindReviewAndWaitsForConfirmation(t *testing.T) {
	a, tt, id := writingFixture(t, "林晚走了，林晚回来了。")
	renameForFindReview(t, a, tt)
	click(t, tt, "将旧名替换为新名")
	f := findOf(a)
	if !f.Finding || !f.Wide || f.Query != "林晚" || f.With != "沈砚" || shownChapter(a) != id {
		t.Fatal("find not prefilled")
	}
	original := "林晚走了，林晚回来了。"
	if shownEditor(a).Doc.String() != original {
		t.Fatal("review changed prose")
	}
	click(t, tt, "全书替换")
	ch, err := a.Store.ReadChapter(id)
	if err != nil || ch.Text != original {
		t.Fatal("confirmation changed prose", ch, err)
	}
	click(t, tt, "确定替换")
	ch, err = a.Store.ReadChapter(id)
	if err != nil || ch.Text != "沈砚走了，沈砚回来了。" {
		t.Fatal(ch, err)
	}
}

func TestLoreRecordsOptionalFindDependencyAndReplacementServiceUse(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文")
	for _, on := range []bool{true, false, true} {
		a.setExtension("find", on, false)
		tt.Frame()
		l := a.extensions.Ledger("lore")
		if !slices.Contains(l.Optional, ext.Dependency{ID: "find", Active: on}) {
			t.Fatal("optional dependency missing", l.Optional)
		}
		if !slices.Contains(l.Uses, ext.ServiceUse{Key: string(findapi.ReplacementService), Optional: true}) {
			t.Fatal("optional service use missing", l.Uses)
		}
		if l.State != ext.On {
			t.Fatal("lore depends on find being active")
		}
		if !slices.Contains(a.extensions.Ledger("find").Provides, string(findapi.ReplacementService)) {
			t.Fatal("find service missing from ledger")
		}
	}
}

func TestExportUsesSnapshotAnchorWhenPresentAndOrderWhenMissing(t *testing.T) {
	a, _, _ := writingFixture(t, "正文")
	for _, on := range []bool{true, false, true} {
		a.setExtension("snaps", on, false)
		var ids []string
		for _, r := range a.fileCommands() {
			ids = append(ids, r.id)
		}
		at := slices.Index(ids, "export.open")
		if at < 0 || at >= slices.Index(ids, "file-organize-separator") {
			t.Fatal(ids)
		}
		anchor := "save"
		if on {
			anchor = "snaps.open"
		}
		if at != slices.Index(ids, anchor)+1 {
			t.Fatal(ids)
		}
	}
}

func TestVersionExtensionsDeclareContributionsAndHostServicesOnLedger(t *testing.T) {
	a, _, _ := writingFixture(t, "正文")
	for id, n := range map[string]int{"snaps": 2, "trash": 2, "find": 5} {
		l := a.extensions.Ledger(id)
		if len(l.Contributions) != n {
			t.Fatal(id, l.Contributions)
		}
		if len(l.Uses) == 0 {
			t.Fatal(id, "no host services recorded")
		}
	}
}

func TestRestoreServicesRefreshEditorTreeCountsAndKeepSelectedTab(t *testing.T) {
	a, tt, id := writingFixture(t, "第一版。")
	before, _ := a.Store.Snapshots(id)
	shownEditor(a).Insert("后来")
	if err := (extensionHost{a}).RestoreSnapshot(before[0], "第一版。"); err != nil {
		t.Fatal(err)
	}
	if shownEditor(a).Doc.String() != "第一版。" || a.write.node != id {
		t.Fatal("editor or tab changed")
	}
	two, err := a.book.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.book.SaveChapter(two, "另一章。", nil); err != nil {
		t.Fatal(err)
	}
	n, _ := a.book.Tree().Node(two)
	a.trashNode(n)
	if err = (extensionHost{a}).RestoreTrash(two); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.book.Tree().Node(two); !ok {
		t.Fatal("tree was not refreshed")
	}
	if a.book.Counts()[two] == 0 || a.write.node != id {
		t.Fatal("counts or selected tab stale")
	}
	a.trashNode(n)
	if err = (extensionHost{a}).PurgeTrash(two); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.node != id {
		t.Fatal("purge changed selected tab")
	}
	items, err := a.Store.Trash()
	if err != nil || len(items) != 0 {
		t.Fatal(items, err)
	}
}
