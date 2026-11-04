package book

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"writer/workspace"
)

func makeWork(t *testing.T) *workspace.Store {
	t.Helper()
	s, err := workspace.Create(t.TempDir(), "书", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// Open reads the whole book once: the tree, the counts, the marks, the
// setting, the plan and outlines, and the stored checks.
func TestOpenReadsEverything(t *testing.T) {
	s := makeWork(t)
	one, err := s.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	text := "井边有一把钥匙。"
	if _, err := s.WriteChapter(one, text, []workspace.ChapterMark{{
		Kind: workspace.MarkForeshadow, Start: 2, End: 4, ID: "f1",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteOutline(one, "她找到钥匙"); err != nil {
		t.Fatal(err)
	}
	category := s.Work().Categories[0].Name
	rev, err := s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
		Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: category, Text: "她怕黑",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	line, err := s.SaveLine(workspace.PlotLine{Name: "线"})
	if err != nil {
		t.Fatal(err)
	}
	card, err := s.SaveCard(workspace.Card{Title: "卡", Chapter: one, Line: line.ID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteCheck(one, workspace.Check{Of: workspace.ProseHash(text), Words: workspace.Chinese.Count(text), By: "test", At: time.Now()}); err != nil {
		t.Fatal(err)
	}

	b := Open(s)
	if _, ok := b.Tree().Node(one); !ok {
		t.Fatal("the tree has no chapter")
	}
	if b.Count(one) != workspace.Chinese.Count(text) {
		t.Fatalf("count = %d", b.Count(one))
	}
	if marks := b.Marks()[one]; len(marks) != 1 || marks[0].Quote != "有一" {
		t.Fatalf("marks = %+v", marks)
	}
	if _, ok := b.Setting().Entry(rev.Ops[0].Entry); !ok {
		t.Fatal("the setting has no entry")
	}
	if len(b.Plan().Cards) != 1 || b.Plan().Cards[0].ID != card.ID {
		t.Fatalf("plan = %+v", b.Plan())
	}
	if b.Outlines()[one] != "她找到钥匙" {
		t.Fatalf("outline = %q", b.Outlines()[one])
	}
	if got := b.Checks()[one]; got.Of != workspace.ProseHash(text) {
		t.Fatalf("check = %+v", got)
	}
	if b.Version() == 0 {
		t.Fatal("Version did not move")
	}
}

// Every write goes to the store and updates the copy: a chapter's count and
// marks, an outline, a check, the tree after a node changes, the setting, and
// the plan.
func TestWritesUpdateTheCopyAndTheDisk(t *testing.T) {
	s := makeWork(t)
	one, err := s.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	b := Open(s)
	up := func(f func() error) {
		t.Helper()
		before := b.Version()
		if err := f(); err != nil {
			t.Fatal(err)
		}
		if b.Version() <= before {
			t.Fatalf("Version stayed at %d", before)
		}
	}

	up(func() error {
		_, err := b.SaveChapter(one, "井边有一把钥匙。", []workspace.ChapterMark{{
			Kind: workspace.MarkForeshadow, Start: 2, End: 4, ID: "f1",
		}})
		return err
	})
	if b.Count(one) != workspace.Chinese.Count("井边有一把钥匙。") {
		t.Fatalf("count = %d", b.Count(one))
	}
	if marks := b.Marks()[one]; len(marks) != 1 || marks[0].Quote != "有一" {
		t.Fatalf("marks = %+v", marks)
	}
	if ch, err := s.ReadChapter(one); err != nil || ch.Text != "井边有一把钥匙。" || len(ch.Marks) != 1 {
		t.Fatalf("on disk: %+v %v", ch, err)
	}

	up(func() error { return b.SaveOutline(one, "她找到钥匙") })
	if b.Outlines()[one] != "她找到钥匙" {
		t.Fatalf("outline = %q", b.Outlines()[one])
	}
	if out, err := s.Outlines(); err != nil || out[one] != "她找到钥匙" {
		t.Fatalf("on disk: %q %v", out[one], err)
	}

	check := workspace.Check{Of: workspace.ProseHash("井边有一把钥匙。"), Words: 8, By: "test", At: time.Now()}
	up(func() error { return b.SaveCheck(one, check) })
	if b.Checks()[one].Of != check.Of {
		t.Fatalf("check = %+v", b.Checks()[one])
	}
	if got, err := s.Checks(); err != nil || got[one].Of != check.Of {
		t.Fatalf("on disk: %+v %v", got, err)
	}
	up(func() error { return b.SaveCheck(one, workspace.Check{}) })
	if _, ok := b.Checks()[one]; ok {
		t.Fatal("a check of nothing stayed")
	}

	up(func() error {
		_, err := b.NewChapter("", "第二章", false)
		return err
	})
	var id string
	b.Tree().Walk(func(n *workspace.Node, _ int) {
		if n.Title == "第二章" {
			id = n.ID
		}
	})
	if id == "" {
		t.Fatal("the new chapter is not in the tree")
	}
	up(func() error { return b.RenameChapter(id, "改名") })
	if n, ok := b.Tree().Node(id); !ok || n.Title != "改名" {
		t.Fatalf("tree after rename: %+v", n)
	}
	up(func() error { return b.MoveChapter(id, "", 0) })
	if n, ok := b.Tree().Node(id); !ok || n.Title != "改名" {
		t.Fatalf("tree after move: %+v", n)
	}
	up(func() error { return b.TrashChapter(id) })
	if _, ok := b.Tree().Node(id); ok {
		t.Fatal("trashed chapter is still in the tree")
	}
	if _, ok := b.Counts()[id]; ok {
		t.Fatal("trashed chapter is still counted")
	}
	up(func() error { return b.RestoreChapter(id) })
	if _, ok := b.Tree().Node(id); !ok {
		t.Fatal("restored chapter is not in the tree")
	}
	if _, ok := b.Counts()[id]; !ok {
		t.Fatal("restored chapter is not counted")
	}
	up(func() error { return b.TrashChapter(id) })
	up(func() error { return b.PurgeChapter(id) })
	if _, ok := b.Tree().Node(id); ok {
		t.Fatal("purged chapter is still in the tree")
	}

	category := s.Work().Categories[0].Name
	var rev workspace.Revision
	up(func() error {
		var err error
		rev, err = b.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
			Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: category, Text: "她怕黑",
		}}})
		return err
	})
	if _, ok := b.Setting().Entry(rev.Ops[0].Entry); !ok {
		t.Fatal("the revision is not in the setting")
	}
	if set, err := s.Setting(); err != nil || len(set.Entries) != 1 {
		t.Fatalf("on disk: %+v %v", set, err)
	}
	up(func() error { return b.RemoveCategory(category) })
	if e, ok := b.Setting().Entry(rev.Ops[0].Entry); !ok || e.Category != workspace.Other {
		t.Fatalf("after removing the category: %+v %v", e, ok)
	}

	var line1, line2 workspace.PlotLine
	up(func() error {
		var err error
		line1, err = b.SaveLine(workspace.PlotLine{Name: "一"})
		if err != nil {
			return err
		}
		line2, err = b.SaveLine(workspace.PlotLine{Name: "二"})
		return err
	})
	if len(b.Plan().Lines) != 2 {
		t.Fatalf("lines = %+v", b.Plan().Lines)
	}
	up(func() error { return b.MoveLine(line2.ID, -1) })
	if b.Plan().Lines[0].ID != line2.ID {
		t.Fatalf("lines after move = %+v", b.Plan().Lines)
	}
	up(func() error { return b.DeleteLine(line1.ID) })
	if len(b.Plan().Lines) != 1 {
		t.Fatalf("lines after delete = %+v", b.Plan().Lines)
	}

	var card workspace.Card
	up(func() error {
		var err error
		card, err = b.SaveCard(workspace.Card{Title: "卡", Chapter: one, When: "春"}, "")
		return err
	})
	if len(b.Plan().Cards) != 1 {
		t.Fatalf("cards = %+v", b.Plan().Cards)
	}
	up(func() error { return b.PlaceCard(card.ID, "", nil) })
	if b.Plan().Cards[0].Chapter != "" {
		t.Fatalf("card after place = %+v", b.Plan().Cards[0])
	}
	up(func() error { return b.DropCard(card.ID, -1, line2.ID) })
	if b.Plan().Cards[0].Line != line2.ID {
		t.Fatalf("card after drop = %+v", b.Plan().Cards[0])
	}
	up(func() error { return b.SaveTime(0, "夏") })
	if b.Plan().Cards[0].When != "夏" {
		t.Fatalf("card after time = %+v", b.Plan().Cards[0])
	}
	if got, err := s.Plan(); err != nil || len(got.Cards) != 1 || got.Cards[0].When != "夏" {
		t.Fatalf("on disk: %+v %v", got, err)
	}
	up(func() error { return b.DeleteCard(card.ID) })
	if len(b.Plan().Cards) != 0 {
		t.Fatalf("cards after delete = %+v", b.Plan().Cards)
	}

	before := b.Version()
	b.SetCount(one, b.Count(one))
	if b.Version() != before {
		t.Fatalf("the same count moved Version from %d to %d", before, b.Version())
	}
	b.SetCount(one, b.Count(one)+1)
	if b.Version() != before+1 {
		t.Fatalf("a changed count moved Version from %d to %d", before, b.Version())
	}
	treeBefore := b.Version()
	if err := b.ReloadTree(); err != nil {
		t.Fatal(err)
	}
	if b.Version() != treeBefore {
		t.Fatalf("the same tree moved Version from %d to %d", treeBefore, b.Version())
	}
	if _, err := s.NewChapter("", "第三章", false); err != nil {
		t.Fatal(err)
	}
	if err := b.ReloadTree(); err != nil {
		t.Fatal(err)
	}
	if b.Version() != treeBefore+1 {
		t.Fatalf("a changed tree moved Version from %d to %d", treeBefore, b.Version())
	}
}

// Every reload moves Version by one.
func TestEveryReloadMovesVersion(t *testing.T) {
	s := makeWork(t)
	one, err := s.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChapter(one, "正文。", nil); err != nil {
		t.Fatal(err)
	}
	b := Open(s)
	for _, reload := range []struct {
		name string
		fn   func() error
	}{
		{"tree", b.ReloadTree},
		{"counts", b.ReloadCounts},
		{"marks", b.ReloadMarks},
		{"setting", b.ReloadSetting},
		{"plan", b.ReloadPlan},
		{"outlines", b.ReloadOutlines},
		{"checks", b.ReloadChecks},
	} {
		if reload.name == "tree" {
			if _, err := s.NewChapter("", "第二章", false); err != nil {
				t.Fatal(err)
			}
		}
		before := b.Version()
		if err := reload.fn(); err != nil {
			t.Fatalf("%s: %v", reload.name, err)
		}
		if b.Version() != before+1 {
			t.Fatalf("%s moved Version from %d to %d", reload.name, before, b.Version())
		}
	}
}

// A broken plan is kept with its error and does not stop the rest.
func TestOpenLeavesABrokenPlanReadable(t *testing.T) {
	s := makeWork(t)
	one, err := s.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChapter(one, "正文。", nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root(), "ext", "outline", "outline.json"), []byte("{broken"), 0644); err != nil {
		t.Fatal(err)
	}
	b := Open(s)
	if b.PlanErr() == nil {
		t.Fatal("a broken plan read as one")
	}
	if _, ok := b.Tree().Node(one); !ok {
		t.Fatal("a broken plan left the rest of the book unread")
	}
}
